package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

type HandoffCreateInput struct {
	AttemptID    string
	Completed    []string
	Remaining    []string
	Risks        []string
	NextAction   string
	CheckIDs     []string
	SupersedesID string
}

func RunHandoffCreate(cwd string, database *db.DB, input HandoffCreateInput) (*db.Handoff, error) {
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
	if current == nil {
		return nil, fmt.Errorf("no active or paused session")
	}
	attempt, err := database.GetAttempt(input.AttemptID)
	if err != nil {
		return nil, err
	}
	if attempt == nil || attempt.SessionID != current.ID {
		return nil, fmt.Errorf("attempt %s does not belong to the current session", input.AttemptID)
	}
	if strings.TrimSpace(input.NextAction) == "" {
		return nil, fmt.Errorf("next action is required")
	}
	now := time.Now().UTC()
	repositoryState, err := attemptRepositoryState(context.Background(), cwd, now)
	if err != nil {
		return nil, err
	}
	id, err := newProtocolID("handoff")
	if err != nil {
		return nil, err
	}
	handoff := &db.Handoff{
		ID: id, SessionID: current.ID, AttemptID: input.AttemptID,
		CreatedAt: now, RepositoryState: repositoryState,
		Completed: input.Completed, Remaining: input.Remaining, Risks: input.Risks,
		NextAction: input.NextAction, CheckIDs: input.CheckIDs,
		SupersedesID: input.SupersedesID,
	}
	if err := database.CreateHandoff(handoff); err != nil {
		return nil, err
	}
	return handoff, nil
}
