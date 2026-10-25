package db

import (
	"strings"
	"testing"
	"time"
)

func TestAttemptLifecycleAndParticipantIdentity(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	session := newTestSession(t, database, project, "feature/attempt", StateActive)
	now := time.Date(2026, 10, 18, 10, 30, 0, 0, time.UTC)
	participant := Participant{
		ID:             "participant:test",
		Kind:           ParticipantAgent,
		Label:          "reviewer",
		IdentitySource: "configured",
		CreatedAt:      now,
	}
	if err := database.CreateParticipant(&participant); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateParticipant(&Participant{
		ID:             participant.ID,
		Kind:           ParticipantAgent,
		Label:          "different",
		IdentitySource: "configured",
	}); err == nil {
		t.Fatal("CreateParticipant() accepted conflicting identity")
	}

	attempt := &Attempt{
		ID:          "attempt:test",
		SessionID:   session.ID,
		Participant: participant,
		Role:        "implementation",
		State:       AttemptActive,
		StartedAt:   now,
		StartRepositoryState: RepositoryState{
			Repository: "github.com/example/repo",
			Branch:     "feature/attempt",
			HeadOID:    strings.Repeat("a", 40),
			Worktree:   "clean",
			CapturedAt: now,
		},
		Claims: []AttemptClaim{
			{Type: "path", Value: "internal/db/**", CreatedAt: now},
		},
	}
	if err := database.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	got, err := database.GetAttempt(attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Participant.ID != participant.ID || len(got.Claims) != 1 {
		t.Fatalf("attempt = %#v", got)
	}
	active, err := database.ListActiveAttempts(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != attempt.ID {
		t.Fatalf("active attempts = %#v", active)
	}

	before := *got
	blocked := *got
	blocked.State = AttemptBlocked
	blocked.BlockedReason = "waiting for review"
	blocked.UpdatedAt = now.Add(time.Minute)
	if err := database.TransitionAttempt(&before, &blocked); err != nil {
		t.Fatal(err)
	}
	resumed := blocked
	resumed.State = AttemptActive
	resumed.BlockedReason = ""
	resumed.UpdatedAt = now.Add(2 * time.Minute)
	if err := database.TransitionAttempt(&blocked, &resumed); err != nil {
		t.Fatal(err)
	}
	ended := resumed
	ended.State = AttemptCompleted
	finished := now.Add(3 * time.Minute)
	ended.FinishedAt = &finished
	ended.UpdatedAt = finished
	ended.ResultRepositoryState = &RepositoryState{
		Repository: "github.com/example/repo",
		Branch:     "feature/attempt",
		HeadOID:    strings.Repeat("b", 40),
		Worktree:   "clean",
		CapturedAt: finished,
	}
	if err := database.TransitionAttempt(&resumed, &ended); err != nil {
		t.Fatal(err)
	}
	if active, err := database.ListActiveAttempts(session.ID); err != nil {
		t.Fatal(err)
	} else if len(active) != 0 {
		t.Fatalf("active attempts after completion = %#v", active)
	}
	if err := database.TransitionAttempt(&ended, &resumed); err == nil {
		t.Fatal("TransitionAttempt() reopened terminal attempt")
	}
}

func TestAttemptRejectsStaleTransitionAndRollsBackClaims(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	session := newTestSession(t, database, project, "feature/attempt", StatePaused)
	now := time.Date(2026, 10, 18, 10, 30, 0, 0, time.UTC)
	participant := Participant{ID: "participant:test", Kind: ParticipantHuman, IdentitySource: "ephemeral", CreatedAt: now}
	if err := database.CreateParticipant(&participant); err != nil {
		t.Fatal(err)
	}
	attempt := &Attempt{
		ID: "attempt:test", SessionID: session.ID, Participant: participant,
		Role: "review", State: AttemptActive, StartedAt: now,
		StartRepositoryState: RepositoryState{Worktree: "unknown", CapturedAt: now},
	}
	if err := database.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	before, err := database.GetAttempt(attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	first := *before
	first.State = AttemptBlocked
	first.BlockedReason = "first"
	first.UpdatedAt = now.Add(time.Minute)
	if err := database.TransitionAttempt(before, &first); err != nil {
		t.Fatal(err)
	}
	stale := *before
	stale.State = AttemptCompleted
	finished := now.Add(2 * time.Minute)
	stale.FinishedAt = &finished
	if err := database.TransitionAttempt(&stale, &stale); err == nil {
		t.Fatal("stale transition succeeded")
	}
	current, err := database.GetAttempt(attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != AttemptBlocked || current.BlockedReason != "first" {
		t.Fatalf("stale transition changed attempt: %#v", current)
	}
}
