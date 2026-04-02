package session

import (
	"fmt"
	"sort"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

// SessionHistoryEntry ties a session to its project metadata for reporting.
type SessionHistoryEntry struct {
	Project *db.Project
	Session *db.Session
}

// ProjectStats summarizes session history for a single project.
type ProjectStats struct {
	TotalProjects  int
	TotalSessions  int
	ActiveSessions int
	PausedSessions int
	EndedSessions  int
	SessionsWithPR int
	TotalElapsed   time.Duration
	AverageElapsed time.Duration
	LongestElapsed time.Duration
	FirstSessionAt *time.Time
	LastSessionAt  *time.Time
	LongestProject string
	LongestBranch  string
	LongestIssueID string
	LongestIssue   string
}

// GetProjectStats aggregates session history for a project.
func (m *Manager) GetProjectStats(projectID int64) (*ProjectStats, error) {
	sessions, err := m.db.ListSessions(projectID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	return m.accumulateStats(nil, sessions), nil
}

// GetGlobalStats aggregates session history across all tracked projects.
func (m *Manager) GetGlobalStats() (*ProjectStats, error) {
	projects, err := m.db.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	stats := &ProjectStats{
		TotalProjects: len(projects),
	}

	for _, project := range projects {
		sessions, err := m.db.ListSessions(project.ID)
		if err != nil {
			return nil, fmt.Errorf("list sessions for project %s: %w", project.Name, err)
		}

		projectStats := m.accumulateStats(project, sessions)
		stats.TotalSessions += projectStats.TotalSessions
		stats.ActiveSessions += projectStats.ActiveSessions
		stats.PausedSessions += projectStats.PausedSessions
		stats.EndedSessions += projectStats.EndedSessions
		stats.SessionsWithPR += projectStats.SessionsWithPR
		stats.TotalElapsed += projectStats.TotalElapsed

		if projectStats.FirstSessionAt != nil && (stats.FirstSessionAt == nil || projectStats.FirstSessionAt.Before(*stats.FirstSessionAt)) {
			start := *projectStats.FirstSessionAt
			stats.FirstSessionAt = &start
		}
		if projectStats.LastSessionAt != nil && (stats.LastSessionAt == nil || projectStats.LastSessionAt.After(*stats.LastSessionAt)) {
			start := *projectStats.LastSessionAt
			stats.LastSessionAt = &start
		}

		if projectStats.LongestElapsed > stats.LongestElapsed {
			stats.LongestElapsed = projectStats.LongestElapsed
			stats.LongestProject = projectStats.LongestProject
			stats.LongestBranch = projectStats.LongestBranch
			stats.LongestIssueID = projectStats.LongestIssueID
			stats.LongestIssue = projectStats.LongestIssue
		}
	}

	if stats.TotalSessions > 0 {
		stats.AverageElapsed = stats.TotalElapsed / time.Duration(stats.TotalSessions)
	}

	return stats, nil
}

// GetGlobalSessionHistory returns recent sessions across all tracked projects.
func (m *Manager) GetGlobalSessionHistory(limit int) ([]*SessionHistoryEntry, error) {
	projects, err := m.db.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	var entries []*SessionHistoryEntry
	for _, project := range projects {
		sessions, err := m.db.ListSessions(project.ID)
		if err != nil {
			return nil, fmt.Errorf("list sessions for project %s: %w", project.Name, err)
		}
		for _, sess := range sessions {
			entries = append(entries, &SessionHistoryEntry{
				Project: project,
				Session: sess,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Session.StartTime.After(entries[j].Session.StartTime)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

func (m *Manager) accumulateStats(project *db.Project, sessions []*db.Session) *ProjectStats {
	stats := &ProjectStats{}
	if project != nil {
		stats.TotalProjects = 1
		stats.LongestProject = project.Name
	}

	for _, sess := range sessions {
		stats.TotalSessions++

		elapsed := time.Duration(sess.TotalElapsed)
		if sess.State == db.StateActive {
			elapsed = m.GetCurrentElapsed(sess)
		}
		stats.TotalElapsed += elapsed

		switch sess.State {
		case db.StateActive:
			stats.ActiveSessions++
		case db.StatePaused:
			stats.PausedSessions++
		case db.StateEnded:
			stats.EndedSessions++
		}

		if sess.PRNumber != nil {
			stats.SessionsWithPR++
		}

		if stats.FirstSessionAt == nil || sess.StartTime.Before(*stats.FirstSessionAt) {
			start := sess.StartTime
			stats.FirstSessionAt = &start
		}
		if stats.LastSessionAt == nil || sess.StartTime.After(*stats.LastSessionAt) {
			start := sess.StartTime
			stats.LastSessionAt = &start
		}

		if elapsed > stats.LongestElapsed {
			stats.LongestElapsed = elapsed
			stats.LongestBranch = sess.Branch
			stats.LongestIssueID = sess.IssueID
			stats.LongestIssue = sess.IssueTitle
			if project != nil {
				stats.LongestProject = project.Name
			}
		}
	}

	if stats.TotalSessions > 0 {
		stats.AverageElapsed = stats.TotalElapsed / time.Duration(stats.TotalSessions)
	}

	return stats
}
