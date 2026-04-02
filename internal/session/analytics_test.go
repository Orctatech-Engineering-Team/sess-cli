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

func TestManagerGetGlobalStatsSummarizesTrackedProjects(t *testing.T) {
	mgr, database := newTestManager(t)

	alpha, err := mgr.InitializeProject("/tmp/alpha", "main")
	if err != nil {
		t.Fatalf("initialize alpha project: %v", err)
	}
	beta, err := mgr.InitializeProject("/tmp/beta", "dev")
	if err != nil {
		t.Fatalf("initialize beta project: %v", err)
	}

	now := time.Now()
	alphaStart := now.Add(-5 * time.Hour)
	betaStart := now.Add(-2 * time.Hour)
	prNumber := int64(77)

	if _, err := database.CreateSession(&db.Session{
		ProjectID:         alpha.ID,
		Branch:            "feature/alpha",
		IssueID:           "10",
		IssueTitle:        "Alpha work",
		State:             db.StateEnded,
		StartTime:         alphaStart,
		EndTime:           ptrTime(now.Add(-4 * time.Hour)),
		CurrentSliceStart: nil,
		TotalElapsed:      int64(90 * time.Minute),
		BranchType:        "feature",
		PRNumber:          &prNumber,
		PRURL:             "https://github.com/example/repo/pull/77",
	}); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}

	if _, err := database.CreateSession(&db.Session{
		ProjectID:         beta.ID,
		Branch:            "bugfix/beta",
		IssueID:           "11",
		IssueTitle:        "Beta fix",
		State:             db.StatePaused,
		StartTime:         betaStart,
		PauseTime:         ptrTime(now.Add(-90 * time.Minute)),
		CurrentSliceStart: nil,
		TotalElapsed:      int64(30 * time.Minute),
		BranchType:        "bugfix",
	}); err != nil {
		t.Fatalf("create beta session: %v", err)
	}

	stats, err := mgr.GetGlobalStats()
	if err != nil {
		t.Fatalf("get global stats: %v", err)
	}

	if stats.TotalProjects != 2 {
		t.Fatalf("stats.TotalProjects = %d, want 2", stats.TotalProjects)
	}
	if stats.TotalSessions != 2 {
		t.Fatalf("stats.TotalSessions = %d, want 2", stats.TotalSessions)
	}
	if stats.ActiveSessions != 0 {
		t.Fatalf("stats.ActiveSessions = %d, want 0", stats.ActiveSessions)
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
	if stats.TotalElapsed != 120*time.Minute {
		t.Fatalf("stats.TotalElapsed = %v, want %v", stats.TotalElapsed, 120*time.Minute)
	}
	if stats.AverageElapsed != 60*time.Minute {
		t.Fatalf("stats.AverageElapsed = %v, want %v", stats.AverageElapsed, 60*time.Minute)
	}
	if stats.LongestElapsed != 90*time.Minute {
		t.Fatalf("stats.LongestElapsed = %v, want %v", stats.LongestElapsed, 90*time.Minute)
	}
	if stats.LongestProject != "alpha" {
		t.Fatalf("stats.LongestProject = %q, want %q", stats.LongestProject, "alpha")
	}
	if stats.LongestBranch != "feature/alpha" {
		t.Fatalf("stats.LongestBranch = %q, want %q", stats.LongestBranch, "feature/alpha")
	}
	if stats.LongestIssueID != "10" {
		t.Fatalf("stats.LongestIssueID = %q, want %q", stats.LongestIssueID, "10")
	}
	if stats.LongestIssue != "Alpha work" {
		t.Fatalf("stats.LongestIssue = %q, want %q", stats.LongestIssue, "Alpha work")
	}
	if stats.FirstSessionAt == nil || !stats.FirstSessionAt.Equal(alphaStart) {
		t.Fatalf("stats.FirstSessionAt = %v, want %v", stats.FirstSessionAt, alphaStart)
	}
	if stats.LastSessionAt == nil || !stats.LastSessionAt.Equal(betaStart) {
		t.Fatalf("stats.LastSessionAt = %v, want %v", stats.LastSessionAt, betaStart)
	}
}

func TestManagerGetGlobalSessionHistoryOrdersNewestFirstAcrossProjects(t *testing.T) {
	mgr, database := newTestManager(t)

	alpha, err := mgr.InitializeProject("/tmp/alpha", "main")
	if err != nil {
		t.Fatalf("initialize alpha project: %v", err)
	}
	beta, err := mgr.InitializeProject("/tmp/beta", "dev")
	if err != nil {
		t.Fatalf("initialize beta project: %v", err)
	}

	alphaStart := time.Now().Add(-3 * time.Hour)
	betaStart := time.Now().Add(-1 * time.Hour)

	if _, err := database.CreateSession(&db.Session{
		ProjectID:         alpha.ID,
		Branch:            "feature/alpha",
		State:             db.StateEnded,
		StartTime:         alphaStart,
		EndTime:           ptrTime(alphaStart.Add(30 * time.Minute)),
		CurrentSliceStart: nil,
		TotalElapsed:      int64(30 * time.Minute),
		BranchType:        "feature",
	}); err != nil {
		t.Fatalf("create alpha session: %v", err)
	}

	if _, err := database.CreateSession(&db.Session{
		ProjectID:         beta.ID,
		Branch:            "bugfix/beta",
		State:             db.StatePaused,
		StartTime:         betaStart,
		PauseTime:         ptrTime(betaStart.Add(45 * time.Minute)),
		CurrentSliceStart: nil,
		TotalElapsed:      int64(45 * time.Minute),
		BranchType:        "bugfix",
	}); err != nil {
		t.Fatalf("create beta session: %v", err)
	}

	history, err := mgr.GetGlobalSessionHistory(10)
	if err != nil {
		t.Fatalf("get global session history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if history[0].Project.Name != "beta" {
		t.Fatalf("history[0].Project.Name = %q, want %q", history[0].Project.Name, "beta")
	}
	if history[0].Session.Branch != "bugfix/beta" {
		t.Fatalf("history[0].Session.Branch = %q, want %q", history[0].Session.Branch, "bugfix/beta")
	}
	if history[1].Project.Name != "alpha" {
		t.Fatalf("history[1].Project.Name = %q, want %q", history[1].Project.Name, "alpha")
	}

	limited, err := mgr.GetGlobalSessionHistory(1)
	if err != nil {
		t.Fatalf("get limited global session history: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("limited history length = %d, want 1", len(limited))
	}
	if limited[0].Project.Name != "beta" {
		t.Fatalf("limited history project = %q, want %q", limited[0].Project.Name, "beta")
	}
}
