package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

func newTestManager(t *testing.T) (*Manager, *db.DB) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "sess.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
	})

	return NewManager(database), database
}

func newTestProject(t *testing.T, mgr *Manager) *db.Project {
	t.Helper()

	projectPath := filepath.Join(t.TempDir(), "repo")
	project, err := mgr.InitializeProject(projectPath, "main")
	if err != nil {
		t.Fatalf("initialize project: %v", err)
	}

	return project
}

func TestManagerSessionLifecycle(t *testing.T) {
	mgr, _ := newTestManager(t)
	project := newTestProject(t, mgr)

	started, err := mgr.StartSession(project.ID, "feature/test", "123", "Test issue", "feature")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if started.State != db.StateActive {
		t.Fatalf("started session state = %q, want %q", started.State, db.StateActive)
	}
	if started.CurrentSliceStart == nil {
		t.Fatal("started session missing current slice start")
	}

	time.Sleep(10 * time.Millisecond)

	paused, err := mgr.PauseSession(project.ID)
	if err != nil {
		t.Fatalf("pause session: %v", err)
	}
	if paused.State != db.StatePaused {
		t.Fatalf("paused session state = %q, want %q", paused.State, db.StatePaused)
	}
	if paused.PauseTime == nil {
		t.Fatal("paused session missing pause time")
	}
	if paused.CurrentSliceStart != nil {
		t.Fatal("paused session should clear current slice start")
	}
	if paused.TotalElapsed <= 0 {
		t.Fatalf("paused session total elapsed = %d, want > 0", paused.TotalElapsed)
	}
	pausedElapsed := paused.TotalElapsed

	resumed, err := mgr.ResumeSession(project.ID)
	if err != nil {
		t.Fatalf("resume session: %v", err)
	}
	if resumed.State != db.StateActive {
		t.Fatalf("resumed session state = %q, want %q", resumed.State, db.StateActive)
	}
	if resumed.CurrentSliceStart == nil {
		t.Fatal("resumed session missing current slice start")
	}

	time.Sleep(10 * time.Millisecond)

	ended, err := mgr.EndSession(project.ID)
	if err != nil {
		t.Fatalf("end session: %v", err)
	}
	if ended.State != db.StateEnded {
		t.Fatalf("ended session state = %q, want %q", ended.State, db.StateEnded)
	}
	if ended.EndTime == nil {
		t.Fatal("ended session missing end time")
	}
	if ended.CurrentSliceStart != nil {
		t.Fatal("ended session should clear current slice start")
	}
	if ended.TotalElapsed <= pausedElapsed {
		t.Fatalf("ended session total elapsed = %d, want > %d", ended.TotalElapsed, pausedElapsed)
	}

	active, err := mgr.GetActiveSession(project.ID)
	if err != nil {
		t.Fatalf("get active session after end: %v", err)
	}
	if active != nil {
		t.Fatalf("active session after end = %#v, want nil", active)
	}
}

func TestManagerCompleteSessionStoresPRMetadata(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)

	if _, err := mgr.StartSession(project.ID, "feature/test", "123", "Test issue", "feature"); err != nil {
		t.Fatalf("start session: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	prNumber := int64(77)
	ended, err := mgr.CompleteSession(project.ID, &prNumber, "https://github.com/example/repo/pull/77")
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}
	if ended.State != db.StateEnded {
		t.Fatalf("ended state = %q, want %q", ended.State, db.StateEnded)
	}
	if ended.PRNumber == nil || *ended.PRNumber != prNumber {
		t.Fatalf("ended PR number = %v, want %d", ended.PRNumber, prNumber)
	}
	if ended.PRURL != "https://github.com/example/repo/pull/77" {
		t.Fatalf("ended PR URL = %q, want PR URL", ended.PRURL)
	}

	history, err := database.GetSessionHistory(project.ID, 10)
	if err != nil {
		t.Fatalf("get session history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].PRNumber == nil || *history[0].PRNumber != prNumber {
		t.Fatalf("history PR number = %v, want %d", history[0].PRNumber, prNumber)
	}
}

func TestManagerStartSessionRejectsExistingActiveSession(t *testing.T) {
	mgr, _ := newTestManager(t)
	project := newTestProject(t, mgr)

	if _, err := mgr.StartSession(project.ID, "feature/one", "", "", "feature"); err != nil {
		t.Fatalf("first start session: %v", err)
	}

	if _, err := mgr.StartSession(project.ID, "feature/two", "", "", "feature"); err == nil {
		t.Fatal("second start session succeeded, want error")
	}
}

func TestManagerSyncActiveSessionCommitsElapsedAndKeepsSessionActive(t *testing.T) {
	mgr, _ := newTestManager(t)
	project := newTestProject(t, mgr)

	started, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	originalSliceStart := *started.CurrentSliceStart
	time.Sleep(10 * time.Millisecond)

	synced, err := mgr.SyncActiveSession(project.ID)
	if err != nil {
		t.Fatalf("sync active session: %v", err)
	}
	if synced.State != db.StateActive {
		t.Fatalf("synced session state = %q, want %q", synced.State, db.StateActive)
	}
	if synced.CurrentSliceStart == nil {
		t.Fatal("synced session missing current slice start")
	}
	if !synced.CurrentSliceStart.After(originalSliceStart) {
		t.Fatalf("synced current slice start = %v, want after %v", synced.CurrentSliceStart, originalSliceStart)
	}
	if synced.TotalElapsed <= 0 {
		t.Fatalf("synced total elapsed = %d, want > 0", synced.TotalElapsed)
	}
}

func TestManagerGetCurrentElapsedIncludesActiveSlice(t *testing.T) {
	mgr, _ := newTestManager(t)

	now := time.Now()
	session := &db.Session{
		State:             db.StateActive,
		TotalElapsed:      int64(2 * time.Second),
		CurrentSliceStart: ptrTime(now.Add(-150 * time.Millisecond)),
	}

	elapsed := mgr.GetCurrentElapsed(session)
	if elapsed < 2*time.Second+100*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least %v", elapsed, 2*time.Second+100*time.Millisecond)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
