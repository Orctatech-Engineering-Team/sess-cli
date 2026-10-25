// Package db provides database operations for tracking projects and sessions
package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
	path string
}

// Project represents a tracked repository
type Project struct {
	ID         int64
	Name       string
	Path       string
	CreatedAt  time.Time
	LastUsedAt time.Time
	BaseBranch string
	IsActive   bool
}

// Session represents a work session
type Session struct {
	ID                int64
	ProjectID         int64
	Branch            string
	IssueID           string
	IssueTitle        string
	State             string // "active", "paused", "ended"
	StartTime         time.Time
	PauseTime         *time.Time
	EndTime           *time.Time
	CurrentSliceStart *time.Time // When the current active slice started
	TotalElapsed      int64      // Duration in nanoseconds
	BranchType        string
	PRNumber          *int64
	PRURL             string
}

const (
	StateActive = "active"
	StatePaused = "paused"
	StateEnded  = "ended"
)

// GetDefaultDBPath returns the default database path in user's home directory
func GetDefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(home, ".sess-cli", "sess.db"), nil
}

// Open opens or creates the database at the given path
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: path,
	}

	if err := db.init(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// OpenReadOnly opens an existing database without applying migrations.
func OpenReadOnly(path string) (*DB, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	uriPath := filepath.ToSlash(absPath)
	if runtime.GOOS == "windows" {
		uriPath = "/" + uriPath
	}
	uri := &url.URL{Scheme: "file", Path: uriPath}
	query := uri.Query()
	query.Set("mode", "ro")
	uri.RawQuery = query.Encode()

	conn, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, fmt.Errorf("open database read-only: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("open database read-only: %w", err)
	}
	return &DB{conn: conn, path: absPath}, nil
}

// init creates the database schema if it doesn't exist
func (db *DB) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		created_at DATETIME NOT NULL,
		last_used_at DATETIME NOT NULL,
		base_branch TEXT NOT NULL DEFAULT 'dev',
		is_active BOOLEAN NOT NULL DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_projects_path ON projects(path);
	CREATE INDEX IF NOT EXISTS idx_projects_is_active ON projects(is_active);

	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		branch TEXT NOT NULL,
		issue_id TEXT,
		issue_title TEXT,
		state TEXT NOT NULL CHECK(state IN ('active', 'paused', 'ended')),
		start_time DATETIME NOT NULL,
		pause_time DATETIME,
		end_time DATETIME,
		current_slice_start DATETIME,
		total_elapsed INTEGER NOT NULL DEFAULT 0,
		branch_type TEXT,
		pr_number INTEGER,
		pr_url TEXT,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_project_id ON sessions(project_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
	CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time);

	CREATE TABLE IF NOT EXISTS participants (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL CHECK(kind IN ('human', 'agent', 'automation')),
		label TEXT NOT NULL DEFAULT '',
		identity_source TEXT NOT NULL CHECK(identity_source IN ('configured', 'orchestrator', 'ephemeral')),
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS attempts (
		id TEXT PRIMARY KEY,
		session_id INTEGER NOT NULL,
		participant_id TEXT NOT NULL,
		role TEXT NOT NULL CHECK(role IN ('implementation', 'review', 'research', 'release', 'other')),
		state TEXT NOT NULL CHECK(state IN ('active', 'blocked', 'completed', 'cancelled', 'failed')),
		started_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		finished_at DATETIME,
		start_repository TEXT,
		start_branch TEXT,
		start_head_oid TEXT,
		start_worktree TEXT NOT NULL,
		start_captured_at DATETIME NOT NULL,
		result_repository TEXT,
		result_branch TEXT,
		result_head_oid TEXT,
		result_worktree TEXT,
		result_captured_at DATETIME,
		blocked_reason TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
		FOREIGN KEY (participant_id) REFERENCES participants(id)
	);

	CREATE INDEX IF NOT EXISTS idx_attempts_session_state ON attempts(session_id, state);
	CREATE INDEX IF NOT EXISTS idx_attempts_participant_id ON attempts(participant_id);

	CREATE TABLE IF NOT EXISTS attempt_claims (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		attempt_id TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('path', 'workflow', 'other')),
		value TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE(attempt_id, type, value),
		FOREIGN KEY (attempt_id) REFERENCES attempts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_attempt_claims_attempt_id ON attempt_claims(attempt_id);

	CREATE TABLE IF NOT EXISTS session_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		kind TEXT NOT NULL CHECK(kind IN (
			'session_started', 'session_paused', 'session_resumed', 'session_ended'
		)),
		occurred_at DATETIME NOT NULL,
		state TEXT NOT NULL CHECK(state IN ('active', 'paused', 'ended')),
		total_elapsed INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_session_events_session_id_id ON session_events(session_id, id);

	CREATE TABLE IF NOT EXISTS end_workflows (
		session_id INTEGER PRIMARY KEY,
		branch TEXT NOT NULL,
		base_branch TEXT NOT NULL,
		commit_message TEXT NOT NULL,
		end_without_pr BOOLEAN NOT NULL,
		pr_title TEXT NOT NULL,
		pr_body TEXT NOT NULL,
		phase TEXT NOT NULL CHECK(phase IN (
			'prepared', 'committed', 'rebased', 'pushed', 'pr_created', 'base_checked_out'
		)),
		head TEXT NOT NULL,
		rebase_target TEXT NOT NULL DEFAULT '',
		pr_number INTEGER,
		pr_url TEXT NOT NULL,
		repository TEXT NOT NULL DEFAULT '',
		remote_id TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing databases
	if err := db.runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runMigrations applies schema changes to existing databases
func (db *DB) runMigrations() error {
	migrations := []struct {
		table      string
		column     string
		definition string
	}{
		{table: "sessions", column: "current_slice_start", definition: "DATETIME"},
		{table: "sessions", column: "pr_number", definition: "INTEGER"},
		{table: "sessions", column: "pr_url", definition: "TEXT"},
		{table: "end_workflows", column: "rebase_target", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "end_workflows", column: "repository", definition: "TEXT NOT NULL DEFAULT ''"},
		{table: "end_workflows", column: "remote_id", definition: "TEXT NOT NULL DEFAULT ''"},
	}

	for _, migration := range migrations {
		exists, err := db.hasColumn(migration.table, migration.column)
		if err != nil {
			return fmt.Errorf("failed to check for %s column: %w", migration.column, err)
		}
		if exists {
			continue
		}

		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", migration.table, migration.column, migration.definition)
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to add %s column: %w", migration.column, err)
		}
	}

	return nil
}

func (db *DB) hasColumn(table, name string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info(?)
		WHERE name = ?
	`, table, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// CreateProject creates a new project record
func (db *DB) CreateProject(name, path, baseBranch string) (*Project, error) {
	now := time.Now()

	// Check if project already exists
	existing, err := db.GetProjectByPath(path)
	if err == nil && existing != nil {
		// Update last_used_at and return existing project
		existing.LastUsedAt = now
		existing.IsActive = true
		if err := db.UpdateProject(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	result, err := db.conn.Exec(
		`INSERT INTO projects (name, path, created_at, last_used_at, base_branch, is_active)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		name, path, now, now, baseBranch, true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID: %w", err)
	}

	return &Project{
		ID:         id,
		Name:       name,
		Path:       path,
		CreatedAt:  now,
		LastUsedAt: now,
		BaseBranch: baseBranch,
		IsActive:   true,
	}, nil
}

// GetProjectByPath retrieves a project by its path
func (db *DB) GetProjectByPath(path string) (*Project, error) {
	var p Project
	err := db.conn.QueryRow(
		`SELECT id, name, path, created_at, last_used_at, base_branch, is_active
		 FROM projects WHERE path = ?`,
		path,
	).Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt, &p.LastUsedAt, &p.BaseBranch, &p.IsActive)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &p, nil
}

// GetProjectByID retrieves a project by its ID
func (db *DB) GetProjectByID(id int64) (*Project, error) {
	var p Project
	err := db.conn.QueryRow(
		`SELECT id, name, path, created_at, last_used_at, base_branch, is_active
		 FROM projects WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt, &p.LastUsedAt, &p.BaseBranch, &p.IsActive)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &p, nil
}

// ListProjects returns all active projects
func (db *DB) ListProjects() ([]*Project, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, path, created_at, last_used_at, base_branch, is_active
		 FROM projects WHERE is_active = 1 ORDER BY last_used_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt, &p.LastUsedAt, &p.BaseBranch, &p.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, &p)
	}

	return projects, nil
}

// UpdateProject updates a project
func (db *DB) UpdateProject(p *Project) error {
	_, err := db.conn.Exec(
		`UPDATE projects SET name = ?, last_used_at = ?, base_branch = ?, is_active = ?
		 WHERE id = ?`,
		p.Name, p.LastUsedAt, p.BaseBranch, p.IsActive, p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

// CreateSession creates a new session
func (db *DB) CreateSession(s *Session) (*Session, error) {
	result, err := db.conn.Exec(
		`INSERT INTO sessions (project_id, branch, issue_id, issue_title, state, start_time, current_slice_start, total_elapsed, branch_type, pr_number, pr_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ProjectID, s.Branch, s.IssueID, s.IssueTitle, s.State, s.StartTime, s.CurrentSliceStart, s.TotalElapsed, s.BranchType, s.PRNumber, s.PRURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get session ID: %w", err)
	}

	s.ID = id
	return s, nil
}

// GetActiveSession retrieves the active or paused session for a project
func (db *DB) GetActiveSession(projectID int64) (*Session, error) {
	var s Session
	var pauseTime, endTime, currentSliceStart sql.NullTime
	var prNumber sql.NullInt64
	var prURL sql.NullString

	err := db.conn.QueryRow(
		`SELECT id, project_id, branch, COALESCE(issue_id, ''), COALESCE(issue_title, ''),
		        state, start_time, pause_time, end_time, current_slice_start, total_elapsed, COALESCE(branch_type, ''),
		        pr_number, COALESCE(pr_url, '')
		 FROM sessions
		 WHERE project_id = ? AND state IN ('active', 'paused')
		 ORDER BY start_time DESC LIMIT 1`,
		projectID,
	).Scan(&s.ID, &s.ProjectID, &s.Branch, &s.IssueID, &s.IssueTitle, &s.State,
		&s.StartTime, &pauseTime, &endTime, &currentSliceStart, &s.TotalElapsed, &s.BranchType, &prNumber, &prURL)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}

	if pauseTime.Valid {
		s.PauseTime = &pauseTime.Time
	}
	if endTime.Valid {
		s.EndTime = &endTime.Time
	}
	if currentSliceStart.Valid {
		s.CurrentSliceStart = &currentSliceStart.Time
	}
	if prNumber.Valid {
		s.PRNumber = &prNumber.Int64
	}
	if prURL.Valid {
		s.PRURL = prURL.String
	}

	return &s, nil
}

// UpdateSession updates a session
func (db *DB) UpdateSession(s *Session) error {
	_, err := db.conn.Exec(
		`UPDATE sessions
		 SET state = ?, pause_time = ?, end_time = ?, current_slice_start = ?, total_elapsed = ?, pr_number = ?, pr_url = ?
		 WHERE id = ?`,
		s.State, s.PauseTime, s.EndTime, s.CurrentSliceStart, s.TotalElapsed, s.PRNumber, s.PRURL, s.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

// CompleteSession ends only the expected session snapshot and clears its checkpoint.
func (db *DB) CompleteSession(before, after *Session) error {
	if before == nil || after == nil {
		return fmt.Errorf("complete session: before and after are required")
	}
	if after.State != StateEnded || after.EndTime == nil || after.CurrentSliceStart != nil {
		return fmt.Errorf("complete session %d: require ended state, end time, and no active slice", after.ID)
	}

	if err := db.TransitionSession(before, after); err != nil {
		return fmt.Errorf("complete session %d: %w", after.ID, err)
	}
	return nil
}

func requireOneRow(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("affected %d rows, want 1", rows)
	}
	return nil
}

// ListSessions retrieves all sessions for a project, newest first.
func (db *DB) ListSessions(projectID int64) ([]*Session, error) {
	rows, err := db.conn.Query(
		`SELECT id, project_id, branch, COALESCE(issue_id, ''), COALESCE(issue_title, ''),
		        state, start_time, pause_time, end_time, current_slice_start, total_elapsed, COALESCE(branch_type, ''),
		        pr_number, COALESCE(pr_url, '')
		 FROM sessions
		 WHERE project_id = ?
		 ORDER BY start_time DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// GetSessionHistory retrieves session history for a project
func (db *DB) GetSessionHistory(projectID int64, limit int) ([]*Session, error) {
	rows, err := db.conn.Query(
		`SELECT id, project_id, branch, COALESCE(issue_id, ''), COALESCE(issue_title, ''),
		        state, start_time, pause_time, end_time, current_slice_start, total_elapsed, COALESCE(branch_type, ''),
		        pr_number, COALESCE(pr_url, '')
		 FROM sessions
		 WHERE project_id = ?
		 ORDER BY start_time DESC LIMIT ?`,
		projectID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}

	return sessions, nil
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var sessions []*Session
	for rows.Next() {
		var s Session
		var pauseTime, endTime, currentSliceStart sql.NullTime
		var prNumber sql.NullInt64
		var prURL sql.NullString

		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Branch, &s.IssueID, &s.IssueTitle,
			&s.State, &s.StartTime, &pauseTime, &endTime, &currentSliceStart, &s.TotalElapsed, &s.BranchType, &prNumber, &prURL); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if pauseTime.Valid {
			s.PauseTime = &pauseTime.Time
		}
		if endTime.Valid {
			s.EndTime = &endTime.Time
		}
		if currentSliceStart.Valid {
			s.CurrentSliceStart = &currentSliceStart.Time
		}
		if prNumber.Valid {
			s.PRNumber = &prNumber.Int64
		}
		if prURL.Valid {
			s.PRURL = prURL.String
		}

		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}

	return sessions, nil
}
