//go:build integration

package tui

import (
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

func TestAttemptLifecycleRecordsIdentityClaimsAndTransitions(t *testing.T) {
	repo, database, manager, project := newEndTestRepo(t)
	started, err := manager.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}

	result, err := RunAttemptStart(repo, database, AttemptStartInput{
		ParticipantID:    "participant:reviewer",
		ParticipantKind:  db.ParticipantAgent,
		ParticipantLabel: "reviewer",
		IdentitySource:   "configured",
		Role:             "review",
		Claims:           []db.AttemptClaim{{Type: "path", Value: "internal/db/**"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Attempt == nil || result.Attempt.SessionID != started.ID ||
		result.Attempt.Participant.ID != "participant:reviewer" ||
		len(result.Attempt.Claims) != 1 {
		t.Fatalf("started attempt = %#v", result.Attempt)
	}
	overlap, err := RunAttemptStart(repo, database, AttemptStartInput{
		ParticipantID:   "participant:implementer",
		ParticipantKind: db.ParticipantAgent,
		IdentitySource:  "orchestrator",
		Role:            "implementation",
		Claims:          []db.AttemptClaim{{Type: "path", Value: "internal/db/**"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(overlap.Warnings) != 1 {
		t.Fatalf("overlap warnings = %#v, want one warning", overlap.Warnings)
	}

	blocked, err := RunAttemptTransition(repo, database, result.Attempt.ID, db.AttemptBlocked, "waiting")
	if err != nil {
		t.Fatal(err)
	}
	if blocked.State != db.AttemptBlocked {
		t.Fatalf("blocked state = %q", blocked.State)
	}
	resumed, err := RunAttemptTransition(repo, database, result.Attempt.ID, db.AttemptActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if resumed.State != db.AttemptActive {
		t.Fatalf("resumed state = %q", resumed.State)
	}
	finished, err := RunAttemptTransition(repo, database, result.Attempt.ID, db.AttemptCompleted, "")
	if err != nil {
		t.Fatal(err)
	}
	if finished.State != db.AttemptCompleted || finished.FinishedAt == nil ||
		finished.ResultRepositoryState == nil {
		t.Fatalf("finished attempt = %#v", finished)
	}

	if _, err := session.NewManager(database).GetActiveSession(project.ID); err != nil {
		t.Fatal(err)
	}
}
