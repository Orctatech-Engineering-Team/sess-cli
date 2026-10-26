package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CheckCommand = "command"
	CheckManual  = "manual"
)

const (
	CheckPassed      = "passed"
	CheckFailed      = "failed"
	CheckCancelled   = "cancelled"
	CheckUnavailable = "unavailable"
)

type Check struct {
	ID              string
	AttemptID       string
	SessionID       int64
	Name            string
	ProcedureType   string
	ProcedureValue  string
	Outcome         string
	StartedAt       time.Time
	FinishedAt      time.Time
	ExitCode        *int
	RepositoryState RepositoryState
	Reproducibility string
	Summary         string
	OutputReference *string
}

func (db *DB) CreateCheck(check *Check) error {
	if check == nil || strings.TrimSpace(check.ID) == "" || strings.TrimSpace(check.AttemptID) == "" {
		return fmt.Errorf("create check: ID and attempt ID are required")
	}
	if strings.TrimSpace(check.Name) == "" || len(check.Name) > 2048 {
		return fmt.Errorf("create check: name must be between 1 and 2048 bytes")
	}
	if !validCheckProcedure(check.ProcedureType) || check.ProcedureValue == "" || len(check.ProcedureValue) > 2048 {
		return fmt.Errorf("create check: invalid procedure")
	}
	if !validCheckOutcome(check.Outcome) || !validReproducibility(check.Reproducibility) {
		return fmt.Errorf("create check: invalid outcome or reproducibility")
	}
	if !utf8.ValidString(check.Summary) || strings.IndexByte(check.Summary, 0) >= 0 || len(check.Summary) > 4096 {
		return fmt.Errorf("create check: summary is invalid or exceeds 4096 bytes")
	}
	if strings.IndexByte(check.ProcedureValue, 0) >= 0 {
		return fmt.Errorf("create check: procedure contains NUL byte")
	}
	if check.StartedAt.IsZero() || check.FinishedAt.IsZero() || check.FinishedAt.Before(check.StartedAt) {
		return fmt.Errorf("create check: invalid time range")
	}
	if check.RepositoryState.Worktree == "" || check.RepositoryState.CapturedAt.IsZero() {
		return fmt.Errorf("create check: repository state is required")
	}
	var attemptCount int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM attempts WHERE id = ?`, check.AttemptID).Scan(&attemptCount); err != nil {
		return fmt.Errorf("create check: attempt lookup: %w", err)
	}
	if attemptCount != 1 {
		return fmt.Errorf("create check: attempt %s not found", check.AttemptID)
	}

	_, err := db.conn.Exec(`
		INSERT INTO checks (
			id, attempt_id, name, procedure_type, procedure_value, outcome,
			started_at, finished_at, exit_code, repository, branch, head_oid,
			worktree, captured_at, reproducibility, summary, output_reference
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, check.ID, check.AttemptID, check.Name, check.ProcedureType, check.ProcedureValue,
		check.Outcome, check.StartedAt.UTC(), check.FinishedAt.UTC(), check.ExitCode,
		nullableString(check.RepositoryState.Repository), nullableString(check.RepositoryState.Branch),
		nullableString(check.RepositoryState.HeadOID), check.RepositoryState.Worktree,
		check.RepositoryState.CapturedAt.UTC(), check.Reproducibility, check.Summary,
		check.OutputReference)
	if err != nil {
		return fmt.Errorf("create check: %w", err)
	}
	return nil
}

func (db *DB) GetCheck(id string) (*Check, error) {
	return getCheck(db.conn, id)
}

func (db *DB) ListChecksForSession(sessionID int64) ([]*Check, error) {
	var tableCount int
	if err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'checks'
	`).Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("list checks: check schema: %w", err)
	}
	if tableCount == 0 {
		return []*Check{}, nil
	}
	rows, err := db.conn.Query(`
		SELECT c.id
		FROM checks c JOIN attempts a ON a.id = c.attempt_id
		WHERE a.session_id = ?
		ORDER BY c.finished_at, c.id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}
	defer rows.Close()
	checks := make([]*Check, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list checks: %w", err)
		}
		check, err := db.GetCheck(id)
		if err != nil {
			return nil, err
		}
		if check != nil {
			checks = append(checks, check)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list checks: %w", err)
	}
	return checks, nil
}

type checkQueryer interface {
	QueryRow(string, ...any) *sql.Row
}

func getCheck(queryer checkQueryer, id string) (*Check, error) {
	var check Check
	var repository, branch, head sql.NullString
	var exitCode sql.NullInt64
	var outputReference sql.NullString
	err := queryer.QueryRow(`
		SELECT c.id, c.attempt_id, a.session_id, c.name, c.procedure_type,
			c.procedure_value, c.outcome, c.started_at, c.finished_at,
			c.exit_code, c.repository, c.branch, c.head_oid, c.worktree,
			c.captured_at, c.reproducibility, c.summary, c.output_reference
		FROM checks c JOIN attempts a ON a.id = c.attempt_id
		WHERE c.id = ?
	`, id).Scan(&check.ID, &check.AttemptID, &check.SessionID, &check.Name,
		&check.ProcedureType, &check.ProcedureValue, &check.Outcome,
		&check.StartedAt, &check.FinishedAt, &exitCode, &repository, &branch,
		&head, &check.RepositoryState.Worktree, &check.RepositoryState.CapturedAt,
		&check.Reproducibility, &check.Summary, &outputReference)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get check %s: %w", id, err)
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		check.ExitCode = &value
	}
	check.RepositoryState.Repository = repository.String
	check.RepositoryState.Branch = branch.String
	check.RepositoryState.HeadOID = head.String
	if outputReference.Valid {
		check.OutputReference = &outputReference.String
	}
	return &check, nil
}

func validCheckProcedure(value string) bool {
	return value == CheckCommand || value == CheckManual
}

func validCheckOutcome(value string) bool {
	return value == CheckPassed || value == CheckFailed || value == CheckCancelled || value == CheckUnavailable
}

func validReproducibility(value string) bool {
	return value == "revision" || value == "workspace" || value == "unknown"
}
