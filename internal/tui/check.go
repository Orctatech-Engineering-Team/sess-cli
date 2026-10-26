package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

type CheckRecordInput struct {
	AttemptID      string
	Name           string
	ProcedureType  string
	ProcedureValue string
	Outcome        string
	Summary        string
	ExitCode       *int
}

func RunCheckRecord(cwd string, database *db.DB, input CheckRecordInput) (*db.Check, error) {
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
	now := time.Now().UTC()
	state, err := attemptRepositoryState(context.Background(), cwd, now)
	if err != nil {
		return nil, err
	}
	id, err := newProtocolID("check")
	if err != nil {
		return nil, err
	}
	check := &db.Check{
		ID: id, AttemptID: input.AttemptID, SessionID: current.ID,
		Name: input.Name, ProcedureType: input.ProcedureType,
		ProcedureValue: input.ProcedureValue, Outcome: input.Outcome,
		StartedAt: now, FinishedAt: now, ExitCode: input.ExitCode,
		RepositoryState: state, Summary: input.Summary,
	}
	if state.Worktree == "clean" && state.HeadOID != "" {
		check.Reproducibility = "revision"
	} else if state.Worktree == "changed" {
		check.Reproducibility = "workspace"
	} else {
		check.Reproducibility = "unknown"
	}
	if err := database.CreateCheck(check); err != nil {
		return nil, err
	}
	return check, nil
}
