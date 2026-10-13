package db

import (
	"reflect"
	"testing"
	"time"
)

func newTestSession(t *testing.T, database *DB, project *Project, branch, state string) *Session {
	t.Helper()
	start := time.Now().Add(-time.Hour)
	s, err := database.CreateSession(&Session{
		ProjectID: project.ID,
		Branch:    branch,
		State:     state,
		StartTime: start,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return s
}

func testEndWorkflow(s *Session) *EndWorkflow {
	return &EndWorkflow{
		SessionID:     s.ID,
		Branch:        s.Branch,
		BaseBranch:    "main",
		CommitMessage: "feat: checkpoint storage",
		PRTitle:       "Checkpoint storage",
		PRBody:        "Summary\n\nCloses #123",
		Phase:         EndPrepared,
		Head:          "initial-head",
	}
}

func TestEndWorkflowRoundTripAcrossReopen(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/checkpoint", StatePaused)
	w := testEndWorkflow(s)
	w.Repository = "github.com/example/repo"
	w.RemoteID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	w.EndWithoutPR = true
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}
	got, err := database.GetEndWorkflow(s.ID)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Fatalf("prepared checkpoint = %#v, err = %v, want %#v", got, err, w)
	}
	w.Phase = EndCommitted
	w.Head = "committed-head"
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatalf("save committed head: %v", err)
	}
	w.RebaseTarget = "0123456789abcdef0123456789abcdef01234567"
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatalf("save rebase target at committed phase: %v", err)
	}
	got, err = database.GetEndWorkflow(s.ID)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Fatalf("committed checkpoint = %#v, err = %v, want %#v", got, err, w)
	}
	prNumber := int64(42)
	for _, phase := range []string{EndCommitted, EndRebased, EndPushed, EndPRCreated, EndBaseCheckedOut} {
		w.Phase = phase
		w.Head = "committed-head"
		w.PRNumber = &prNumber
		w.PRURL = "https://github.com/example/repo/pull/42"
		if err := database.UpdateEndWorkflow(w); err != nil {
			t.Fatalf("advance to %s: %v", phase, err)
		}
	}
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatalf("retry equal phase: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close before reopen: %v", err)
	}
	reopened, err := Open(database.path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	got, err = reopened.GetEndWorkflow(s.ID)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Fatalf("reopened checkpoint = %#v, err = %v, want %#v", got, err, w)
	}
	got, err = reopened.GetEndWorkflow(s.ID + 1)
	if err != nil || got != nil {
		t.Fatalf("absent checkpoint = %#v, err = %v, want nil, nil", got, err)
	}
}

func TestCreateEndWorkflowRejectsInvalidSessionOrPhase(t *testing.T) {
	tests := []struct {
		name  string
		state string
		edit  func(*EndWorkflow)
	}{
		{name: "missing session", state: StateActive, edit: func(w *EndWorkflow) { w.SessionID++ }},
		{name: "wrong branch", state: StateActive, edit: func(w *EndWorkflow) { w.Branch = "other" }},
		{name: "ended session", state: StateEnded, edit: func(w *EndWorkflow) {}},
		{name: "unknown phase", state: StateActive, edit: func(w *EndWorkflow) { w.Phase = "unknown" }},
		{name: "not prepared", state: StateActive, edit: func(w *EndWorkflow) { w.Phase = EndCommitted }},
		{name: "nonempty rebase target", state: StateActive, edit: func(w *EndWorkflow) { w.RebaseTarget = "target" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", tt.state)
			w := testEndWorkflow(s)
			tt.edit(w)
			if err := database.CreateEndWorkflow(w); err == nil {
				t.Fatal("create checkpoint succeeded, want error")
			}
			got, err := database.GetEndWorkflow(w.SessionID)
			if err != nil || got != nil {
				t.Fatalf("rejected checkpoint = %#v, err = %v, want nil, nil", got, err)
			}
		})
	}
}

func TestEndWorkflowAllowsEmptyRepositoryIdentity(t *testing.T) {
	for _, name := range []string{"PR compatibility", "without PR"} {
		t.Run(name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			w := testEndWorkflow(s)
			w.EndWithoutPR = name == "without PR"
			if err := database.CreateEndWorkflow(w); err != nil {
				t.Fatalf("create with empty identity: %v", err)
			}
			w.Phase = EndCommitted
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatalf("update with empty identity: %v", err)
			}
			for _, field := range []string{"repository", "remote ID"} {
				changed := *w
				if field == "repository" {
					changed.Repository = "github.com/example/repo"
				} else {
					changed.RemoteID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
				}
				if err := database.UpdateEndWorkflow(&changed); err == nil {
					t.Fatalf("setting initially empty %s succeeded, want error", field)
				}
			}
			got, err := database.GetEndWorkflow(s.ID)
			if err != nil || !reflect.DeepEqual(got, w) {
				t.Fatalf("empty identity checkpoint = %#v, err = %v, want %#v", got, err, w)
			}
		})
	}
}

func TestCreateEndWorkflowRejectsDuplicateWithoutReplacingInputs(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/test", StateActive)
	w := testEndWorkflow(s)
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	duplicate := *w
	duplicate.CommitMessage = "replacement"
	if err := database.CreateEndWorkflow(&duplicate); err == nil {
		t.Fatal("duplicate creation succeeded, want error")
	}
	got, err := database.GetEndWorkflow(s.ID)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Fatalf("checkpoint after duplicate = %#v, err = %v, want %#v", got, err, w)
	}
}

func TestUpdateEndWorkflowRejectsInvalidStaleOrChangedInputs(t *testing.T) {
	tests := []struct {
		name string
		edit func(*EndWorkflow)
	}{
		{name: "session ID", edit: func(w *EndWorkflow) { w.SessionID++ }},
		{name: "branch", edit: func(w *EndWorkflow) { w.Branch = "other" }},
		{name: "base branch", edit: func(w *EndWorkflow) { w.BaseBranch = "dev" }},
		{name: "commit message", edit: func(w *EndWorkflow) { w.CommitMessage = "other" }},
		{name: "without PR", edit: func(w *EndWorkflow) { w.EndWithoutPR = true }},
		{name: "PR title", edit: func(w *EndWorkflow) { w.PRTitle = "other" }},
		{name: "PR body", edit: func(w *EndWorkflow) { w.PRBody = "other" }},
		{name: "repository", edit: func(w *EndWorkflow) { w.Repository = "github.com/other/repo" }},
		{name: "clear repository", edit: func(w *EndWorkflow) { w.Repository = "" }},
		{name: "remote ID", edit: func(w *EndWorkflow) {
			w.RemoteID = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
		}},
		{name: "clear remote ID", edit: func(w *EndWorkflow) { w.RemoteID = "" }},
		{name: "invalid phase", edit: func(w *EndWorkflow) { w.Phase = "unknown" }},
		{name: "stale phase", edit: func(w *EndWorkflow) { w.Phase = EndPrepared }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			w := testEndWorkflow(s)
			w.Repository = "github.com/example/repo"
			w.RemoteID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			if err := database.CreateEndWorkflow(w); err != nil {
				t.Fatal(err)
			}
			w.Phase = EndCommitted
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatal(err)
			}
			changed := *w
			changed.Phase = EndRebased
			changed.Head = "should-not-save"
			tt.edit(&changed)
			if err := database.UpdateEndWorkflow(&changed); err == nil {
				t.Fatal("invalid update succeeded, want error")
			}
			got, err := database.GetEndWorkflow(s.ID)
			if err != nil || !reflect.DeepEqual(got, w) {
				t.Fatalf("checkpoint after rejected update = %#v, err = %v, want %#v", got, err, w)
			}
		})
	}
}

func TestUpdateEndWorkflowRejectsMissingOrEndedSession(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/test", StateActive)
	w := testEndWorkflow(s)
	if err := database.UpdateEndWorkflow(w); err == nil {
		t.Fatal("update missing checkpoint succeeded, want error")
	}
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	s.State = StateEnded
	if err := database.UpdateSession(s); err != nil {
		t.Fatal(err)
	}
	w.Phase = EndCommitted
	if err := database.UpdateEndWorkflow(w); err == nil {
		t.Fatal("update ended session checkpoint succeeded, want error")
	}
}

func TestEndWorkflowRejectsNil(t *testing.T) {
	database := newTestDB(t)
	if err := database.CreateEndWorkflow(nil); err == nil {
		t.Fatal("create nil checkpoint succeeded, want error")
	}
	if err := database.UpdateEndWorkflow(nil); err == nil {
		t.Fatal("update nil checkpoint succeeded, want error")
	}
}

func TestUpdateEndWorkflowPreservesPushedHeadAndDiscoveredPRNumber(t *testing.T) {
	for _, phase := range []string{EndPushed, EndPRCreated, EndBaseCheckedOut} {
		t.Run(phase, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			w := testEndWorkflow(s)
			if err := database.CreateEndWorkflow(w); err != nil {
				t.Fatal(err)
			}
			number := int64(42)
			w.Phase = phase
			w.PRNumber = &number
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatalf("monotonic jump to %s: %v", phase, err)
			}
			changed := *w
			changed.Head = "different-head"
			if err := database.UpdateEndWorkflow(&changed); err == nil {
				t.Fatal("changing pushed head succeeded, want error")
			}
			changed = *w
			otherNumber := int64(99)
			changed.PRNumber = &otherNumber
			if err := database.UpdateEndWorkflow(&changed); err == nil {
				t.Fatal("changing PR number succeeded, want error")
			}
			changed.PRNumber = nil
			if err := database.UpdateEndWorkflow(&changed); err == nil {
				t.Fatal("clearing PR number succeeded, want error")
			}
			got, err := database.GetEndWorkflow(s.ID)
			if err != nil || !reflect.DeepEqual(got, w) {
				t.Fatalf("checkpoint changed = %#v, err = %v, want %#v", got, err, w)
			}
		})
	}
}

func TestUpdateEndWorkflowPreservesPRNumberBeforePush(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/test", StateActive)
	w := testEndWorkflow(s)
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	number := int64(42)
	w.PRNumber = &number
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatalf("save discovered PR at same phase: %v", err)
	}
	w.PRURL = "https://github.com/example/repo/pull/42"
	w.Head = "new-head-before-push"
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatalf("save URL and unpushed head at same phase: %v", err)
	}
	w.PRNumber = nil
	if err := database.UpdateEndWorkflow(w); err == nil {
		t.Fatal("clearing discovered PR before push succeeded, want error")
	}
}

func TestUpdateEndWorkflowRebaseTargetRequiresCommittedPhase(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/test", StateActive)
	w := testEndWorkflow(s)
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	changed := *w
	changed.RebaseTarget = "0123456789abcdef0123456789abcdef01234567"
	if err := database.UpdateEndWorkflow(&changed); err == nil {
		t.Fatal("setting rebase target at prepared phase succeeded, want error")
	}
	got, err := database.GetEndWorkflow(s.ID)
	if err != nil || !reflect.DeepEqual(got, w) {
		t.Fatalf("checkpoint after rejected update = %#v, err = %v, want %#v", got, err, w)
	}
}

func TestUpdateEndWorkflowRebaseTargetIsWriteOnce(t *testing.T) {
	for _, phase := range []string{EndCommitted, EndRebased, EndPushed, EndPRCreated, EndBaseCheckedOut} {
		t.Run(phase, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			w := testEndWorkflow(s)
			if err := database.CreateEndWorkflow(w); err != nil {
				t.Fatal(err)
			}
			w.Phase = phase
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatalf("advance to %s: %v", phase, err)
			}
			w.RebaseTarget = "0123456789abcdef0123456789abcdef01234567"
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatalf("save target at %s: %v", phase, err)
			}
			if err := database.UpdateEndWorkflow(w); err != nil {
				t.Fatalf("retry unchanged target at %s: %v", phase, err)
			}
			for _, nextPhase := range []string{phase, EndBaseCheckedOut} {
				for _, target := range []string{"", "abcdef0123456789abcdef0123456789abcdef01"} {
					changed := *w
					changed.Phase = nextPhase
					changed.RebaseTarget = target
					if err := database.UpdateEndWorkflow(&changed); err == nil {
						t.Fatalf("changing target to %q at %s succeeded, want error", target, nextPhase)
					}
					got, err := database.GetEndWorkflow(s.ID)
					if err != nil || !reflect.DeepEqual(got, w) {
						t.Fatalf("checkpoint after rejected update = %#v, err = %v, want %#v", got, err, w)
					}
				}
			}
		})
	}
}
