// Package session provides high-level session management operations
package session

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

// Manager handles session operations
type Manager struct {
	db *db.DB
}

// NewManager creates a new session manager
func NewManager(database *db.DB) *Manager {
	return &Manager{
		db: database,
	}
}

// InitializeProject initializes or retrieves a project for the given path
func (m *Manager) InitializeProject(path, baseBranch string) (*db.Project, error) {
	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Get project name from path
	name := filepath.Base(absPath)

	// Create or update project
	project, err := m.db.CreateProject(name, absPath, baseBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize project: %w", err)
	}

	return project, nil
}

// StartSession starts a new session for a project
func (m *Manager) StartSession(projectID int64, branch, issueID, issueTitle, branchType string) (*db.Session, error) {
	// Check if there's already an active session
	existing, err := m.db.GetActiveSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for active session: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("there is already an %s session on branch '%s'. Please end or pause it first", existing.State, existing.Branch)
	}

	now := time.Now()

	// Create new session
	session := &db.Session{
		ProjectID:         projectID,
		Branch:            branch,
		IssueID:           issueID,
		IssueTitle:        issueTitle,
		State:             db.StateActive,
		StartTime:         now,
		TotalElapsed:      0,
		BranchType:        branchType,
		CurrentSliceStart: &now, // Track current slice start time
	}

	created, err := m.db.CreateSession(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update project's last_used_at
	project, err := m.db.GetProjectByID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	project.LastUsedAt = time.Now()
	if err := m.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return created, nil
}

// GetActiveSession retrieves the active or paused session for a project
func (m *Manager) GetActiveSession(projectID int64) (*db.Session, error) {
	return m.db.GetActiveSession(projectID)
}

// PauseSession pauses an active session
func (m *Manager) PauseSession(projectID int64) (*db.Session, error) {
	session, err := m.db.GetActiveSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}
	if session.State != db.StateActive {
		return nil, fmt.Errorf("can only pause an active session")
	}

	// Ensure CurrentSliceStart is set (handle old sessions or corrupted state)
	if session.CurrentSliceStart == nil {
		return nil, fmt.Errorf("session is in invalid state: missing slice start time")
	}

	// Compute elapsed time for current slice with nanosecond precision
	now := time.Now()
	elapsed := now.Sub(*session.CurrentSliceStart)
	session.TotalElapsed += elapsed.Nanoseconds()

	// Update session state
	session.State = db.StatePaused
	session.PauseTime = &now
	session.CurrentSliceStart = nil // Clear slice start when paused

	if err := m.db.UpdateSession(session); err != nil {
		return nil, fmt.Errorf("failed to pause session: %w", err)
	}

	return session, nil
}

// ResumeSession resumes a paused session
func (m *Manager) ResumeSession(projectID int64) (*db.Session, error) {
	session, err := m.db.GetActiveSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil || session.State != db.StatePaused {
		return nil, fmt.Errorf("no paused session found")
	}

	now := time.Now()

	// Update state and start new slice
	session.State = db.StateActive
	session.CurrentSliceStart = &now

	if err := m.db.UpdateSession(session); err != nil {
		return nil, fmt.Errorf("failed to resume session: %w", err)
	}

	return session, nil
}

// EndSession ends an active or paused session
func (m *Manager) EndSession(projectID int64) (*db.Session, error) {
	session, err := m.db.GetActiveSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no active session found")
	}

	now := time.Now()

	// Commit any in-progress slice
	if session.State == db.StateActive && session.CurrentSliceStart != nil {
		elapsed := now.Sub(*session.CurrentSliceStart)
		session.TotalElapsed += elapsed.Nanoseconds()
	}

	// Update state to ended
	session.State = db.StateEnded
	session.EndTime = &now
	session.CurrentSliceStart = nil

	if err := m.db.UpdateSession(session); err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	return session, nil
}

// GetCurrentElapsed calculates the current elapsed time for a session
func (m *Manager) GetCurrentElapsed(session *db.Session) time.Duration {
	total := time.Duration(session.TotalElapsed)

	// Add current slice time if session is active
	if session.State == db.StateActive && session.CurrentSliceStart != nil {
		total += time.Since(*session.CurrentSliceStart)
	}

	return total
}

// GetProject retrieves a project by path
func (m *Manager) GetProject(path string) (*db.Project, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	return m.db.GetProjectByPath(absPath)
}

// ListProjects returns all tracked projects
func (m *Manager) ListProjects() ([]*db.Project, error) {
	return m.db.ListProjects()
}

// GetSessionHistory retrieves session history for a project
func (m *Manager) GetSessionHistory(projectID int64, limit int) ([]*db.Session, error) {
	return m.db.GetSessionHistory(projectID, limit)
}

// SyncActiveSession updates elapsed time for currently active session without changing state
// Useful for periodic syncing in long-running sessions
func (m *Manager) SyncActiveSession(projectID int64) (*db.Session, error) {
	session, err := m.db.GetActiveSession(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil || session.State != db.StateActive {
		return session, nil // Nothing to sync
	}

	if session.CurrentSliceStart == nil {
		return nil, fmt.Errorf("active session has no slice start time")
	}

	// Calculate and commit current slice
	now := time.Now()
	elapsed := now.Sub(*session.CurrentSliceStart)
	session.TotalElapsed += elapsed.Nanoseconds()

	// Restart slice from now
	session.CurrentSliceStart = &now

	if err := m.db.UpdateSession(session); err != nil {
		return nil, fmt.Errorf("failed to sync session: %w", err)
	}

	return session, nil
}
