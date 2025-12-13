package tui

import (
	"context"
	"fmt"
	"regexp"
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

// SessionState represents the possible states of a session
type SessionState int

const (
	Idle SessionState = iota
	Active
	Paused
	Ended
)

var (
	choiceSelect    = listItem{title: "Select issue", desc: "Select issue"}
	choiceAutomatic = listItem{title: "Start without issue", desc: "Start the session without and issue"}
	choiceStash     = listItem{title: "Stash changes", desc: "Stash uncommitted changes"}
	choiceCommit    = listItem{title: "Commit changes", desc: "Commit all changes"}
	choiceDiscard   = listItem{title: "Discard changes", desc: "Discard all uncommitted changes"}
	choiceQuit      = listItem{title: "Quit", desc: "Exit without doing anything"}

	// Branch types
	choiceFeature  = listItem{title: "feature", desc: "New feature development"}
	choiceBugfix   = listItem{title: "bugfix", desc: "Bug fix"}
	choiceRefactor = listItem{title: "refactor", desc: "Code refactoring"}
)

type promptModel struct {
	list   list.Model
	done   bool
	choice listItem
}

func newPromptModel() promptModel {
	items := []list.Item{
		choiceAutomatic,
		choiceSelect,
	}

	l := newStyledList(items, "Select an issue or start without an issue")

	return promptModel{list: l}
}

func (m promptModel) Init() tea.Cmd { return nil }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			i, ok := m.list.SelectedItem().(listItem)
			if ok {
				m.choice = i
				m.done = true
				return m, tea.Quit
			}
		case "q", "ctrl+c":
			m.choice = choiceQuit
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m promptModel) View() string {
	if m.done {
		return fmt.Sprintf("Selected: %s\n", m.choice)
	}
	m.list.ShowHelp()
	m.list.ShowFilter()
	return m.list.View()
}

type listItem struct {
	title, desc string
}

func (i listItem) FilterValue() string { return i.title }
func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }

// branchInputModel prompts the user for a branch name
type branchInputModel struct {
	textInput textinput.Model
	branch    string
	done      bool
}

func newBranchInputModel() branchInputModel {
	ti := textinput.New()
	ti.Placeholder = "my-feature"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	return branchInputModel{
		textInput: ti,
	}
}

func (m branchInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m branchInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.branch = m.textInput.Value()
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m branchInputModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf(
		"Enter branch name:\n\n%s\n\n%s",
		m.textInput.View(),
		"(esc to cancel)",
	)
}

// sanitizeBranchName converts a string to a valid git branch name
func sanitizeBranchName(name string) string {
	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	sanitized := reg.ReplaceAllString(name, "-")

	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)

	return sanitized
}

// branchTypeModel prompts the user to select a branch type
type branchTypeModel struct {
	list   list.Model
	done   bool
	choice listItem
}

func newBranchTypeModel() branchTypeModel {
	items := []list.Item{
		choiceFeature,
		choiceBugfix,
		choiceRefactor,
	}

	l := newStyledList(items, "Select branch type")

	return branchTypeModel{list: l}
}

func (m branchTypeModel) Init() tea.Cmd { return nil }

func (m branchTypeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			i, ok := m.list.SelectedItem().(listItem)
			if ok {
				m.choice = i
				m.done = true
				return m, tea.Quit
			}
		case "q", "ctrl+c":
			m.choice = choiceQuit
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m branchTypeModel) View() string {
	if m.done {
		return ""
	}
	return m.list.View()
}

func (s SessionState) String() string {
	return [...]string{"Idle", "Active", "Paused", "Ended"}[s]
}

type Model struct {
	Branch       string
	State        SessionState
	StartTime    time.Time
	PauseTime    time.Time
	TotalElapsed time.Duration
}

// Start begins a new session
func (m *Model) Start(branch string) {
	m.Branch = branch
	m.State = Active
	m.StartTime = time.Now()
	m.TotalElapsed = 0
	fmt.Println("Session started on branch:", branch)
}

// Pause pauses the session
func (m *Model) Pause() {
	if m.State == Active {
		m.State = Paused
		m.PauseTime = time.Now()
		m.TotalElapsed += m.PauseTime.Sub(m.StartTime)
		fmt.Println("Session paused.")
	}
}

// Resume resumes a paused session
func (m *Model) Resume() {
	if m.State == Paused {
		m.State = Active
		m.StartTime = time.Now()
		fmt.Println("Session resumed.")
	}
}

func (m *Model) End() {
	if m.State == Active {
		m.TotalElapsed += time.Since(m.StartTime)
	}
	m.State = Ended
	fmt.Printf("Session ended. Total time: %v on branch %s\n", m.TotalElapsed, m.Branch)
}

type startModel struct {
	spinner spinner.Model
	logs    []string
	err     error
	done    bool
	branch  string
	session Model
}

func newStartModel(branch string) startModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return startModel{
		spinner: s,
		branch:  branch,
		session: Model{Branch: branch},
	}
}

func (m startModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case gitErrMsg:
		m.err = msg
		m.done = true
		return m, tea.Quit
	case gitSuccessMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m startModel) View() string {
	s := fmt.Sprintf("Sess: Starting a new session '%s'\n\n", m.branch)
	if m.err != nil {
		s += "Error: " + m.err.Error() + "\n"
	} else if m.done {
		s += ""
	} else {
		s += m.spinner.View() + " Running git commands...\n\n"
	}
	start := 0
	if len(m.logs) > 10 {
		start = len(m.logs) - 10
	}
	for _, line := range m.logs[start:] {
		s += line + "\n"
	}
	if m.done {
		s += "\n(press q to quit)"
	}
	return s
}

// runStart orchestrates checkout dev → pull → create branch with live logs
func runStart(p *tea.Program, branchType, branchName string) {
	streamStep(p, ".", []string{"checkout", "dev"}, func() {
		streamStep(p, ".", []string{"pull", "origin", "dev"}, func() {
			safe := sanitizeBranchName(branchName)
			fullBranch := branchType + "/" + safe
			streamStep(p, ".", []string{"checkout", "-b", fullBranch}, func() {
				p.Send(gitSuccessMsg{})
			})
		})
	})
}

// ---------------- Public Entry ----------------

func RunStartTUI(featureName string, cwd string, database *db.DB) error {
	var branchName string
	var selectedIssue *git.Issue

	// Create session manager
	mgr := session.NewManager(database)

	// Initialize or get project (default base branch is "dev")
	project, err := mgr.InitializeProject(cwd, "dev")
	if err != nil {
		return fmt.Errorf("failed to initialize project: %w", err)
	}

	// Check if there's already an active session
	existingSession, err := mgr.GetActiveSession(project.ID)
	if err != nil {
		return fmt.Errorf("failed to check for active session: %w", err)
	}

	if existingSession != nil {
		return fmt.Errorf("there is already an %s session on branch '%s'\nUse 'sess status' to view it, 'sess pause' to pause it, or finish the current session first",
			existingSession.State, existingSession.Branch)
	}

	// 1. Ask if user wants to select an issue or start without one
	if featureName == "" {
		pm := tea.NewProgram(newPromptModel())
		final, err := pm.Run()
		if err != nil {
			return err
		}
		if p, ok := final.(promptModel); ok {
			switch p.choice {
			case choiceSelect:
				// Run issue selection TUI
				selectedIssue, err = RunIssueSelectTUI()
				if err != nil {
					return err
				}
				if selectedIssue != nil {
					// Use issue title as branch name
					branchName = selectedIssue.Title
				}
			case choiceAutomatic:
				// Continue without issue - will prompt for branch name
			case choiceQuit:
				return nil
			default:
				return nil
			}
		}
	} else {
		branchName = featureName
	}

	// 2. If no branch name yet, prompt for it
	if branchName == "" {
		tiProgram := tea.NewProgram(newBranchInputModel())
		final, err := tiProgram.Run()
		if err != nil {
			return err
		}
		if m, ok := final.(branchInputModel); ok {
			branchName = m.branch
		}
		if branchName == "" {
			return fmt.Errorf("no branch name provided")
		}
	}

	// 3. Select branch type (feature/bugfix/refactor)
	btProgram := tea.NewProgram(newBranchTypeModel())
	final, err := btProgram.Run()
	if err != nil {
		return err
	}
	var branchType string
	if bt, ok := final.(branchTypeModel); ok {
		if bt.choice == choiceQuit {
			return nil
		}
		branchType = bt.choice.title
	}
	if branchType == "" {
		return fmt.Errorf("no branch type selected")
	}

	// 4. Check if repo is dirty
	ctx := context.Background()
	dirty, err := git.IsDirty(ctx, ".")
	if err != nil {
		return err
	}
	if dirty {
		// Show options with stash/commit/discard choices
		items := []list.Item{
			choiceStash,
			choiceCommit,
			choiceDiscard,
			choiceQuit,
		}

		l := newStyledList(items, "Repository has uncommitted changes. What would you like to do?")
		dirtyPrompt := promptModel{list: l}
		dirtyProgram := tea.NewProgram(dirtyPrompt)
		final, err := dirtyProgram.Run()
		if err != nil {
			return err
		}
		if p, ok := final.(promptModel); ok {
			switch p.choice {
			case choiceStash:
				_, err = git.RunCombined(ctx, ".", "stash", "push", "-u")
			case choiceCommit:
				_, err = git.RunCombined(ctx, ".", "add", "-A")
				if err == nil {
					// Open Git editor for commit message
					_, err = git.RunCombined(ctx, ".", "commit")
				}
			case choiceDiscard:
				_, err = git.RunCombined(ctx, ".", "reset", "--hard")
			case choiceQuit:
				return nil
			}
			if err != nil {
				return err
			}
		}
	}

	// 5. Run main start model with live logs
	fullBranch := branchType + "/" + sanitizeBranchName(branchName)
	if selectedIssue != nil {
		fmt.Printf("Starting session for issue %s: %s\n", selectedIssue.ID, selectedIssue.Title)
		fmt.Printf("Branch: %s\n\n", fullBranch)
	}

	p := tea.NewProgram(newStartModel(fullBranch))
	go runStart(p, branchType, branchName) // start git orchestration in background
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if git operations succeeded
	if sm, ok := finalModel.(startModel); ok {
		if sm.err != nil {
			return sm.err
		}
	}

	// 6. Persist session to database
	issueID := ""
	issueTitle := ""
	if selectedIssue != nil {
		issueID = selectedIssue.ID
		issueTitle = selectedIssue.Title
	}

	_, err = mgr.StartSession(project.ID, fullBranch, issueID, issueTitle, branchType)
	if err != nil {
		// Session creation failed, but git operations succeeded
		// Print warning but don't fail
		fmt.Printf("\n  Warning: Failed to save session to database: %v\n", err)
		fmt.Println("Your branch was created successfully, but session tracking is unavailable.")
	} else {
		fmt.Println("\n✅ Session started and saved!")
		fmt.Println(" Use 'sess status' to view session details")
		fmt.Println(" Use 'sess pause' to pause when you need a break")
	}

	return nil
}
