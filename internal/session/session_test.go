package session

import (
	"database/sql"
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
	mgr, database := newTestManager(t)
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
	assertSessionEventKinds(t, database, started.ID, []string{
		db.EventSessionStarted, db.EventSessionPaused, db.EventSessionResumed, db.EventSessionEnded,
	})
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

func TestManagerCompleteSessionClearsCheckpointAndPreservesPausedElapsed(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)
	start := time.Now().Add(-time.Hour)
	elapsed := int64(10 * time.Minute)
	s, err := database.CreateSession(&db.Session{
		ProjectID:    project.ID,
		Branch:       "feature/paused",
		State:        db.StatePaused,
		StartTime:    start,
		TotalElapsed: elapsed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.CreateEndWorkflow(&db.EndWorkflow{
		SessionID:    s.ID,
		Branch:       s.Branch,
		BaseBranch:   "main",
		EndWithoutPR: true,
		Phase:        db.EndPrepared,
	}); err != nil {
		t.Fatal(err)
	}
	ended, err := mgr.CompleteSession(project.ID, nil, "")
	if err != nil {
		t.Fatalf("complete paused session: %v", err)
	}
	if ended.TotalElapsed != elapsed || ended.State != db.StateEnded || ended.CurrentSliceStart != nil {
		t.Fatalf("completed paused session = %#v, want unchanged elapsed %d", ended, elapsed)
	}
	w, err := database.GetEndWorkflow(s.ID)
	if err != nil || w != nil {
		t.Fatalf("checkpoint after manager completion = %#v, err = %v, want nil", w, err)
	}
	assertSessionEventKinds(t, database, s.ID, []string{db.EventSessionEnded})
}

func TestManagerCompleteEndWorkflowUsesSavedSessionIdentity(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)
	start := time.Now().Add(-time.Minute).UTC()
	elapsed := int64(time.Second)
	s, err := database.CreateSession(&db.Session{
		ProjectID:         project.ID,
		Branch:            "feature/end",
		State:             db.StateActive,
		StartTime:         start,
		CurrentSliceStart: &start,
		TotalElapsed:      elapsed,
	})
	if err != nil {
		t.Fatal(err)
	}
	number := int64(42)
	w := &db.EndWorkflow{
		SessionID:  s.ID,
		Branch:     s.Branch,
		BaseBranch: "main",
		Phase:      db.EndPrepared,
		PRNumber:   &number,
		PRURL:      "https://github.com/example/repo/pull/42",
	}
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	w.Phase = db.EndBaseCheckedOut
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	ended, err := mgr.CompleteEndWorkflow(project.ID, w)
	if err != nil {
		t.Fatalf("complete workflow: %v", err)
	}
	wantElapsed := elapsed + ended.EndTime.Sub(start).Nanoseconds()
	if ended.ID != s.ID || ended.State != db.StateEnded || ended.TotalElapsed != wantElapsed ||
		ended.PRNumber == nil || *ended.PRNumber != number || ended.PRURL != w.PRURL {
		t.Fatalf("completed session = %#v, want ID %d, elapsed %d and saved PR", ended, s.ID, wantElapsed)
	}
	saved, err := database.GetEndWorkflow(s.ID)
	if err != nil || saved != nil {
		t.Fatalf("checkpoint after completion = %#v, err = %v, want nil", saved, err)
	}
	assertSessionEventKinds(t, database, s.ID, []string{db.EventSessionEnded})
}

func TestManagerCompleteEndWorkflowRejectsStaleOrMissingCheckpoint(t *testing.T) {
	tests := []struct {
		name string
		edit func(*db.EndWorkflow)
		save bool
	}{
		{name: "missing", edit: func(w *db.EndWorkflow) {}, save: false},
		{name: "session changed", edit: func(w *db.EndWorkflow) { w.SessionID++ }, save: true},
		{name: "branch changed", edit: func(w *db.EndWorkflow) { w.Branch = "other" }, save: true},
		{name: "stale phase", edit: func(w *db.EndWorkflow) { w.Phase = db.EndCommitted }, save: true},
		{name: "stale PR number", edit: func(w *db.EndWorkflow) { n := int64(99); w.PRNumber = &n }, save: true},
		{name: "nil", edit: func(w *db.EndWorkflow) {}, save: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, database := newTestManager(t)
			project := newTestProject(t, mgr)
			s, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
			if err != nil {
				t.Fatal(err)
			}
			w := &db.EndWorkflow{SessionID: s.ID, Branch: s.Branch, BaseBranch: "main", Phase: db.EndPrepared}
			if tt.save {
				if err := database.CreateEndWorkflow(w); err != nil {
					t.Fatal(err)
				}
			}
			tt.edit(w)
			if tt.name == "nil" {
				w = nil
			}
			if _, err := mgr.CompleteEndWorkflow(project.ID, w); err == nil {
				t.Fatal("invalid workflow completion succeeded, want error")
			}
			active, err := mgr.GetActiveSession(project.ID)
			if err != nil || active == nil || active.ID != s.ID || active.State != db.StateActive {
				t.Fatalf("active session changed: %#v, err = %v", active, err)
			}
			assertSessionEventKinds(t, database, s.ID, []string{db.EventSessionStarted})
		})
	}
}

func TestManagerCompleteEndWorkflowDoesNotEndReplacementSession(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)
	s, err := mgr.StartSession(project.ID, "feature/original", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	w := &db.EndWorkflow{SessionID: s.ID, Branch: s.Branch, BaseBranch: "main", Phase: db.EndPrepared}
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.EndSession(project.ID); err != nil {
		t.Fatal(err)
	}
	replacement, err := mgr.StartSession(project.ID, "feature/replacement", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.CompleteEndWorkflow(project.ID, w); err == nil {
		t.Fatal("completion of stale workflow succeeded, want error")
	}
	active, err := mgr.GetActiveSession(project.ID)
	if err != nil || active == nil || active.ID != replacement.ID {
		t.Fatalf("replacement session lost: %#v, err = %v", active, err)
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

func assertSessionEventKinds(t *testing.T, database *db.DB, sessionID int64, want []string) {
	t.Helper()
	events, err := database.ListSessionEvents(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d (%v)", len(events), len(want), want)
	}
	for i, event := range events {
		if event.Kind != want[i] {
			t.Errorf("event %d kind = %q, want %q", i, event.Kind, want[i])
		}
		if i > 0 && event.ID <= events[i-1].ID {
			t.Errorf("event %d ID = %d, want after %d", i, event.ID, events[i-1].ID)
		}
	}
}

func TestManagerRejectedRepetitionsAndSyncDoNotEmitEvents(t *testing.T) {
	mgr, database := newTestManager(t)
	project := newTestProject(t, mgr)
	s, err := mgr.StartSession(project.ID, "feature/journal", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.StartSession(project.ID, "feature/duplicate", "", "", "feature"); err == nil {
		t.Fatal("repeated start succeeded")
	}
	if _, err := mgr.ResumeSession(project.ID); err == nil {
		t.Fatal("resume active session succeeded")
	}
	if _, err := mgr.SyncActiveSession(project.ID); err != nil {
		t.Fatal(err)
	}
	assertSessionEventKinds(t, database, s.ID, []string{db.EventSessionStarted})
	if _, err := mgr.PauseSession(project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.PauseSession(project.ID); err == nil {
		t.Fatal("repeated pause succeeded")
	}
	assertSessionEventKinds(t, database, s.ID, []string{db.EventSessionStarted, db.EventSessionPaused})
	if _, err := mgr.ResumeSession(project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.ResumeSession(project.ID); err == nil {
		t.Fatal("repeated resume succeeded")
	}
	if _, err := mgr.EndSession(project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.EndSession(project.ID); err == nil {
		t.Fatal("repeated end succeeded")
	}
	assertSessionEventKinds(t, database, s.ID, []string{
		db.EventSessionStarted, db.EventSessionPaused, db.EventSessionResumed, db.EventSessionEnded,
	})
}

func TestManagerEventWriteFailureRollsBack(t *testing.T) {
	tests := []struct {
		name     string
		complete bool
		workflow bool
	}{
		{name: "pause"},
		{name: "end", complete: true},
		{name: "end workflow", complete: true, workflow: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sess.db")
			database, err := db.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := database.Close(); err != nil {
					t.Error(err)
				}
			})
			mgr := NewManager(database)
			project := newTestProject(t, mgr)
			started, err := mgr.StartSession(project.ID, "feature/rollback", "", "", "feature")
			if err != nil {
				t.Fatal(err)
			}
			before, err := database.GetSessionByID(started.ID)
			if err != nil {
				t.Fatal(err)
			}
			projectBefore, err := database.GetProjectByID(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			w := &db.EndWorkflow{
				SessionID: started.ID, Branch: started.Branch, BaseBranch: "main",
				Phase: db.EndPrepared, EndWithoutPR: true,
			}
			if tt.complete {
				if err := database.CreateEndWorkflow(w); err != nil {
					t.Fatal(err)
				}
			}
			conn, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := conn.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := conn.Exec(`
				CREATE TRIGGER fail_journal BEFORE INSERT ON session_events
				BEGIN SELECT RAISE(ABORT, 'test event failure'); END
			`); err != nil {
				t.Fatal(err)
			}
			var result *db.Session
			switch {
			case tt.workflow:
				result, err = mgr.CompleteEndWorkflow(project.ID, w)
			case tt.complete:
				result, err = mgr.CompleteSession(project.ID, nil, "")
			default:
				result, err = mgr.PauseSession(project.ID)
			}
			if err == nil || result != nil {
				t.Fatalf("failed journal returned session %#v, err = %v", result, err)
			}
			after, err := database.GetSessionByID(started.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.State != before.State || after.TotalElapsed != before.TotalElapsed ||
				!after.CurrentSliceStart.Equal(*before.CurrentSliceStart) || after.EndTime != nil {
				t.Fatalf("failed journal mutated session: %#v", after)
			}
			projectAfter, err := database.GetProjectByID(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !projectAfter.LastUsedAt.Equal(projectBefore.LastUsedAt) {
				t.Fatal("failed journal touched project")
			}
			assertSessionEventKinds(t, database, started.ID, []string{db.EventSessionStarted})
			if tt.complete {
				saved, err := database.GetEndWorkflow(started.ID)
				if err != nil || saved == nil {
					t.Fatalf("failed completion lost checkpoint: %#v, err = %v", saved, err)
				}
			}
		})
	}
}
