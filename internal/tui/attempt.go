package tui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

type AttemptStartInput struct {
	ParticipantID    string
	ParticipantKind  string
	ParticipantLabel string
	IdentitySource   string
	Role             string
	Claims           []db.AttemptClaim
}

type AttemptStartResult struct {
	Attempt  *db.Attempt
	Warnings []string
}

func RunAttemptStart(cwd string, database *db.DB, input AttemptStartInput) (*AttemptStartResult, error) {
	ctx := context.Background()
	manager := session.NewManager(database)
	project, err := manager.GetProject(cwd)
	if err != nil {
		return nil, fmt.Errorf("get tracked project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("not a tracked project. Run 'sess start' first")
	}
	current, err := manager.GetActiveSession(project.ID)
	if err != nil {
		return nil, fmt.Errorf("get current session: %w", err)
	}
	if current == nil {
		return nil, fmt.Errorf("no active or paused session")
	}

	now := time.Now().UTC()
	participantID := strings.TrimSpace(input.ParticipantID)
	if participantID == "" {
		participantID, err = newProtocolID("participant")
		if err != nil {
			return nil, err
		}
		input.IdentitySource = "ephemeral"
	} else if input.IdentitySource == "" {
		input.IdentitySource = "configured"
	}
	if input.ParticipantKind == "" {
		input.ParticipantKind = db.ParticipantHuman
	}
	if input.Role == "" {
		input.Role = "implementation"
	}
	participant := db.Participant{
		ID: participantID, Kind: input.ParticipantKind, Label: input.ParticipantLabel,
		IdentitySource: input.IdentitySource, CreatedAt: now, UpdatedAt: now,
	}
	if err := database.CreateParticipant(&participant); err != nil {
		return nil, err
	}
	repositoryState, err := attemptRepositoryState(ctx, cwd, now)
	if err != nil {
		return nil, err
	}
	attemptID, err := newProtocolID("attempt")
	if err != nil {
		return nil, err
	}
	attempt := &db.Attempt{
		ID: attemptID, SessionID: current.ID, Participant: participant,
		Role: input.Role, State: db.AttemptActive, StartedAt: now, UpdatedAt: now,
		StartRepositoryState: repositoryState, Claims: input.Claims,
	}
	active, err := database.ListActiveAttempts(current.ID)
	if err != nil {
		return nil, fmt.Errorf("list active attempts: %w", err)
	}
	warnings := overlappingClaimWarnings(active, input.Claims)
	if err := database.CreateAttempt(attempt); err != nil {
		return nil, err
	}
	return &AttemptStartResult{Attempt: attempt, Warnings: warnings}, nil
}

func RunAttemptTransition(
	cwd string,
	database *db.DB,
	attemptID, targetState, reason string,
) (*db.Attempt, error) {
	ctx := context.Background()
	attempt, err := database.GetAttempt(attemptID)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, fmt.Errorf("attempt %s not found", attemptID)
	}
	manager := session.NewManager(database)
	project, err := manager.GetProject(cwd)
	if err != nil {
		return nil, fmt.Errorf("get tracked project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("not a tracked project")
	}
	current, err := manager.GetActiveSession(project.ID)
	if err != nil {
		return nil, fmt.Errorf("get current session: %w", err)
	}
	if current == nil || current.ID != attempt.SessionID {
		return nil, fmt.Errorf("attempt %s does not belong to the current session", attemptID)
	}
	before := *attempt
	after := *attempt
	after.State = targetState
	after.UpdatedAt = time.Now().UTC()
	after.BlockedReason = reason
	if targetState == db.AttemptActive {
		after.BlockedReason = ""
	}
	if targetState == db.AttemptCompleted || targetState == db.AttemptCancelled || targetState == db.AttemptFailed {
		finished := after.UpdatedAt
		after.FinishedAt = &finished
		state, stateErr := attemptRepositoryState(ctx, cwd, finished)
		if stateErr != nil {
			return nil, stateErr
		}
		after.ResultRepositoryState = &state
	}
	if err := database.TransitionAttempt(&before, &after); err != nil {
		return nil, err
	}
	return &after, nil
}

func attemptRepositoryState(ctx context.Context, cwd string, capturedAt time.Time) (db.RepositoryState, error) {
	state := db.RepositoryState{Worktree: "unknown", CapturedAt: capturedAt}
	branch, err := git.CurrentBranch(ctx, cwd)
	if err != nil {
		ref, refErr := git.RunCombined(ctx, cwd, "symbolic-ref", "--quiet", "HEAD")
		if refErr != nil {
			return state, fmt.Errorf("read current branch: %w", err)
		}
		branch = strings.TrimPrefix(ref, "refs/heads/")
	}
	if branch != "" && branch != "HEAD" {
		state.Branch = branch
	}
	head, err := git.RunCombined(ctx, cwd, "rev-parse", "--verify", "HEAD")
	if err == nil {
		state.HeadOID = head
	} else if ctx.Err() != nil {
		return state, ctx.Err()
	}
	dirty, err := git.IsDirty(ctx, cwd)
	if err != nil {
		return state, fmt.Errorf("read worktree state: %w", err)
	}
	if dirty {
		state.Worktree = "changed"
	} else {
		state.Worktree = "clean"
	}
	return state, nil
}

func overlappingClaimWarnings(active []*db.Attempt, claims []db.AttemptClaim) []string {
	warnings := make([]string, 0)
	for _, existing := range active {
		for _, existingClaim := range existing.Claims {
			for _, claim := range claims {
				if claimsOverlap(existingClaim, claim) {
					warnings = append(warnings,
						fmt.Sprintf("claim overlaps attempt %s on %s %q", existing.ID, claim.Type, claim.Value))
				}
			}
		}
	}
	return warnings
}

func claimsOverlap(existing, requested db.AttemptClaim) bool {
	if existing.Type != requested.Type || existing.Value == requested.Value {
		return existing.Type == requested.Type && existing.Value == requested.Value
	}
	if existing.Type != "path" {
		return false
	}
	return pathClaimContains(existing.Value, requested.Value) ||
		pathClaimContains(requested.Value, existing.Value)
}

func pathClaimContains(pattern, value string) bool {
	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "**"))
	}
	return false
}

func newProtocolID(kind string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate %s ID: %w", kind, err)
	}
	return kind + ":" + hex.EncodeToString(random[:]), nil
}
