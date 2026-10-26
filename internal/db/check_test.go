package db

import (
	"strings"
	"testing"
	"time"
)

func TestCheckPersistsImmutableRevisionEvidence(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	session := newTestSession(t, database, project, "feature/check", StateActive)
	now := time.Date(2026, 10, 18, 10, 30, 0, 0, time.UTC)
	participant := Participant{ID: "participant:check", Kind: ParticipantAgent, IdentitySource: "configured", CreatedAt: now}
	if err := database.CreateParticipant(&participant); err != nil {
		t.Fatal(err)
	}
	attempt := &Attempt{
		ID: "attempt:check", SessionID: session.ID, Participant: participant,
		Role: "implementation", State: AttemptActive, StartedAt: now,
		StartRepositoryState: RepositoryState{HeadOID: strings.Repeat("a", 40), Worktree: "clean", CapturedAt: now},
	}
	if err := database.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Minute)
	exitCode := 0
	check := &Check{
		ID: "check:test", AttemptID: attempt.ID, Name: "Go tests",
		ProcedureType: CheckCommand, ProcedureValue: "go test ./...",
		Outcome: CheckPassed, StartedAt: now, FinishedAt: finished,
		ExitCode: &exitCode, RepositoryState: RepositoryState{
			HeadOID: strings.Repeat("b", 40), Worktree: "clean", CapturedAt: finished,
		},
		Reproducibility: "revision", Summary: "All packages passed.",
	}
	if err := database.CreateCheck(check); err != nil {
		t.Fatal(err)
	}
	got, err := database.GetCheck(check.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Outcome != CheckPassed || got.ExitCode == nil || *got.ExitCode != 0 ||
		got.RepositoryState.HeadOID != strings.Repeat("b", 40) {
		t.Fatalf("check = %#v", got)
	}
	if err := database.CreateCheck(check); err == nil {
		t.Fatal("duplicate check succeeded")
	}
	check.Summary = strings.Repeat("x", 4097)
	if err := database.CreateCheck(&Check{
		ID: "check:oversized", AttemptID: attempt.ID, Name: "bad",
		ProcedureType: CheckCommand, ProcedureValue: "go test",
		Outcome: CheckPassed, StartedAt: now, FinishedAt: finished,
		RepositoryState: RepositoryState{Worktree: "clean", CapturedAt: finished},
		Reproducibility: "revision", Summary: check.Summary,
	}); err == nil {
		t.Fatal("oversized summary succeeded")
	}
	list, err := database.ListChecksForSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != check.ID {
		t.Fatalf("checks = %#v", list)
	}
}
