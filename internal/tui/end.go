package tui

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	choiceEndAnyway   = listItem{title: "End session anyway", desc: "End this session without creating a PR"}
	choiceCancelEnd   = listItem{title: "Cancel", desc: "Leave the session open"}
	choiceKeepBranch  = listItem{title: "Keep local branch", desc: "Leave the session branch checked in locally"}
	choiceDeleteLocal = listItem{title: "Delete local branch", desc: "Delete the local session branch after ending"}
)

type textPromptModel struct {
	title       string
	placeholder string
	required    bool
	input       textinput.Model
	value       string
	done        bool
	cancelled   bool
	err         string
}

func newTextPromptModel(title, placeholder string, required bool) textPromptModel {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Focus()
	input.CharLimit = 200
	input.Width = 64

	return textPromptModel{
		title:       title,
		placeholder: placeholder,
		required:    required,
		input:       input,
	}
}

func (m textPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "ctrl+j":
			value := strings.TrimSpace(m.input.Value())
			if m.required && value == "" {
				m.err = "Value required"
				return m, nil
			}
			m.value = value
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}

	m.input, cmd = m.input.Update(msg)
	if strings.TrimSpace(m.input.Value()) != "" {
		m.err = ""
	}
	return m, cmd
}

func (m textPromptModel) View() string {
	if m.done {
		return ""
	}

	view := fmt.Sprintf("%s\n\n%s\n", m.title, m.input.View())
	if m.err != "" {
		view += "\n" + m.err + "\n"
	}
	if m.required {
		view += "\n(enter to submit, esc to cancel)"
	} else {
		view += "\n(optional · enter to submit, esc to cancel)"
	}
	return view
}

type prPromptValues struct {
	Summary string
	Testing string
	Notes   string
}

type endWorkflowConfig struct {
	cwd           string
	project       *db.Project
	session       *db.Session
	commitMessage string
	endWithoutPR  bool
	prTitle       string
	prBody        string
}

type endWorkflowResult struct {
	session      *db.Session
	pr           *git.PR
	endWithoutPR bool
}

type endSuccessMsg struct {
	result endWorkflowResult
}

type endWorkflowModel struct {
	spinner spinner.Model
	logs    []string
	err     error
	done    bool
	result  endWorkflowResult
	branch  string
}

func newEndWorkflowModel(branch string) endWorkflowModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return endWorkflowModel{
		spinner: s,
		branch:  branch,
	}
}

func (m endWorkflowModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m endWorkflowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		if !m.done {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case gitLineMsg:
		m.logs = append(m.logs, string(msg))
	case gitErrLineMsg:
		m.logs = append(m.logs, "[stderr] "+string(msg))
	case gitErrMsg:
		m.err = msg
		m.done = true
		return m, tea.Quit
	case endSuccessMsg:
		m.done = true
		m.result = msg.result
		return m, tea.Quit
	}
	return m, nil
}

func (m endWorkflowModel) View() string {
	view := fmt.Sprintf("Sess: Ending session '%s'\n\n", m.branch)
	switch {
	case m.err != nil:
		view += "Error: " + m.err.Error() + "\n"
	case !m.done:
		view += m.spinner.View() + " Running end workflow...\n\n"
	}

	start := 0
	if len(m.logs) > 12 {
		start = len(m.logs) - 12
	}
	for _, line := range m.logs[start:] {
		view += line + "\n"
	}

	if m.done {
		view += "\n(press q to quit)"
	}
	return view
}

func RunEndTUI(cwd string, database *db.DB) error {
	ctx := context.Background()
	mgr := session.NewManager(database)

	project, err := mgr.GetProject(cwd)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}
	if project == nil {
		return fmt.Errorf("not a tracked project. Run 'sess start' first")
	}

	activeSession, err := mgr.GetActiveSession(project.ID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if activeSession == nil {
		return fmt.Errorf("no active or paused session found")
	}

	currentBranch, err := git.CurrentBranch(ctx, cwd)
	if err != nil {
		return fmt.Errorf("determine current branch before ending session: %w", err)
	}
	if currentBranch != activeSession.Branch {
		fmt.Printf("Switching to branch %s...", activeSession.Branch)
		if err := git.Checkout(ctx, cwd, activeSession.Branch); err != nil {
			fmt.Println(" failed")
			return fmt.Errorf("switch to session branch %s before ending: %w", activeSession.Branch, err)
		}
		fmt.Println(" done")
	}

	dirty, err := git.IsDirty(ctx, cwd)
	if err != nil {
		return fmt.Errorf("check worktree state: %w", err)
	}

	baseBranch := project.BaseBranch
	if baseBranch == "" {
		baseBranch = "dev"
	}
	project.BaseBranch = baseBranch
	ahead, err := git.HasCommitsAheadOf(ctx, cwd, "origin/"+baseBranch)
	if err != nil {
		return fmt.Errorf("check commits ahead of origin/%s: %w", baseBranch, err)
	}

	shipWork := hasShippableWork(dirty, ahead)
	endWithoutPR := false
	commitMessage := ""
	if !shipWork {
		choice, err := runListPrompt("No uncommitted changes or commits ahead of the base branch. What would you like to do?", []list.Item{
			choiceEndAnyway,
			choiceCancelEnd,
		})
		if err != nil {
			return err
		}
		switch choice {
		case choiceCancelEnd:
			return nil
		case choiceEndAnyway:
			endWithoutPR = true
		default:
			return nil
		}
	}

	if dirty {
		var cancelled bool
		commitMessage, cancelled, err = runTextPrompt("Enter a commit message for the changes you want to ship", "Update session work", true)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
	}

	cfg := endWorkflowConfig{
		cwd:           cwd,
		project:       project,
		session:       activeSession,
		commitMessage: commitMessage,
		endWithoutPR:  endWithoutPR,
	}

	if !endWithoutPR {
		cfg.prTitle, err = buildPRTitle(ctx, cwd, activeSession)
		if err != nil {
			return err
		}

		summary, cancelled, err := runTextPrompt("PR summary", "Describe what changed", true)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		testing, cancelled, err := runTextPrompt("PR testing", "How did you verify it?", false)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		notes, cancelled, err := runTextPrompt("PR notes", "Anything reviewers should know?", false)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		cfg.prBody = renderPRBody(activeSession, prPromptValues{
			Summary: summary,
			Testing: testing,
			Notes:   notes,
		})
	}

	program := tea.NewProgram(newEndWorkflowModel(activeSession.Branch))
	go runEndWorkflow(program, database, cfg)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	workflow, ok := finalModel.(endWorkflowModel)
	if !ok {
		return fmt.Errorf("unexpected end workflow result")
	}
	if workflow.err != nil {
		return workflow.err
	}

	cleanupChoice, err := runListPrompt("What would you like to do with the local session branch?", []list.Item{
		choiceKeepBranch,
		choiceDeleteLocal,
	})
	if err != nil {
		return err
	}

	if cleanupChoice == choiceDeleteLocal {
		if err := git.DeleteBranch(ctx, cwd, workflow.result.session.Branch); err != nil {
			fmt.Printf("Warning: failed to delete local branch %s: %v\n", workflow.result.session.Branch, err)
		}
	}

	printEndSummary(workflow.result, mgr.GetCurrentElapsed(workflow.result.session))
	return nil
}

func runListPrompt(title string, items []list.Item) (listItem, error) {
	program := tea.NewProgram(promptModel{list: newStyledList(items, title)})
	final, err := program.Run()
	if err != nil {
		return listItem{}, err
	}

	result, ok := final.(promptModel)
	if !ok {
		return listItem{}, fmt.Errorf("unexpected prompt result")
	}
	return result.choice, nil
}

func runTextPrompt(title, placeholder string, required bool) (string, bool, error) {
	program := tea.NewProgram(newTextPromptModel(title, placeholder, required))
	final, err := program.Run()
	if err != nil {
		return "", false, err
	}

	result, ok := final.(textPromptModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected prompt result")
	}
	if result.cancelled {
		return "", true, nil
	}
	if required && strings.TrimSpace(result.value) == "" {
		return "", false, fmt.Errorf("no value provided")
	}
	return result.value, false, nil
}

func runEndWorkflow(p *tea.Program, database *db.DB, cfg endWorkflowConfig) {
	ctx := context.Background()
	mgr := session.NewManager(database)
	gh := git.NewGH()

	fail := func(err error) {
		p.Send(gitErrMsg{err})
	}

	sendLog := func(format string, args ...any) {
		p.Send(gitLineMsg(fmt.Sprintf(format, args...)))
	}

	if cfg.commitMessage != "" {
		sendLog("Adding changes")
		if err := git.AddAll(ctx, cfg.cwd); err != nil {
			fail(fmt.Errorf("commit staged changes failed: %w", err))
			return
		}
		sendLog("Creating commit")
		if err := git.Commit(ctx, cfg.cwd, cfg.commitMessage); err != nil {
			fail(fmt.Errorf("commit changes failed: %w", err))
			return
		}
	}

	if cfg.endWithoutPR {
		sendLog("Checking out %s", cfg.project.BaseBranch)
		if err := runGitStep(ctx, p, cfg.cwd, "checkout", cfg.project.BaseBranch); err != nil {
			fail(fmt.Errorf("switch back to %s failed: %w", cfg.project.BaseBranch, err))
			return
		}

		ended, err := mgr.CompleteSession(cfg.project.ID, nil, "")
		if err != nil {
			fail(fmt.Errorf("mark session ended: %w", err))
			return
		}

		p.Send(endSuccessMsg{result: endWorkflowResult{
			session:      ended,
			endWithoutPR: true,
		}})
		return
	}

	sendLog("Fetching origin/%s", cfg.project.BaseBranch)
	if err := runGitStep(ctx, p, cfg.cwd, "fetch", "origin", cfg.project.BaseBranch); err != nil {
		fail(fmt.Errorf("fetch origin/%s failed: %w", cfg.project.BaseBranch, err))
		return
	}

	sendLog("Rebasing onto origin/%s", cfg.project.BaseBranch)
	if err := runGitStep(ctx, p, cfg.cwd, "rebase", "origin/"+cfg.project.BaseBranch); err != nil {
		fail(fmt.Errorf("rebase onto origin/%s failed: %w\n\nResolve conflicts manually or run 'git rebase --abort', then rerun 'sess end'", cfg.project.BaseBranch, err))
		return
	}

	sendLog("Pushing %s", cfg.session.Branch)
	if err := runGitStep(ctx, p, cfg.cwd, "push", "-u", "origin", cfg.session.Branch); err != nil {
		fail(fmt.Errorf("push branch %s failed: %w\n\nInspect your remote/auth state, then rerun 'sess end'", cfg.session.Branch, err))
		return
	}

	sendLog("Checking for existing pull request")
	pr, err := gh.FindOpenPRByHead(ctx, cfg.cwd, cfg.session.Branch)
	if err != nil {
		fail(fmt.Errorf("check for existing pull request failed: %w", err))
		return
	}

	if pr == nil {
		sendLog("Creating pull request")
		out, err := gh.CreatePR(ctx, cfg.cwd, cfg.prTitle, cfg.prBody, cfg.project.BaseBranch)
		if err != nil {
			fail(fmt.Errorf("create pull request failed: %w\n\nThe branch may already be pushed. Rerun 'sess end' after resolving the issue", err))
			return
		}
		sendLog(strings.TrimSpace(out))

		pr, err = gh.FindOpenPRByHead(ctx, cfg.cwd, cfg.session.Branch)
		if err != nil {
			fail(fmt.Errorf("look up created pull request failed: %w", err))
			return
		}
		if pr == nil {
			pr = prFromCreateOutput(strings.TrimSpace(out))
		}
		if pr == nil {
			fail(fmt.Errorf("created pull request, but could not determine its metadata"))
			return
		}
	} else {
		sendLog("Using existing PR #%d", pr.Number)
	}

	sendLog("Checking out %s", cfg.project.BaseBranch)
	if err := runGitStep(ctx, p, cfg.cwd, "checkout", cfg.project.BaseBranch); err != nil {
		fail(fmt.Errorf("switch back to %s failed after PR creation: %w\n\nThe pull request succeeded, but the session remains open because cleanup did not complete", cfg.project.BaseBranch, err))
		return
	}

	var prNumber *int64
	if pr != nil && pr.Number > 0 {
		number := int64(pr.Number)
		prNumber = &number
	}

	ended, err := mgr.CompleteSession(cfg.project.ID, prNumber, pr.URL)
	if err != nil {
		fail(fmt.Errorf("mark session ended: %w", err))
		return
	}

	p.Send(endSuccessMsg{result: endWorkflowResult{
		session: ended,
		pr:      pr,
	}})
}

func runGitStep(ctx context.Context, p *tea.Program, dir string, args ...string) error {
	return git.RunStream(ctx, dir, args,
		func(line string) {
			p.Send(gitLineMsg(line))
		},
		func(line string) {
			p.Send(gitErrLineMsg(line))
		},
	)
}

func hasShippableWork(dirty, ahead bool) bool {
	return dirty || ahead
}

func buildPRTitle(ctx context.Context, cwd string, sess *db.Session) (string, error) {
	if sess.IssueID != "" {
		title := fmt.Sprintf("#%s", sess.IssueID)
		if sess.IssueTitle != "" {
			title += " " + sess.IssueTitle
		}
		return title, nil
	}

	subject, err := git.LatestCommitSubject(ctx, cwd)
	if err != nil {
		return "", fmt.Errorf("determine PR title from latest commit: %w", err)
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return sess.Branch, nil
	}
	return subject, nil
}

func renderPRBody(sess *db.Session, values prPromptValues) string {
	var builder strings.Builder

	if sess.IssueID != "" {
		fmt.Fprintf(&builder, "Closes #%s\n\n", sess.IssueID)
	}

	fmt.Fprintf(&builder, "## Summary\n%s\n\n", requiredPRField(values.Summary))
	fmt.Fprintf(&builder, "## Testing\n%s\n\n", optionalPRField(values.Testing))
	fmt.Fprintf(&builder, "## Notes\n%s\n", optionalPRField(values.Notes))

	return builder.String()
}

func requiredPRField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Not provided"
	}
	return value
}

func optionalPRField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Not provided"
	}
	return value
}

func prFromCreateOutput(output string) *git.PR {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}

	line := strings.Split(output, "\n")[0]
	parsed, err := url.Parse(line)
	if err != nil {
		return &git.PR{URL: line}
	}

	number, err := strconv.Atoi(path.Base(parsed.Path))
	if err != nil {
		return &git.PR{URL: line}
	}

	return &git.PR{
		Number: number,
		URL:    line,
		State:  "open",
	}
}

func printEndSummary(result endWorkflowResult, elapsed time.Duration) {
	session := result.session
	branchDisplay := session.Branch
	if session.BranchType != "" {
		branchDisplay = fmt.Sprintf("%s (%s)", session.Branch, session.BranchType)
	}

	fmt.Printf("Ended session on %s\n", branchDisplay)
	if session.IssueID != "" {
		fmt.Printf("Issue #%s", session.IssueID)
		if session.IssueTitle != "" {
			fmt.Printf(" · %s", session.IssueTitle)
		}
		fmt.Println()
	}
	if result.endWithoutPR {
		fmt.Println("No PR created")
	} else if result.pr != nil {
		if result.pr.Number > 0 {
			fmt.Printf("PR #%d", result.pr.Number)
		} else {
			fmt.Print("PR")
		}
		if result.pr.URL != "" {
			fmt.Printf(" · %s", result.pr.URL)
		}
		fmt.Println()
	}
	fmt.Printf("Total elapsed: %s\n", formatElapsed(elapsed))
}

func formatElapsed(d time.Duration) string {
	seconds := int64(d.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
