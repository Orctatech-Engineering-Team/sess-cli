package db

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	EventSessionStarted = "session_started"
	EventSessionPaused  = "session_paused"
	EventSessionResumed = "session_resumed"
	EventSessionEnded   = "session_ended"
)

// SessionEvent records the persisted state after a lifecycle transition.
type SessionEvent struct {
	ID           int64
	SessionID    int64
	Kind         string
	OccurredAt   time.Time
	State        string
	TotalElapsed int64
}

// CreateTrackedSession starts a session and journals it in one transaction.
// CreateSession remains available for legacy records without lifecycle events.
func (db *DB) CreateTrackedSession(s *Session) (*Session, error) {
	if s == nil {
		return nil, fmt.Errorf("create tracked session: session is nil")
	}
	if s.ID != 0 || s.StartTime.IsZero() {
		return nil, fmt.Errorf("create tracked session: require new identity and start time")
	}
	if s.State != "" && s.State != StateActive {
		return nil, fmt.Errorf("create tracked session: state %q must be active", s.State)
	}
	started := *s
	started.State = StateActive
	started.StartTime = s.StartTime.UTC()
	started.CurrentSliceStart = &started.StartTime
	started.PauseTime = nil
	started.EndTime = nil
	started.TotalElapsed = 0

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("create tracked session: begin transaction: %w", err)
	}
	defer tx.Rollback()

	// The guard is part of the first write, so competing starts cannot both insert.
	result, err := tx.Exec(`
		INSERT INTO sessions (
			project_id, branch, issue_id, issue_title, state, start_time,
			current_slice_start, total_elapsed, branch_type, pr_number, pr_url
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM sessions WHERE project_id = ? AND state IN ('active', 'paused')
		)
	`,
		started.ProjectID,
		started.Branch,
		started.IssueID,
		started.IssueTitle,
		started.State,
		started.StartTime,
		started.CurrentSliceStart,
		started.TotalElapsed,
		started.BranchType,
		started.PRNumber,
		started.PRURL,
		started.ProjectID,
	)
	if err != nil {
		return nil, fmt.Errorf("create tracked session: insert session: %w", err)
	}
	if err := requireOneRow(result); err != nil {
		return nil, fmt.Errorf("create tracked session: project %d already has an active/paused session: %w",
			s.ProjectID, err)
	}
	started.ID, err = result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create tracked session: get session ID: %w", err)
	}
	if err := insertSessionEvent(tx, &started, EventSessionStarted, started.StartTime); err != nil {
		return nil, err
	}
	if err := touchSessionProject(tx, started.ProjectID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create tracked session: commit transaction: %w", err)
	}
	*s = started
	return s, nil
}

// TransitionSession compares the full expected record before updating and journaling.
// Ending a session also removes its checkpoint in the same transaction.
func (db *DB) TransitionSession(before, after *Session) error {
	if before == nil || after == nil {
		return fmt.Errorf("transition session: before and after are required")
	}
	if !sameSessionIdentity(before, after) {
		return fmt.Errorf("transition session %d: session identity changed", before.ID)
	}
	kind, occurredAt, err := sessionTransition(before, after)
	if err != nil {
		return fmt.Errorf("transition session %d: %w", before.ID, err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("transition session %d: begin transaction: %w", before.ID, err)
	}
	defer tx.Rollback()
	current, err := getSessionByID(tx, before.ID)
	if err != nil {
		return err
	}
	if current == nil || !sameSession(current, before) {
		return fmt.Errorf("transition session %d: stale or missing session", before.ID)
	}

	// SQLite keeps this read and write in one serializable transaction. A concurrent
	// writer makes the upgrade fail rather than overwriting a newer snapshot.
	if err := writeSession(tx, after); err != nil {
		return err
	}
	if err := insertSessionEvent(tx, after, kind, occurredAt); err != nil {
		return err
	}
	if err := touchSessionProject(tx, after.ProjectID); err != nil {
		return err
	}
	if after.State == StateEnded {
		if _, err := tx.Exec(`DELETE FROM end_workflows WHERE session_id = ?`, after.ID); err != nil {
			return fmt.Errorf("transition session %d: delete end workflow: %w", before.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transition session %d: commit transaction: %w", before.ID, err)
	}
	return nil
}

// SyncSession persists active-slice accounting without inventing a lifecycle event.
func (db *DB) SyncSession(before, after *Session) error {
	if before == nil || after == nil {
		return fmt.Errorf("sync session: before and after are required")
	}
	if !sameSessionIdentity(before, after) {
		return fmt.Errorf("sync session %d: session identity changed", before.ID)
	}
	if before.State != StateActive || after.State != StateActive {
		return fmt.Errorf("sync session %d: require active state before and after", before.ID)
	}
	if after.CurrentSliceStart == nil || after.CurrentSliceStart.IsZero() || after.EndTime != nil {
		return fmt.Errorf("sync session %d: require active slice and no end time", before.ID)
	}
	if after.TotalElapsed < before.TotalElapsed || after.TotalElapsed < 0 {
		return fmt.Errorf("sync session %d: total elapsed cannot decrease", before.ID)
	}
	if !sameTime(before.PauseTime, after.PauseTime) ||
		!sameInt64(before.PRNumber, after.PRNumber) || before.PRURL != after.PRURL {
		return fmt.Errorf("sync session %d: only active-slice accounting may change", before.ID)
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("sync session %d: begin transaction: %w", before.ID, err)
	}
	defer tx.Rollback()
	current, err := getSessionByID(tx, before.ID)
	if err != nil {
		return err
	}
	if current == nil || !sameSession(current, before) {
		return fmt.Errorf("sync session %d: stale or missing session", before.ID)
	}
	if err := writeSession(tx, after); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sync session %d: commit transaction: %w", before.ID, err)
	}
	return nil
}

func writeSession(tx *sql.Tx, s *Session) error {
	result, err := tx.Exec(`
		UPDATE sessions
		SET state = ?, pause_time = ?, end_time = ?, current_slice_start = ?,
		    total_elapsed = ?, pr_number = ?, pr_url = ?
		WHERE id = ?
	`,
		s.State,
		utcTime(s.PauseTime),
		utcTime(s.EndTime),
		utcTime(s.CurrentSliceStart),
		s.TotalElapsed,
		s.PRNumber,
		s.PRURL,
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("update session %d: %w", s.ID, err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("update session %d: %w", s.ID, err)
	}
	return nil
}

func sessionTransition(before, after *Session) (string, time.Time, error) {
	var kind string
	var occurredAt *time.Time
	switch {
	case before.State == StateActive && after.State == StatePaused:
		kind, occurredAt = EventSessionPaused, after.PauseTime
		if after.CurrentSliceStart != nil || after.EndTime != nil {
			return "", time.Time{}, fmt.Errorf("paused session must have no active slice or end time")
		}
	case before.State == StatePaused && after.State == StateActive:
		kind, occurredAt = EventSessionResumed, after.CurrentSliceStart
		if after.EndTime != nil {
			return "", time.Time{}, fmt.Errorf("active session must have no end time")
		}
	case (before.State == StateActive || before.State == StatePaused) && after.State == StateEnded:
		kind, occurredAt = EventSessionEnded, after.EndTime
		if after.CurrentSliceStart != nil {
			return "", time.Time{}, fmt.Errorf("ended session must have no active slice")
		}
	default:
		return "", time.Time{}, fmt.Errorf("invalid transition %q -> %q", before.State, after.State)
	}
	if occurredAt == nil || occurredAt.IsZero() {
		return "", time.Time{}, fmt.Errorf("transition %q requires an event time", kind)
	}
	if after.TotalElapsed < before.TotalElapsed || after.TotalElapsed < 0 {
		return "", time.Time{}, fmt.Errorf("total elapsed must be nonnegative and cannot decrease")
	}
	return kind, occurredAt.UTC(), nil
}

func insertSessionEvent(tx *sql.Tx, s *Session, kind string, occurredAt time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO session_events (session_id, kind, occurred_at, state, total_elapsed)
		VALUES (?, ?, ?, ?, ?)
	`,
		s.ID,
		kind,
		occurredAt.UTC(),
		s.State,
		s.TotalElapsed,
	)
	if err != nil {
		return fmt.Errorf("journal session %d: %w", s.ID, err)
	}
	return nil
}

func touchSessionProject(tx *sql.Tx, projectID int64) error {
	result, err := tx.Exec(`UPDATE projects SET last_used_at = ? WHERE id = ?`, time.Now().UTC(), projectID)
	if err != nil {
		return fmt.Errorf("touch session project %d: %w", projectID, err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("touch session project %d: %w", projectID, err)
	}
	return nil
}

// GetSessionByID returns nil when no session has the given identity.
func (db *DB) GetSessionByID(id int64) (*Session, error) {
	return getSessionByID(db.conn, id)
}

func getSessionByID(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int64) (*Session, error) {
	var s Session
	err := q.QueryRow(`
		SELECT id, project_id, branch, COALESCE(issue_id, ''), COALESCE(issue_title, ''),
		       state, start_time, pause_time, end_time, current_slice_start, total_elapsed,
		       COALESCE(branch_type, ''), pr_number, COALESCE(pr_url, '')
		FROM sessions WHERE id = ?
	`, id).Scan(
		&s.ID,
		&s.ProjectID,
		&s.Branch,
		&s.IssueID,
		&s.IssueTitle,
		&s.State,
		&s.StartTime,
		&s.PauseTime,
		&s.EndTime,
		&s.CurrentSliceStart,
		&s.TotalElapsed,
		&s.BranchType,
		&s.PRNumber,
		&s.PRURL,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session %d: %w", id, err)
	}
	return &s, nil
}

// ListSessionEvents returns lifecycle events in insertion order, including tied timestamps.
func (db *DB) ListSessionEvents(sessionID int64) ([]*SessionEvent, error) {
	return listSessionEvents(db.conn, sessionID)
}

// ReadSessionTimeline reads a session and its journal from the same database snapshot.
// A missing session returns nil for both the session and its events.
func (db *DB) ReadSessionTimeline(sessionID int64) (*Session, []*SessionEvent, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("read session timeline %d: begin transaction: %w", sessionID, err)
	}
	defer tx.Rollback()
	s, err := getSessionByID(tx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if s == nil {
		return nil, nil, nil
	}
	events, err := listSessionEvents(tx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("read session timeline %d: commit transaction: %w", sessionID, err)
	}
	return s, events, nil
}

func listSessionEvents(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, sessionID int64) ([]*SessionEvent, error) {
	rows, err := q.Query(`
		SELECT id, session_id, kind, occurred_at, state, total_elapsed
		FROM session_events WHERE session_id = ? ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list events for session %d: %w", sessionID, err)
	}
	defer rows.Close()
	events := []*SessionEvent{}
	for rows.Next() {
		var event SessionEvent
		if err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.Kind,
			&event.OccurredAt,
			&event.State,
			&event.TotalElapsed,
		); err != nil {
			return nil, fmt.Errorf("scan event for session %d: %w", sessionID, err)
		}
		events = append(events, &event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events for session %d: %w", sessionID, err)
	}
	return events, nil
}

func sameSessionIdentity(a, b *Session) bool {
	return a.ID == b.ID && a.ProjectID == b.ProjectID && a.Branch == b.Branch &&
		a.IssueID == b.IssueID && a.IssueTitle == b.IssueTitle &&
		a.StartTime.Equal(b.StartTime) && a.BranchType == b.BranchType
}

func sameSession(a, b *Session) bool {
	return sameSessionIdentity(a, b) && a.State == b.State && a.TotalElapsed == b.TotalElapsed &&
		sameTime(a.PauseTime, b.PauseTime) && sameTime(a.EndTime, b.EndTime) &&
		sameTime(a.CurrentSliceStart, b.CurrentSliceStart) && sameInt64(a.PRNumber, b.PRNumber) &&
		a.PRURL == b.PRURL
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func sameInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func utcTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
