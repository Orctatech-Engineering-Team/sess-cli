package session

import (
	"fmt"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

// ProjectStats summarizes session history for a single project.
type ProjectStats struct {
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

	stats := &ProjectStats{}

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
		}
	}

	if stats.TotalSessions > 0 {
		stats.AverageElapsed = stats.TotalElapsed / time.Duration(stats.TotalSessions)
	}

	return stats, nil
}
