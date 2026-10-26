package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Handoff struct {
	ID              string
	SessionID       int64
	AttemptID       string
	CreatedAt       time.Time
	RepositoryState RepositoryState
	Completed       []string
	Remaining       []string
	Risks           []string
	NextAction      string
	CheckIDs        []string
	SupersedesID    string
}

func (db *DB) CreateHandoff(h *Handoff) error {
	if h == nil || strings.TrimSpace(h.ID) == "" || strings.TrimSpace(h.AttemptID) == "" {
		return fmt.Errorf("create handoff: ID and attempt ID are required")
	}
	if h.SessionID <= 0 || h.CreatedAt.IsZero() || h.RepositoryState.Worktree == "" ||
		h.RepositoryState.CapturedAt.IsZero() {
		return fmt.Errorf("create handoff: session and repository state are required")
	}
	if err := validateHandoffEntries(h.Completed, 20, 1024, "completed"); err != nil {
		return err
	}
	if err := validateHandoffEntries(h.Remaining, 20, 1024, "remaining"); err != nil {
		return err
	}
	if err := validateHandoffEntries(h.Risks, 20, 1024, "risks"); err != nil {
		return err
	}
	if err := validateHandoffText(h.NextAction, 2048, "next action"); err != nil {
		return err
	}
	for _, id := range h.CheckIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("create handoff: check ID is empty")
		}
	}

	completed, err := json.Marshal(h.Completed)
	if err != nil {
		return fmt.Errorf("create handoff: encode completed: %w", err)
	}
	remaining, err := json.Marshal(h.Remaining)
	if err != nil {
		return fmt.Errorf("create handoff: encode remaining: %w", err)
	}
	risks, err := json.Marshal(h.Risks)
	if err != nil {
		return fmt.Errorf("create handoff: encode risks: %w", err)
	}
	checkIDs, err := json.Marshal(h.CheckIDs)
	if err != nil {
		return fmt.Errorf("create handoff: encode checks: %w", err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("create handoff: begin transaction: %w", err)
	}
	defer tx.Rollback()

	var attemptSessionID int64
	if err := tx.QueryRow(`SELECT session_id FROM attempts WHERE id = ?`, h.AttemptID).Scan(&attemptSessionID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("create handoff: attempt %s not found", h.AttemptID)
		}
		return fmt.Errorf("create handoff: attempt lookup: %w", err)
	}
	if attemptSessionID != h.SessionID {
		return fmt.Errorf("create handoff: attempt does not belong to session")
	}
	for _, checkID := range h.CheckIDs {
		var checkAttemptID string
		if err := tx.QueryRow(`SELECT attempt_id FROM checks WHERE id = ?`, checkID).Scan(&checkAttemptID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("create handoff: check %s not found", checkID)
			}
			return fmt.Errorf("create handoff: check lookup: %w", err)
		}
		if checkAttemptID != h.AttemptID {
			return fmt.Errorf("create handoff: check %s belongs to a different attempt", checkID)
		}
	}
	if h.SupersedesID != "" {
		var oldSessionID int64
		if err := tx.QueryRow(`SELECT session_id FROM handoffs WHERE id = ?`, h.SupersedesID).Scan(&oldSessionID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("create handoff: superseded handoff %s not found", h.SupersedesID)
			}
			return fmt.Errorf("create handoff: superseded handoff lookup: %w", err)
		}
		if oldSessionID != h.SessionID || h.SupersedesID == h.ID {
			return fmt.Errorf("create handoff: invalid superseded handoff")
		}
		seen := map[string]bool{h.ID: true}
		cursor := h.SupersedesID
		for cursor != "" {
			if seen[cursor] {
				return fmt.Errorf("create handoff: supersession cycle")
			}
			seen[cursor] = true
			var next sql.NullString
			if err := tx.QueryRow(`SELECT supersedes_id FROM handoffs WHERE id = ?`, cursor).Scan(&next); err != nil {
				return fmt.Errorf("create handoff: supersession lookup: %w", err)
			}
			cursor = next.String
		}
	}
	_, err = tx.Exec(`
		INSERT INTO handoffs (
			id, session_id, attempt_id, created_at, repository, branch, head_oid,
			worktree, captured_at, completed_json, remaining_json, risks_json,
			next_action, check_ids_json, supersedes_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, h.ID, h.SessionID, h.AttemptID, h.CreatedAt.UTC(),
		nullableString(h.RepositoryState.Repository), nullableString(h.RepositoryState.Branch),
		nullableString(h.RepositoryState.HeadOID), h.RepositoryState.Worktree,
		h.RepositoryState.CapturedAt.UTC(), completed, remaining, risks, h.NextAction,
		checkIDs, nullableString(h.SupersedesID))
	if err != nil {
		return fmt.Errorf("create handoff: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create handoff: commit: %w", err)
	}
	return nil
}

func (db *DB) ListHandoffsForSession(sessionID int64) ([]*Handoff, error) {
	var tableCount int
	if err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'handoffs'
	`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("list handoffs: check schema: %w", err)
	}
	if tableCount == 0 {
		return []*Handoff{}, nil
	}
	rows, err := db.conn.Query(`
		SELECT id FROM handoffs WHERE session_id = ?
		ORDER BY created_at, id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list handoffs: %w", err)
	}
	defer rows.Close()
	result := make([]*Handoff, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list handoffs: %w", err)
		}
		handoff, err := getHandoff(db.conn, id)
		if err != nil {
			return nil, err
		}
		if handoff != nil {
			result = append(result, handoff)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list handoffs: %w", err)
	}
	return result, nil
}

func getHandoff(queryer interface {
	QueryRow(string, ...any) *sql.Row
}, id string) (*Handoff, error) {
	var h Handoff
	var repository, branch, head, supersedes sql.NullString
	var completed, remaining, risks, checkIDs string
	err := queryer.QueryRow(`
		SELECT id, session_id, attempt_id, created_at, repository, branch, head_oid,
			worktree, captured_at, completed_json, remaining_json, risks_json,
			next_action, check_ids_json, supersedes_id
		FROM handoffs WHERE id = ?
	`, id).Scan(&h.ID, &h.SessionID, &h.AttemptID, &h.CreatedAt, &repository,
		&branch, &head, &h.RepositoryState.Worktree, &h.RepositoryState.CapturedAt,
		&completed, &remaining, &risks, &h.NextAction, &checkIDs, &supersedes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get handoff %s: %w", id, err)
	}
	h.RepositoryState.Repository = repository.String
	h.RepositoryState.Branch = branch.String
	h.RepositoryState.HeadOID = head.String
	h.SupersedesID = supersedes.String
	if err := json.Unmarshal([]byte(completed), &h.Completed); err != nil {
		return nil, fmt.Errorf("get handoff %s: decode completed: %w", id, err)
	}
	if err := json.Unmarshal([]byte(remaining), &h.Remaining); err != nil {
		return nil, fmt.Errorf("get handoff %s: decode remaining: %w", id, err)
	}
	if err := json.Unmarshal([]byte(risks), &h.Risks); err != nil {
		return nil, fmt.Errorf("get handoff %s: decode risks: %w", id, err)
	}
	if err := json.Unmarshal([]byte(checkIDs), &h.CheckIDs); err != nil {
		return nil, fmt.Errorf("get handoff %s: decode checks: %w", id, err)
	}
	return &h, nil
}

func validateHandoffEntries(entries []string, maxEntries, maxLength int, field string) error {
	if len(entries) > maxEntries {
		return fmt.Errorf("create handoff: %s has more than %d entries", field, maxEntries)
	}
	for _, entry := range entries {
		if err := validateHandoffText(entry, maxLength, field); err != nil {
			return err
		}
	}
	return nil
}

func validateHandoffText(value string, maxLength int, field string) error {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 || len(value) > maxLength {
		return fmt.Errorf("create handoff: %s is invalid or exceeds %d bytes", field, maxLength)
	}
	return nil
}
