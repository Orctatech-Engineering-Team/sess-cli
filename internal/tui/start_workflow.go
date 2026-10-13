package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

type startWorkflowConfig struct {
	cwd        string
	project    *db.Project
	branch     string
	branchType string
	issueID    string
	issueTitle string
	adopt      bool
}

func checkStartRebase(ctx context.Context, cwd string) error {
	inProgress, err := git.RebaseInProgress(ctx, cwd)
	if err != nil {
		return fmt.Errorf("check rebase state before starting: %w", err)
	}
	if inProgress {
		return fmt.Errorf("rebase in progress; finish or abort it with Git before starting a session")
	}
	return nil
}

func executeStartWorkflow(
	ctx context.Context,
	database *db.DB,
	cfg startWorkflowConfig,
	send func(tea.Msg),
) (*db.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.project == nil || cfg.project.BaseBranch == "" {
		return nil, fmt.Errorf("start requires a tracked project with a base branch")
	}
	if cfg.branch == cfg.project.BaseBranch {
		return nil, fmt.Errorf("cannot start a session on base branch %s", cfg.branch)
	}
	if err := checkStartRebase(ctx, cfg.cwd); err != nil {
		return nil, err
	}
	head, err := git.LocalBranchHead(ctx, cfg.cwd, cfg.branch)
	if err != nil {
		return nil, fmt.Errorf("check session branch: %w", err)
	}
	manager := session.NewManager(database)
	existing, err := manager.GetActiveSession(cfg.project.ID)
	if err != nil {
		return nil, fmt.Errorf("check existing session: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("session already %s on %s; inspect 'sess status' before retrying", existing.State, existing.Branch)
	}
	if cfg.adopt && head == "" {
		return nil, fmt.Errorf("local branch %s does not exist; adoption does not create or fetch branches", cfg.branch)
	}
	if !cfg.adopt && head != "" {
		return nil, fmt.Errorf("branch %s already exists; inspect it, then use 'sess start --adopt %s'", cfg.branch, cfg.branch)
	}

	dirty, err := git.IsDirty(ctx, cfg.cwd)
	if err != nil {
		return nil, fmt.Errorf("check worktree before starting: %w", err)
	}
	current, err := git.CurrentBranch(ctx, cfg.cwd)
	if err != nil {
		return nil, fmt.Errorf("check current branch before starting: %w", err)
	}
	if cfg.adopt {
		if current != cfg.branch {
			if dirty {
				return nil, fmt.Errorf("uncommitted changes on another branch; switch to %s manually before adopting", cfg.branch)
			}
			if err := runGitStep(ctx, send, cfg.cwd, "checkout", "--no-overwrite-ignore", cfg.branch); err != nil {
				return nil, fmt.Errorf("switch to existing branch %s: %w", cfg.branch, err)
			}
		}
	} else {
		if dirty {
			return nil, fmt.Errorf("worktree changed before starting; resolve uncommitted changes and retry")
		}
		if err := runGitStep(ctx, send, cfg.cwd, "checkout", "--no-overwrite-ignore", cfg.project.BaseBranch); err != nil {
			return nil, fmt.Errorf("checkout base branch %s: %w", cfg.project.BaseBranch, err)
		}
		if err := runGitStep(ctx, send, cfg.cwd, "fetch", "origin", cfg.project.BaseBranch); err != nil {
			return nil, fmt.Errorf("fetch base branch %s: %w", cfg.project.BaseBranch, err)
		}
		if err := runGitStep(ctx, send, cfg.cwd, "merge", "--ff-only", "--no-overwrite-ignore", "--no-autostash", "FETCH_HEAD"); err != nil {
			return nil, fmt.Errorf("update base branch %s: %w", cfg.project.BaseBranch, err)
		}
		head, err = git.RunCombined(ctx, cfg.cwd, "rev-parse", "--verify", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("read base head before creating branch: %w", err)
		}
		if err := runGitStep(ctx, send, cfg.cwd, "checkout", "--no-overwrite-ignore", "-b", cfg.branch); err != nil {
			return nil, fmt.Errorf("create branch %s: %w", cfg.branch, err)
		}
	}

	send(gitLineMsg("Recording session start"))
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("start interrupted; branch retained, inspect 'sess status' before retrying: %w", err)
	}
	if err := checkStartRebase(ctx, cfg.cwd); err != nil {
		return nil, err
	}
	current, err = git.CurrentBranch(ctx, cfg.cwd)
	if err != nil {
		return nil, fmt.Errorf("verify branch before recording session: %w", err)
	}
	actualHead, err := git.LocalBranchHead(ctx, cfg.cwd, cfg.branch)
	if err != nil {
		return nil, fmt.Errorf("verify session branch head: %w", err)
	}
	if current != cfg.branch || actualHead != head {
		return nil, fmt.Errorf("branch changed during start; no session saved, inspect Git state before retrying")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	started, err := manager.StartSession(
		cfg.project.ID,
		cfg.branch,
		cfg.issueID,
		cfg.issueTitle,
		cfg.branchType,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"save session on %s: %w\nBranch retained. Inspect 'sess status' and fix the database problem, then retry 'sess start' with the same issue/name and type, or use 'sess start --adopt %s' without issue metadata",
			cfg.branch,
			err,
			cfg.branch,
		)
	}
	return started, nil
}

func runStartWorkflow(database *db.DB, cfg startWorkflowConfig) (*db.Session, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	program := tea.NewProgram(newStartModel(cfg.branch))
	finished := make(chan struct{})
	var started *db.Session
	var workflowErr error
	go func() {
		defer close(finished)
		started, workflowErr = executeStartWorkflow(ctx, database, cfg, program.Send)
		if workflowErr != nil {
			program.Send(gitErrMsg{workflowErr})
			return
		}
		program.Send(gitSuccessMsg{})
	}()
	final, err := program.Run()
	cancel()
	<-finished
	if err != nil {
		return nil, fmt.Errorf("start UI failed; inspect 'sess status' before retrying: %w", err)
	}
	model, ok := final.(startModel)
	if !ok {
		return nil, fmt.Errorf("unexpected start UI result")
	}
	if !model.done {
		return nil, fmt.Errorf("start interrupted; inspect 'sess status' and Git state before retrying")
	}
	if workflowErr != nil {
		return nil, workflowErr
	}
	if started == nil {
		return nil, fmt.Errorf("start finished without a persisted session")
	}
	return started, nil
}

// RunAdoptStart tracks an explicitly selected local branch without recreating it.
func RunAdoptStart(cwd string, database *db.DB, branch string) error {
	ctx := context.Background()
	head, err := git.LocalBranchHead(ctx, cwd, branch)
	if err != nil {
		return fmt.Errorf("check branch for adoption: %w", err)
	}
	if head == "" {
		return fmt.Errorf("local branch %s does not exist; adoption does not create or fetch branches", branch)
	}
	if err := checkStartRebase(ctx, cwd); err != nil {
		return err
	}
	manager := session.NewManager(database)
	project, _, err := resolveStartProject(ctx, manager, database, cwd)
	if err != nil {
		return err
	}
	branchType, _, _ := strings.Cut(branch, "/")
	switch branchType {
	case "feature", "bugfix", "refactor":
	default:
		branchType = ""
	}
	started, err := executeStartWorkflow(ctx, database, startWorkflowConfig{
		cwd: cwd, project: project, branch: branch, branchType: branchType, adopt: true,
	}, func(tea.Msg) {})
	if err != nil {
		return err
	}
	printStartedSession(started)
	return nil
}

func printStartedSession(started *db.Session) {
	fmt.Printf("Started session on %s", started.Branch)
	if started.BranchType != "" {
		fmt.Printf(" (%s)", started.BranchType)
	}
	fmt.Println()
	if started.IssueID != "" {
		fmt.Printf("Issue #%s · %s\n", started.IssueID, started.IssueTitle)
	}
}
