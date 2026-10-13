package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var endPhases = []string{
	db.EndPrepared,
	db.EndCommitted,
	db.EndRebased,
	db.EndPushed,
	db.EndPRCreated,
	db.EndBaseCheckedOut,
}

func prepareEndConfig(
	ctx context.Context,
	database *db.DB,
	cfg endWorkflowConfig,
) (endWorkflowConfig, bool, error) {
	checkpoint, err := database.GetEndWorkflow(cfg.session.ID)
	if err != nil {
		return cfg, false, fmt.Errorf("load end checkpoint: %w", err)
	}
	if checkpoint != nil {
		if err := validateEndCheckpoint(checkpoint, cfg.session); err != nil {
			return cfg, false, err
		}
		fmt.Printf("Resuming end workflow · %s\n", checkpoint.Phase)
		return configFromEndCheckpoint(cfg, checkpoint), false, nil
	}

	current, err := git.CurrentBranch(ctx, cfg.cwd)
	if err != nil {
		return cfg, false, err
	}
	if current != cfg.session.Branch {
		if err := git.Checkout(ctx, cfg.cwd, cfg.session.Branch); err != nil {
			return cfg, false, fmt.Errorf("switch to session branch before ending: %w", err)
		}
	}
	if cfg.project.BaseBranch == "" {
		cfg.project.BaseBranch = "dev"
	}
	dirty, err := git.IsDirty(ctx, cfg.cwd)
	if err != nil {
		return cfg, false, fmt.Errorf("check worktree state: %w", err)
	}
	ahead, err := git.HasCommitsAheadOf(ctx, cfg.cwd, "origin/"+cfg.project.BaseBranch)
	if err != nil {
		return cfg, false, fmt.Errorf("check commits ahead of base branch: %w", err)
	}
	if !hasShippableWork(dirty, ahead) {
		choice, err := runListPrompt("No uncommitted changes or commits ahead of the base branch. What would you like to do?", []list.Item{
			choiceEndAnyway,
			choiceCancelEnd,
		})
		if err != nil {
			return cfg, false, err
		}
		if choice != choiceEndAnyway {
			return cfg, true, nil
		}
		cfg.endWithoutPR = true
	}
	if !cfg.endWithoutPR {
		cfg.repository, cfg.remoteID, err = endRepositoryIdentity(ctx, cfg.cwd)
		if err != nil {
			return cfg, false, err
		}
		cfg.pr, err = git.NewGH().FindPRByHead(ctx, cfg.cwd, cfg.session.Branch, cfg.project.BaseBranch, cfg.repository)
		if err != nil {
			return cfg, false, fmt.Errorf("check pull request before prompting: %w", err)
		}
		if cfg.pr != nil && cfg.pr.State == "CLOSED" {
			return cfg, false, closedEndPRError(cfg.pr)
		}
	}
	if dirty {
		var cancelled bool
		cfg.commitMessage, cancelled, err = runTextPrompt(
			"Enter a commit message for the changes you want to ship", "Update session work", true,
		)
		if err != nil || cancelled {
			return cfg, cancelled, err
		}
	}
	if cfg.endWithoutPR || cfg.pr != nil {
		return cfg, false, nil
	}
	cfg.prTitle, err = buildPRTitle(ctx, cfg.cwd, cfg.session)
	if err != nil {
		return cfg, false, err
	}
	values := prPromptValues{}
	for _, field := range []struct {
		title       string
		placeholder string
		required    bool
		value       *string
	}{
		{title: "PR summary", placeholder: "Describe what changed", required: true, value: &values.Summary},
		{title: "PR testing", placeholder: "How did you verify it?", value: &values.Testing},
		{title: "PR notes", placeholder: "Anything reviewers should know?", value: &values.Notes},
	} {
		value, cancelled, err := runTextPrompt(field.title, field.placeholder, field.required)
		if err != nil || cancelled {
			return cfg, cancelled, err
		}
		*field.value = value
	}
	cfg.prBody = renderPRBody(cfg.session, values)
	return cfg, false, nil
}

func configFromEndCheckpoint(cfg endWorkflowConfig, checkpoint *db.EndWorkflow) endWorkflowConfig {
	project := *cfg.project
	project.BaseBranch = checkpoint.BaseBranch
	cfg.project = &project
	cfg.commitMessage = checkpoint.CommitMessage
	cfg.endWithoutPR = checkpoint.EndWithoutPR
	cfg.prTitle = checkpoint.PRTitle
	cfg.prBody = checkpoint.PRBody
	cfg.repository = checkpoint.Repository
	cfg.remoteID = checkpoint.RemoteID
	return cfg
}

func validateEndCheckpoint(checkpoint *db.EndWorkflow, sess *db.Session) error {
	if checkpoint.SessionID != sess.ID || checkpoint.Branch != sess.Branch {
		return fmt.Errorf("end checkpoint does not match the current session")
	}
	if checkpoint.BaseBranch == "" || checkpoint.Head == "" || slices.Index(endPhases, checkpoint.Phase) < 0 {
		return fmt.Errorf("end checkpoint is invalid; session remains open")
	}
	if checkpoint.EndWithoutPR {
		if checkpoint.PRNumber != nil || checkpoint.PRURL != "" {
			return fmt.Errorf("no-PR checkpoint contains PR metadata; session remains open")
		}
		return nil
	}
	if slices.Index(endPhases, checkpoint.Phase) >= slices.Index(endPhases, db.EndPRCreated) {
		if checkpoint.PRNumber == nil || *checkpoint.PRNumber <= 0 || checkpoint.PRURL == "" {
			return fmt.Errorf("end checkpoint is missing PR identity; session remains open")
		}
	}
	if checkpoint.Repository == "" || checkpoint.RemoteID == "" {
		return fmt.Errorf("end checkpoint is missing repository identity; session remains open")
	}
	return nil
}

type endAttempt struct {
	ctx        context.Context
	database   *db.DB
	mgr        *session.Manager
	gh         *git.GH
	cfg        endWorkflowConfig
	checkpoint *db.EndWorkflow
	pr         *git.PR
	send       func(tea.Msg)
}

func executeEndWorkflow(
	ctx context.Context,
	database *db.DB,
	cfg endWorkflowConfig,
	send func(tea.Msg),
) (endWorkflowResult, error) {
	if err := checkEndRebase(ctx, cfg.cwd); err != nil {
		return endWorkflowResult{}, err
	}
	mgr := session.NewManager(database)
	current, err := mgr.GetActiveSession(cfg.project.ID)
	if err != nil {
		return endWorkflowResult{}, fmt.Errorf("load session before end workflow: %w", err)
	}
	if current == nil || current.ID != cfg.session.ID || current.Branch != cfg.session.Branch {
		return endWorkflowResult{}, fmt.Errorf("session changed before end workflow; no work performed")
	}
	checkpoint, err := database.GetEndWorkflow(current.ID)
	if err != nil {
		return endWorkflowResult{}, fmt.Errorf("load end checkpoint: %w", err)
	}
	if checkpoint == nil {
		if !cfg.endWithoutPR {
			cfg.repository, cfg.remoteID, err = endRepositoryIdentity(ctx, cfg.cwd)
			if err != nil {
				return endWorkflowResult{}, err
			}
		}
		if !cfg.endWithoutPR && cfg.pr == nil && (cfg.prTitle == "" || cfg.prBody == "") {
			return endWorkflowResult{}, fmt.Errorf("no saved PR details; rerun 'sess end' to enter PR details")
		}
		head, err := endBranchHead(ctx, cfg.cwd, current.Branch)
		if err != nil {
			return endWorkflowResult{}, err
		}
		if head == "" {
			return endWorkflowResult{}, fmt.Errorf("session branch %s is missing", current.Branch)
		}
		checkpoint = &db.EndWorkflow{
			SessionID: current.ID, Branch: current.Branch, BaseBranch: cfg.project.BaseBranch,
			CommitMessage: cfg.commitMessage, EndWithoutPR: cfg.endWithoutPR,
			PRTitle: cfg.prTitle, PRBody: cfg.prBody, Phase: db.EndPrepared, Head: head,
			Repository: cfg.repository, RemoteID: cfg.remoteID,
		}
		if cfg.pr != nil {
			number := int64(cfg.pr.Number)
			checkpoint.PRNumber = &number
			checkpoint.PRURL = cfg.pr.URL
		}
		if err := ctx.Err(); err != nil {
			return endWorkflowResult{}, err
		}
		if err := database.CreateEndWorkflow(checkpoint); err != nil {
			return endWorkflowResult{}, fmt.Errorf("save end checkpoint before Git operations: %w", err)
		}
	}
	if err := validateEndCheckpoint(checkpoint, current); err != nil {
		return endWorkflowResult{}, err
	}
	cfg = configFromEndCheckpoint(cfg, checkpoint)
	attempt := endAttempt{
		ctx: ctx, database: database, mgr: mgr, gh: git.NewGH(),
		cfg: cfg, checkpoint: checkpoint, send: send,
	}
	return attempt.run()
}

func (a *endAttempt) run() (endWorkflowResult, error) {
	if !a.checkpoint.EndWithoutPR {
		remoteID, err := endRemoteIdentity(a.ctx, a.cfg.cwd)
		if err != nil {
			return endWorkflowResult{}, err
		}
		if remoteID != a.checkpoint.RemoteID {
			return endWorkflowResult{}, fmt.Errorf("origin changed since end checkpoint; session remains open")
		}
	}
	if err := a.validateLocalState(); err != nil {
		return endWorkflowResult{}, err
	}
	if a.checkpoint.PRNumber != nil {
		pr, err := a.gh.GetPR(a.ctx, a.cfg.cwd, int(*a.checkpoint.PRNumber), a.checkpoint.Repository)
		if err != nil {
			return endWorkflowResult{}, fmt.Errorf("recover saved PR: %w", err)
		}
		if err := a.acceptPR(pr, a.before(db.EndPushed)); err != nil {
			return endWorkflowResult{}, err
		}
		if pr.State == "MERGED" && a.before(db.EndPRCreated) {
			if err := a.savePhase(db.EndPRCreated); err != nil {
				return endWorkflowResult{}, err
			}
		}
	}
	if !a.checkpoint.EndWithoutPR && a.before(db.EndPRCreated) {
		if a.before(db.EndPushed) {
			if err := a.prepareWork(); err != nil {
				return endWorkflowResult{}, err
			}
			if err := a.pushWork(); err != nil {
				return endWorkflowResult{}, err
			}
		}
		if err := a.recoverPR(); err != nil {
			return endWorkflowResult{}, err
		}
	}
	if err := a.cleanup(); err != nil {
		return endWorkflowResult{}, err
	}
	a.log("Recording session completion")
	if err := a.ctx.Err(); err != nil {
		return endWorkflowResult{}, fmt.Errorf("end workflow interrupted before session completion: %w", err)
	}
	ended, err := a.mgr.CompleteEndWorkflow(a.cfg.project.ID, a.checkpoint)
	if err != nil {
		return endWorkflowResult{}, fmt.Errorf("mark session ended; checkpoint retained for retry: %w", err)
	}
	return endWorkflowResult{
		session: ended, pr: a.pr, endWithoutPR: a.checkpoint.EndWithoutPR,
	}, nil
}

func (a *endAttempt) before(phase string) bool {
	return slices.Index(endPhases, a.checkpoint.Phase) < slices.Index(endPhases, phase)
}

func (a *endAttempt) log(format string, args ...any) {
	a.send(gitLineMsg(fmt.Sprintf(format, args...)))
}

func (a *endAttempt) savePhase(phase string) error {
	if err := a.ctx.Err(); err != nil {
		return fmt.Errorf("end workflow interrupted before saving %s: %w", phase, err)
	}
	a.checkpoint.Phase = phase
	if err := a.database.UpdateEndWorkflow(a.checkpoint); err != nil {
		return fmt.Errorf("save end checkpoint %s; retry 'sess end' after fixing storage: %w", phase, err)
	}
	return nil
}

func (a *endAttempt) validateLocalState() error {
	dirty, err := git.IsDirty(a.ctx, a.cfg.cwd)
	if err != nil {
		return err
	}
	if dirty && (a.checkpoint.Phase != db.EndPrepared || a.checkpoint.EndWithoutPR) {
		return fmt.Errorf("uncommitted changes since end checkpoint; stash or commit them manually before retrying 'sess end'")
	}
	head, err := endBranchHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch)
	if err != nil {
		return err
	}
	if a.checkpoint.EndWithoutPR && head != "" && head != a.checkpoint.Head {
		return fmt.Errorf("session branch changed since no-PR end checkpoint; session remains open")
	}
	if a.before(db.EndRebased) && head != a.checkpoint.Head {
		proven, err := a.provesCompletedGitOperation(head)
		if err != nil {
			return err
		}
		if !proven {
			return fmt.Errorf("session branch changed since end checkpoint without a matching commit or rebase record; session remains open")
		}
	}
	if !a.before(db.EndRebased) && head != a.checkpoint.Head {
		if head == "" && !a.before(db.EndPushed) {
			return nil
		}
		return fmt.Errorf("session branch changed since end checkpoint %s; session remains open", a.checkpoint.Phase)
	}
	return nil
}

func (a *endAttempt) provesCompletedGitOperation(head string) (bool, error) {
	if a.checkpoint.EndWithoutPR || head == "" {
		return false, nil
	}
	ref := "refs/heads/" + a.checkpoint.Branch
	switch a.checkpoint.Phase {
	case db.EndPrepared:
		if a.checkpoint.CommitMessage == "" {
			return false, nil
		}
		parent, err := git.RunCombined(a.ctx, a.cfg.cwd, "rev-parse", "--verify", ref+"^")
		if err != nil {
			return false, fmt.Errorf("verify interrupted commit: %w", err)
		}
		message, err := git.RunCombined(a.ctx, a.cfg.cwd, "log", "-1", "--format=%B", ref)
		if err != nil {
			return false, fmt.Errorf("verify interrupted commit message: %w", err)
		}
		return parent == a.checkpoint.Head &&
			strings.TrimSpace(message) == strings.TrimSpace(a.checkpoint.CommitMessage), nil
	case db.EndCommitted:
		if a.checkpoint.RebaseTarget == "" {
			return false, nil
		}
		log, err := git.RunCombined(a.ctx, a.cfg.cwd, "reflog", "show", "-2", "--format=%H%n%gs", ref)
		if err != nil {
			return false, fmt.Errorf("verify interrupted rebase using branch reflog: %w", err)
		}
		lines := strings.Split(log, "\n")
		if len(lines) < 3 {
			return false, nil
		}
		finish := "rebase (finish): " + ref + " onto " + a.checkpoint.RebaseTarget
		return lines[0] == head && lines[1] == finish && lines[2] == a.checkpoint.Head, nil
	default:
		return false, nil
	}
}

func (a *endAttempt) prepareWork() error {
	current, err := git.CurrentBranch(a.ctx, a.cfg.cwd)
	if err != nil {
		return err
	}
	dirty, err := git.IsDirty(a.ctx, a.cfg.cwd)
	if err != nil {
		return err
	}
	if current != a.checkpoint.Branch {
		if dirty {
			return fmt.Errorf("dirty working tree on another branch; switch to %s manually before retrying", a.checkpoint.Branch)
		}
		if err := git.Checkout(a.ctx, a.cfg.cwd, a.checkpoint.Branch); err != nil {
			return fmt.Errorf("switch to session branch: %w", err)
		}
	}
	if !a.before(db.EndCommitted) {
		return nil
	}
	if dirty {
		if a.checkpoint.CommitMessage == "" {
			return fmt.Errorf("checkpoint has no commit message; commit changes manually before retrying 'sess end'")
		}
		a.log("Adding changes")
		if err := git.AddAll(a.ctx, a.cfg.cwd); err != nil {
			return fmt.Errorf("commit staged changes failed: %w", err)
		}
		a.log("Creating commit")
		if err := git.Commit(a.ctx, a.cfg.cwd, a.checkpoint.CommitMessage); err != nil {
			return fmt.Errorf("commit changes failed: %w", err)
		}
	}
	a.checkpoint.Head, err = endBranchHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch)
	if err != nil {
		return err
	}
	return a.savePhase(db.EndCommitted)
}

func (a *endAttempt) pushWork() error {
	if a.before(db.EndRebased) {
		if a.checkpoint.RebaseTarget == "" {
			a.log("Fetching origin/%s", a.checkpoint.BaseBranch)
			if err := runGitStep(a.ctx, a.send, a.cfg.cwd, "fetch", "origin", a.checkpoint.BaseBranch); err != nil {
				return fmt.Errorf("fetch origin/%s failed: %w", a.checkpoint.BaseBranch, err)
			}
			target, err := git.RunCombined(a.ctx, a.cfg.cwd, "rev-parse", "--verify", "refs/remotes/origin/"+a.checkpoint.BaseBranch)
			if err != nil {
				return fmt.Errorf("record rebase target: %w", err)
			}
			a.checkpoint.RebaseTarget = target
			if err := a.savePhase(db.EndCommitted); err != nil {
				return err
			}
		}
		a.log("Rebasing onto origin/%s", a.checkpoint.BaseBranch)
		if err := runGitStep(a.ctx, a.send, a.cfg.cwd, "rebase", a.checkpoint.RebaseTarget); err != nil {
			return fmt.Errorf("rebase onto origin/%s failed: %w\n\n%s", a.checkpoint.BaseBranch, err, endRebaseRecovery)
		}
		head, err := endBranchHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch)
		if err != nil {
			return err
		}
		a.checkpoint.Head = head
		if err := a.savePhase(db.EndRebased); err != nil {
			return err
		}
	}
	if !a.before(db.EndPushed) {
		return nil
	}
	a.log("Pushing %s", a.checkpoint.Branch)
	if err := runGitStep(a.ctx, a.send, a.cfg.cwd, "push", "-u", "origin", a.checkpoint.Branch); err != nil {
		return fmt.Errorf("push branch %s failed: %w\n\nInspect your remote/auth state, then rerun 'sess end'", a.checkpoint.Branch, err)
	}
	return a.savePhase(db.EndPushed)
}

func (a *endAttempt) recoverPR() error {
	a.log("Checking pull request")
	var pr *git.PR
	var err error
	if a.checkpoint.PRNumber != nil {
		pr, err = a.gh.GetPR(a.ctx, a.cfg.cwd, int(*a.checkpoint.PRNumber), a.checkpoint.Repository)
	} else {
		pr, err = a.gh.FindPRByHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch, a.checkpoint.BaseBranch, a.checkpoint.Repository)
	}
	if err != nil {
		return fmt.Errorf("check for existing pull request failed: %w", err)
	}
	if pr == nil {
		if a.checkpoint.PRTitle == "" || a.checkpoint.PRBody == "" {
			return fmt.Errorf("no saved PR details; rerun 'sess end' to enter PR details")
		}
		head, err := endBranchHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch)
		if err != nil {
			return err
		}
		if head != a.checkpoint.Head {
			return fmt.Errorf("session branch is missing or changed; cannot create PR from saved checkpoint")
		}
		if err := a.verifyRemoteHead(); err != nil {
			return err
		}
		if err := git.Checkout(a.ctx, a.cfg.cwd, a.checkpoint.Branch); err != nil {
			return fmt.Errorf("switch to session branch before PR creation: %w", err)
		}
		a.log("Creating pull request")
		out, err := a.gh.CreatePRForHead(
			a.ctx, a.cfg.cwd, a.checkpoint.PRTitle, a.checkpoint.PRBody, a.checkpoint.BaseBranch,
			a.checkpoint.Branch, a.checkpoint.Repository,
		)
		if err != nil {
			return fmt.Errorf("create pull request failed: %w\n\nCheckpoint retained; rerun 'sess end' after resolving the issue", err)
		}
		a.log("%s", strings.TrimSpace(out))
		pr = prFromCreateOutput(out)
		if pr == nil || pr.Number <= 0 {
			pr, err = a.gh.FindPRByHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch, a.checkpoint.BaseBranch, a.checkpoint.Repository)
			if err != nil {
				return fmt.Errorf("PR creation succeeded but metadata is unavailable; checkpoint retained for retry: %w", err)
			}
			if pr == nil {
				return fmt.Errorf("PR creation succeeded but metadata is unavailable; checkpoint retained for retry")
			}
		}
	}
	if pr.HeadRefOid != "" {
		if err := a.acceptPR(pr, false); err != nil {
			return err
		}
	}
	number := int64(pr.Number)
	a.checkpoint.PRNumber = &number
	a.checkpoint.PRURL = pr.URL
	if err := a.savePhase(db.EndPRCreated); err != nil {
		return err
	}
	pr, err = a.gh.GetPR(a.ctx, a.cfg.cwd, pr.Number, a.checkpoint.Repository)
	if err != nil {
		return fmt.Errorf("verify saved PR before completion: %w", err)
	}
	return a.acceptPR(pr, false)
}

func (a *endAttempt) verifyRemoteHead() error {
	ref := "refs/heads/" + a.checkpoint.Branch
	out, err := git.RunCombined(a.ctx, a.cfg.cwd, "ls-remote", "--heads", "origin", ref)
	if err != nil {
		return fmt.Errorf("verify remote branch before PR creation: %w", err)
	}
	head, name, ok := strings.Cut(out, "\t")
	if ok && !strings.Contains(out, "\n") && name == ref && head == a.checkpoint.Head {
		return nil
	}
	return fmt.Errorf("remote branch differs from end checkpoint; inspect local and remote history before retrying")
}

func (a *endAttempt) acceptPR(pr *git.PR, beforePush bool) error {
	if pr.HeadRefName != a.checkpoint.Branch || pr.BaseRefName != a.checkpoint.BaseBranch {
		return fmt.Errorf("saved PR #%d targets different branches; session remains open", pr.Number)
	}
	if pr.State == "CLOSED" {
		return closedEndPRError(pr)
	}
	if !beforePush || pr.State == "MERGED" {
		if pr.HeadRefOid != a.checkpoint.Head {
			return fmt.Errorf("PR #%d head differs from end checkpoint; session remains open", pr.Number)
		}
	}
	if pr.State == "MERGED" {
		dirty, err := git.IsDirty(a.ctx, a.cfg.cwd)
		if err != nil {
			return err
		}
		head, err := endBranchHead(a.ctx, a.cfg.cwd, a.checkpoint.Branch)
		if err != nil {
			return err
		}
		if dirty || (head != "" && head != a.checkpoint.Head) {
			return fmt.Errorf("local work changed after PR #%d merged; session remains open", pr.Number)
		}
	}
	a.pr = pr
	a.log("Using PR #%d · %s", pr.Number, strings.ToLower(pr.State))
	return nil
}

func closedEndPRError(pr *git.PR) error {
	return fmt.Errorf(
		"PR #%d closed without merging. Session and checkpoint remain open; inspect %s and reopen the PR before retrying",
		pr.Number, pr.URL,
	)
}

func (a *endAttempt) cleanup() error {
	current, err := git.CurrentBranch(a.ctx, a.cfg.cwd)
	if err != nil {
		return err
	}
	if !a.before(db.EndBaseCheckedOut) {
		if current != a.checkpoint.BaseBranch {
			return fmt.Errorf("checkout %s before completing the saved end workflow", a.checkpoint.BaseBranch)
		}
		return nil
	}
	a.log("Checking out %s", a.checkpoint.BaseBranch)
	if err := runGitStep(a.ctx, a.send, a.cfg.cwd, "checkout", a.checkpoint.BaseBranch); err != nil {
		return fmt.Errorf("switch back to %s failed: %w\n\nCheckpoint retained; fix the checkout problem and rerun 'sess end'", a.checkpoint.BaseBranch, err)
	}
	return a.savePhase(db.EndBaseCheckedOut)
}

func endBranchHead(ctx context.Context, cwd, branch string) (string, error) {
	ref := "refs/heads/" + branch
	out, err := git.RunCombined(ctx, cwd, "for-each-ref", "--format=%(refname) %(objectname)", ref)
	if err != nil {
		return "", fmt.Errorf("read session branch head: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		name, head, ok := strings.Cut(line, " ")
		if ok && name == ref {
			return head, nil
		}
	}
	return "", nil
}

func endRemoteIdentity(ctx context.Context, cwd string) (string, error) {
	fetch, err := git.RunCombined(ctx, cwd, "remote", "get-url", "--all", "origin")
	if err != nil {
		return "", fmt.Errorf("resolve origin: %w", err)
	}
	push, err := git.RunCombined(ctx, cwd, "remote", "get-url", "--push", "--all", "origin")
	if err != nil {
		return "", fmt.Errorf("resolve origin push destination: %w", err)
	}
	if fetch == "" || strings.Contains(fetch, "\n") || fetch != push {
		return "", fmt.Errorf("end recovery requires one matching origin fetch and push URL")
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fetch))), nil
}

func endRepositoryIdentity(ctx context.Context, cwd string) (string, string, error) {
	remoteID, err := endRemoteIdentity(ctx, cwd)
	if err != nil {
		return "", "", err
	}
	remote, err := git.RunCombined(ctx, cwd, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("resolve origin repository: %w", err)
	}
	repository, err := git.NewGH().ResolveRepository(ctx, cwd, remote)
	if err != nil {
		return "", "", fmt.Errorf("resolve GitHub origin repository: %w", err)
	}
	return repository, remoteID, nil
}
