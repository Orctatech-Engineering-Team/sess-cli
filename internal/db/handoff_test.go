package db

import (
	"strings"
	"testing"
	"time"
)

func TestHandoffPersistsAndSupersedesWithoutRewriting(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	session := newTestSession(t, database, project, "feature/handoff", StateActive)
	now := time.Date(2026, 10, 18, 10, 30, 0, 0, time.UTC)
	participant := Participant{ID: "participant:handoff", Kind: ParticipantAgent, IdentitySource: "configured", CreatedAt: now}
	if err := database.CreateParticipant(&participant); err != nil {
		t.Fatal(err)
	}
	attempt := &Attempt{
		ID: "attempt:handoff", SessionID: session.ID, Participant: participant,
		Role: "implementation", State: AttemptActive, StartedAt: now,
		StartRepositoryState: RepositoryState{HeadOID: strings.Repeat("a", 40), Worktree: "clean", CapturedAt: now},
	}
	if err := database.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	check := &Check{
		ID: "check:handoff", AttemptID: attempt.ID, Name: "tests",
		ProcedureType: CheckCommand, ProcedureValue: "go test ./...",
		Outcome: CheckPassed, StartedAt: now, FinishedAt: now,
		RepositoryState: RepositoryState{HeadOID: strings.Repeat("a", 40), Worktree: "clean", CapturedAt: now},
		Reproducibility: "revision",
	}
	if err := database.CreateCheck(check); err != nil {
		t.Fatal(err)
	}
	first := &Handoff{
		ID: "handoff:first", SessionID: session.ID, AttemptID: attempt.ID, CreatedAt: now,
		RepositoryState: check.RepositoryState, Completed: []string{"Implemented the change."},
		Remaining: []string{"Review the change."}, Risks: []string{"None known."},
		NextAction: "Run review.", CheckIDs: []string{check.ID},
	}
	if err := database.CreateHandoff(first); err != nil {
		t.Fatal(err)
	}
	second := &Handoff{
		ID: "handoff:second", SessionID: session.ID, AttemptID: attempt.ID,
		CreatedAt: now.Add(time.Minute), RepositoryState: check.RepositoryState,
		Completed: []string{"Implemented the change.", "Addressed review."},
		Remaining: []string{"Merge the pull request."}, Risks: []string{"None known."},
		NextAction: "Merge the pull request.", CheckIDs: []string{check.ID},
		SupersedesID: first.ID,
	}
	if err := database.CreateHandoff(second); err != nil {
		t.Fatal(err)
	}
	handoffs, err := database.ListHandoffsForSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(handoffs) != 2 || handoffs[0].ID != first.ID || handoffs[1].SupersedesID != first.ID {
		t.Fatalf("handoffs = %#v", handoffs)
	}
	if err := database.CreateHandoff(&Handoff{
		ID: "handoff:bad", SessionID: session.ID, AttemptID: attempt.ID,
		CreatedAt: now.Add(2 * time.Minute), RepositoryState: check.RepositoryState,
		Completed: []string{"bad"}, NextAction: "bad", CheckIDs: []string{"check:missing"},
	}); err == nil {
		t.Fatal("handoff accepted missing check")
	}
}
