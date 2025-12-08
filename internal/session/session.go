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
	return &Manager{db: database}
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

	// Create new session
	session := &db.Session{
		ProjectID:    projectID,
		Branch:       branch,
		IssueID:      issueID,
		IssueTitle:   issueTitle,
		State:        db.StateActive,
		StartTime:    time.Now(),
		TotalElapsed: 0,
		BranchType:   branchType,
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

	if session.State == db.StatePaused {
		return nil, fmt.Errorf("session is already paused")
	}

	if session.State != db.StateActive {
		return nil, fmt.Errorf("can only pause an active session")
	}

	// Calculate elapsed time since start
	now := time.Now()
	elapsed := int64(now.Sub(session.StartTime).Seconds())
	session.TotalElapsed += elapsed

	// Update session state
	session.State = db.StatePaused
	session.PauseTime = &now

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

	if session == nil {
		return nil, fmt.Errorf("no paused session found")
	}

	if session.State == db.StateActive {
		return nil, fmt.Errorf("session is already active")
	}

	if session.State != db.StatePaused {
		return nil, fmt.Errorf("can only resume a paused session")
	}

	// Update session state
	session.State = db.StateActive
	session.StartTime = time.Now() // Reset start time for new active period

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

	// If session is active, add the current period to elapsed time
	if session.State == db.StateActive {
		elapsed := int64(now.Sub(session.StartTime).Seconds())
		session.TotalElapsed += elapsed
	}

	// Update session state
	session.State = db.StateEnded
	session.EndTime = &now

	if err := m.db.UpdateSession(session); err != nil {
		return nil, fmt.Errorf("failed to end session: %w", err)
	}

	return session, nil
}

// GetCurrentElapsed calculates the current elapsed time for a session
func (m *Manager) GetCurrentElapsed(session *db.Session) time.Duration {
	totalSeconds := session.TotalElapsed

	// If session is active, add time since start
	if session.State == db.StateActive {
		totalSeconds += int64(time.Since(session.StartTime).Seconds())
	}

	return time.Duration(totalSeconds) * time.Second
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
