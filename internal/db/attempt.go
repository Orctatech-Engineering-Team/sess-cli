package db

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	AttemptActive    = "active"
	AttemptBlocked   = "blocked"
	AttemptCompleted = "completed"
	AttemptCancelled = "cancelled"
	AttemptFailed    = "failed"
)

const (
	ParticipantHuman      = "human"
	ParticipantAgent      = "agent"
	ParticipantAutomation = "automation"
)

type Participant struct {
	ID             string
	Kind           string
	Label          string
	IdentitySource string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RepositoryState struct {
	Repository string
	Branch     string
	HeadOID    string
	Worktree   string
	CapturedAt time.Time
}

type AttemptClaim struct {
	Type      string
	Value     string
	CreatedAt time.Time
}

type Attempt struct {
	ID                    string
	SessionID             int64
	Participant           Participant
	Role                  string
	State                 string
	StartedAt             time.Time
	UpdatedAt             time.Time
	FinishedAt            *time.Time
	StartRepositoryState  RepositoryState
	ResultRepositoryState *RepositoryState
	BlockedReason         string
	Claims                []AttemptClaim
}

func (db *DB) CreateParticipant(p *Participant) error {
	if p == nil || strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("create participant: ID is required")
	}
	if !validParticipantKind(p.Kind) || !validIdentitySource(p.IdentitySource) {
		return fmt.Errorf("create participant: invalid kind or identity source")
	}
	now := p.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	updated := p.UpdatedAt.UTC()
	if updated.IsZero() {
		updated = now
	}
	_, err := db.conn.Exec(`
		INSERT INTO participants (id, kind, label, identity_source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			updated_at = participants.updated_at
	`, p.ID, p.Kind, p.Label, p.IdentitySource, now, updated)
	if err != nil {
		return fmt.Errorf("create participant: %w", err)
	}
	existing, err := db.GetParticipant(p.ID)
	if err != nil {
		return err
	}
	if existing.Kind != p.Kind || existing.Label != p.Label || existing.IdentitySource != p.IdentitySource {
		return fmt.Errorf("create participant %s: identity already has different attributes", p.ID)
	}
	*p = *existing
	return nil
}

func (db *DB) GetParticipant(id string) (*Participant, error) {
	var p Participant
	err := db.conn.QueryRow(`
		SELECT id, kind, label, identity_source, created_at, updated_at
		FROM participants WHERE id = ?
	`, id).Scan(&p.ID, &p.Kind, &p.Label, &p.IdentitySource, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get participant %s: %w", id, err)
	}
	return &p, nil
}

func (db *DB) CreateAttempt(a *Attempt) error {
	if a == nil || strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("create attempt: ID is required")
	}
	if a.State != AttemptActive || a.SessionID <= 0 || a.StartedAt.IsZero() {
		return fmt.Errorf("create attempt: invalid initial state")
	}
	if !validAttemptRole(a.Role) || a.StartRepositoryState.Worktree == "" {
		return fmt.Errorf("create attempt: invalid role or repository state")
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("create attempt: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var sessionState string
	if err := tx.QueryRow(`SELECT state FROM sessions WHERE id = ?`, a.SessionID).Scan(&sessionState); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("create attempt: session %d not found", a.SessionID)
		}
		return fmt.Errorf("create attempt: session lookup: %w", err)
	}
	if sessionState != StateActive && sessionState != StatePaused {
		return fmt.Errorf("create attempt: session %d is %s", a.SessionID, sessionState)
	}
	var participantExists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM participants WHERE id = ?`, a.Participant.ID).Scan(&participantExists); err != nil {
		return fmt.Errorf("create attempt: participant lookup: %w", err)
	}
	if participantExists != 1 {
		return fmt.Errorf("create attempt: participant %s not found", a.Participant.ID)
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = a.StartedAt
	}
	_, err = tx.Exec(`
		INSERT INTO attempts (
			id, session_id, participant_id, role, state, started_at, updated_at,
			start_repository, start_branch, start_head_oid, start_worktree,
			start_captured_at, blocked_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.SessionID, a.Participant.ID, a.Role, a.State, a.StartedAt.UTC(), a.UpdatedAt.UTC(),
		nullableString(a.StartRepositoryState.Repository), nullableString(a.StartRepositoryState.Branch),
		nullableString(a.StartRepositoryState.HeadOID), a.StartRepositoryState.Worktree,
		a.StartRepositoryState.CapturedAt.UTC(), a.BlockedReason)
	if err != nil {
		return fmt.Errorf("create attempt: insert: %w", err)
	}
	for _, claim := range a.Claims {
		if !validClaimType(claim.Type) || strings.TrimSpace(claim.Value) == "" {
			return fmt.Errorf("create attempt: invalid claim")
		}
		createdAt := claim.CreatedAt
		if createdAt.IsZero() {
			createdAt = a.StartedAt
		}
		if _, err := tx.Exec(`
			INSERT INTO attempt_claims (attempt_id, type, value, created_at)
			VALUES (?, ?, ?, ?)
		`, a.ID, claim.Type, claim.Value, createdAt.UTC()); err != nil {
			return fmt.Errorf("create attempt: insert claim: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create attempt: commit: %w", err)
	}
	return nil
}

func (db *DB) GetAttempt(id string) (*Attempt, error) {
	return getAttempt(db.conn, id)
}

func (db *DB) ListActiveAttempts(sessionID int64) ([]*Attempt, error) {
	var tableCount int
	if err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'attempts'
	`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("list active attempts: check schema: %w", err)
	}
	if tableCount == 0 {
		return []*Attempt{}, nil
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("list active attempts: begin transaction: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`
		SELECT id FROM attempts
		WHERE session_id = ? AND state IN ('active', 'blocked')
		ORDER BY started_at, id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list active attempts: %w", err)
	}
	defer rows.Close()
	var attempts []*Attempt
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list active attempts: %w", err)
		}
		attempt, err := getAttempt(tx, id)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active attempts: %w", err)
	}
	if attempts == nil {
		attempts = make([]*Attempt, 0)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("list active attempts: close rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("list active attempts: commit: %w", err)
	}
	return attempts, nil
}

func (db *DB) TransitionAttempt(before, after *Attempt) error {
	if before == nil || after == nil || before.ID != after.ID || before.SessionID != after.SessionID {
		return fmt.Errorf("transition attempt: identity changed")
	}
	if !validAttemptTransition(before.State, after.State) {
		return fmt.Errorf("transition attempt %s: invalid transition %s -> %s", before.ID, before.State, after.State)
	}
	if after.State == AttemptBlocked && strings.TrimSpace(after.BlockedReason) == "" {
		return fmt.Errorf("transition attempt %s: blocked reason is required", after.ID)
	}
	if isTerminalAttempt(after.State) && after.FinishedAt == nil {
		return fmt.Errorf("transition attempt %s: finished time is required", after.ID)
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("transition attempt %s: begin transaction: %w", before.ID, err)
	}
	defer tx.Rollback()
	current, err := getAttempt(tx, before.ID)
	if err != nil {
		return err
	}
	if current == nil || !sameAttempt(current, before) {
		return fmt.Errorf("transition attempt %s: stale or missing attempt", before.ID)
	}
	if after.UpdatedAt.IsZero() {
		after.UpdatedAt = time.Now().UTC()
	}
	_, err = tx.Exec(`
		UPDATE attempts SET state = ?, updated_at = ?, finished_at = ?,
			result_repository = ?, result_branch = ?, result_head_oid = ?,
			result_worktree = ?, result_captured_at = ?, blocked_reason = ?
		WHERE id = ? AND state = ?
	`, after.State, after.UpdatedAt.UTC(), after.FinishedAt,
		nullableAttemptResult(after.ResultRepositoryState, "repository"),
		nullableAttemptResult(after.ResultRepositoryState, "branch"),
		nullableAttemptResult(after.ResultRepositoryState, "head_oid"),
		nullableAttemptResult(after.ResultRepositoryState, "worktree"),
		nullableAttemptTime(after.ResultRepositoryState), after.BlockedReason,
		after.ID, before.State)
	if err != nil {
		return fmt.Errorf("transition attempt %s: update: %w", before.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("transition attempt %s: commit: %w", before.ID, err)
	}
	return nil
}

type attemptQueryer interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

func getAttempt(queryer attemptQueryer, id string) (*Attempt, error) {
	var a Attempt
	var participant Participant
	var startRepository, startBranch, startHead sql.NullString
	var resultRepository, resultBranch, resultHead, resultWorktree sql.NullString
	var resultCaptured sql.NullTime
	err := queryer.QueryRow(`
		SELECT a.id, a.session_id, a.participant_id, a.role, a.state,
			a.started_at, a.updated_at, a.finished_at,
			a.start_repository, a.start_branch, a.start_head_oid, a.start_worktree,
			a.start_captured_at, a.result_repository, a.result_branch,
			a.result_head_oid, a.result_worktree, a.result_captured_at,
			a.blocked_reason, p.kind, p.label, p.identity_source,
			p.created_at, p.updated_at
		FROM attempts a JOIN participants p ON p.id = a.participant_id
		WHERE a.id = ?
	`, id).Scan(&a.ID, &a.SessionID, &participant.ID, &a.Role, &a.State,
		&a.StartedAt, &a.UpdatedAt, &a.FinishedAt,
		&startRepository, &startBranch, &startHead, &a.StartRepositoryState.Worktree,
		&a.StartRepositoryState.CapturedAt, &resultRepository, &resultBranch,
		&resultHead, &resultWorktree, &resultCaptured, &a.BlockedReason,
		&participant.Kind, &participant.Label, &participant.IdentitySource,
		&participant.CreatedAt, &participant.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attempt %s: %w", id, err)
	}
	a.Participant = participant
	a.StartRepositoryState.Repository = startRepository.String
	a.StartRepositoryState.Branch = startBranch.String
	a.StartRepositoryState.HeadOID = startHead.String
	if resultCaptured.Valid {
		a.ResultRepositoryState = &RepositoryState{
			Repository: resultRepository.String,
			Branch:     resultBranch.String,
			HeadOID:    resultHead.String,
			Worktree:   resultWorktree.String,
			CapturedAt: resultCaptured.Time,
		}
	}
	claims, err := queryAttemptClaims(queryer, a.ID)
	if err != nil {
		return nil, err
	}
	a.Claims = claims
	return &a, nil
}

func queryAttemptClaims(queryer attemptQueryer, attemptID string) ([]AttemptClaim, error) {
	rows, err := queryer.Query(`
		SELECT type, value, created_at FROM attempt_claims
		WHERE attempt_id = ? ORDER BY id
	`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("list claims for attempt %s: %w", attemptID, err)
	}
	defer rows.Close()
	claims := make([]AttemptClaim, 0)
	for rows.Next() {
		var claim AttemptClaim
		if err := rows.Scan(&claim.Type, &claim.Value, &claim.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan claim: %w", err)
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func sameAttempt(a, b *Attempt) bool {
	return reflect.DeepEqual(a, b)
}

func validParticipantKind(kind string) bool {
	return kind == ParticipantHuman || kind == ParticipantAgent || kind == ParticipantAutomation
}

func validIdentitySource(source string) bool {
	return source == "configured" || source == "orchestrator" || source == "ephemeral"
}

func validAttemptRole(role string) bool {
	return role == "implementation" || role == "review" || role == "research" || role == "release" || role == "other"
}

func validClaimType(kind string) bool {
	return kind == "path" || kind == "workflow" || kind == "other"
}

func validAttemptTransition(before, after string) bool {
	if before == AttemptActive {
		return after == AttemptBlocked || isTerminalAttempt(after)
	}
	if before == AttemptBlocked {
		return after == AttemptActive || isTerminalAttempt(after)
	}
	return false
}

func isTerminalAttempt(state string) bool {
	return state == AttemptCompleted || state == AttemptCancelled || state == AttemptFailed
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableAttemptResult(state *RepositoryState, field string) any {
	if state == nil {
		return nil
	}
	switch field {
	case "repository":
		return nullableString(state.Repository)
	case "branch":
		return nullableString(state.Branch)
	case "head_oid":
		return nullableString(state.HeadOID)
	case "worktree":
		return nullableString(state.Worktree)
	default:
		return nil
	}
}

func nullableAttemptTime(state *RepositoryState) any {
	if state == nil || state.CapturedAt.IsZero() {
		return nil
	}
	return state.CapturedAt.UTC()
}
