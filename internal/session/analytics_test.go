package session

import (
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

func TestManagerGetProjectStatsSummarizesSessions(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)

	now := time.Now()
	endedStart := now.Add(-4 * time.Hour)
	pausedStart := now.Add(-3 * time.Hour)
	activeStart := now.Add(-2 * time.Hour)
	pausedAt := now.Add(-150 * time.Minute)

	prNumber := int64(42)
	if _, err := database.CreateSession(&db.Session{
		ProjectID:         project.ID,
		Branch:            "feature/ended",
		IssueID:           "123",
		IssueTitle:        "Ended issue",
		State:             db.StateEnded,
		StartTime:         endedStart,
		EndTime:           ptrTime(now.Add(-3 * time.Hour)),
		CurrentSliceStart: nil,
		TotalElapsed:      int64(90 * time.Minute),
		BranchType:        "feature",
		PRNumber:          &prNumber,
		PRURL:             "https://github.com/example/repo/pull/42",
	}); err != nil {
		t.Fatalf("create ended session: %v", err)
	}

	if _, err := database.CreateSession(&db.Session{
		ProjectID:         project.ID,
		Branch:            "bugfix/paused",
		IssueID:           "124",
		IssueTitle:        "Paused issue",
		State:             db.StatePaused,
		StartTime:         pausedStart,
		PauseTime:         &pausedAt,
		CurrentSliceStart: nil,
		TotalElapsed:      int64(30 * time.Minute),
		BranchType:        "bugfix",
	}); err != nil {
		t.Fatalf("create paused session: %v", err)
	}

	currentSliceStart := now.Add(-45 * time.Minute)
	if _, err := database.CreateSession(&db.Session{
		ProjectID:         project.ID,
		Branch:            "feature/active",
		IssueID:           "125",
		IssueTitle:        "Active issue",
		State:             db.StateActive,
		StartTime:         activeStart,
		CurrentSliceStart: &currentSliceStart,
		TotalElapsed:      int64(15 * time.Minute),
		BranchType:        "feature",
	}); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	stats, err := mgr.GetProjectStats(project.ID)
	if err != nil {
		t.Fatalf("get project stats: %v", err)
	}

	if stats.TotalSessions != 3 {
		t.Fatalf("stats.TotalSessions = %d, want 3", stats.TotalSessions)
	}
	if stats.ActiveSessions != 1 {
		t.Fatalf("stats.ActiveSessions = %d, want 1", stats.ActiveSessions)
	}
	if stats.PausedSessions != 1 {
		t.Fatalf("stats.PausedSessions = %d, want 1", stats.PausedSessions)
	}
	if stats.EndedSessions != 1 {
		t.Fatalf("stats.EndedSessions = %d, want 1", stats.EndedSessions)
	}
	if stats.SessionsWithPR != 1 {
		t.Fatalf("stats.SessionsWithPR = %d, want 1", stats.SessionsWithPR)
	}
	if stats.TotalElapsed < 135*time.Minute {
		t.Fatalf("stats.TotalElapsed = %v, want at least %v", stats.TotalElapsed, 135*time.Minute)
	}
	if stats.AverageElapsed < 45*time.Minute {
		t.Fatalf("stats.AverageElapsed = %v, want at least %v", stats.AverageElapsed, 45*time.Minute)
	}
	if stats.LongestElapsed < 90*time.Minute {
		t.Fatalf("stats.LongestElapsed = %v, want at least %v", stats.LongestElapsed, 90*time.Minute)
	}
	if stats.LongestBranch != "feature/ended" {
		t.Fatalf("stats.LongestBranch = %q, want %q", stats.LongestBranch, "feature/ended")
	}
	if stats.LongestIssueID != "123" {
		t.Fatalf("stats.LongestIssueID = %q, want %q", stats.LongestIssueID, "123")
	}
	if stats.LongestIssue != "Ended issue" {
		t.Fatalf("stats.LongestIssue = %q, want %q", stats.LongestIssue, "Ended issue")
	}
	if stats.FirstSessionAt == nil || !stats.FirstSessionAt.Equal(endedStart) {
		t.Fatalf("stats.FirstSessionAt = %v, want %v", stats.FirstSessionAt, endedStart)
	}
	if stats.LastSessionAt == nil || !stats.LastSessionAt.Equal(activeStart) {
		t.Fatalf("stats.LastSessionAt = %v, want %v", stats.LastSessionAt, activeStart)
	}
}
