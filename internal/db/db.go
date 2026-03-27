// Package db provides database operations for tracking projects and sessions
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

	sessDir := filepath.Join(home, ".sess-cli")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .sess-cli directory: %w", err)
	}

	return filepath.Join(sessDir, "sess.db"), nil
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
		column     string
		definition string
	}{
		{column: "current_slice_start", definition: "DATETIME"},
		{column: "pr_number", definition: "INTEGER"},
		{column: "pr_url", definition: "TEXT"},
	}

	for _, migration := range migrations {
		exists, err := db.hasSessionsColumn(migration.column)
		if err != nil {
			return fmt.Errorf("failed to check for %s column: %w", migration.column, err)
		}
		if exists {
			continue
		}

		query := fmt.Sprintf("ALTER TABLE sessions ADD COLUMN %s %s", migration.column, migration.definition)
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to add %s column: %w", migration.column, err)
		}
	}

	return nil
}

func (db *DB) hasSessionsColumn(name string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('sessions')
		WHERE name = ?
	`, name).Scan(&count)
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

	return sessions, nil
}
