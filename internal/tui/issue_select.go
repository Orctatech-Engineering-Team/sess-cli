package tui

import (
	"context"
	"fmt"

	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// issueItem wraps a git.Issue to implement list.Item interface
type issueItem struct {
	issue git.Issue
}

func (i issueItem) FilterValue() string { return i.issue.Title }
func (i issueItem) Title() string       { return fmt.Sprintf("#%d: %s", i.issue.Number, i.issue.Title) }
func (i issueItem) Description() string { return i.issue.URL }

// issuesLoadedMsg is sent when issues are successfully loaded
type issuesLoadedMsg struct {
	issues []git.Issue
}

// issuesErrorMsg is sent when there's an error loading issues
type issuesErrorMsg struct {
	err error
}

// issueSelectedMsg is sent when the user selects an issue
type issueSelectedMsg struct {
	issue *git.Issue
}

// issueSelectModel handles the issue selection UI
type issueSelectModel struct {
	list     list.Model
	spinner  spinner.Model
	loading  bool
	err      error
	selected *git.Issue
	done     bool
}

func newIssueSelectModel() issueSelectModel {
	// Create spinner for loading state
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#6f03fc"))

	// Create empty list (will be populated when issues load)
	l := newStyledList([]list.Item{}, "Select an issue")
	l.SetShowFilter(true)

	return issueSelectModel{
		list:    l,
		spinner: s,
		loading: true,
	}
}

func (m issueSelectModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadIssues(),
	)
}

// loadIssues fetches issues from GitHub in the background
func loadIssues() tea.Cmd {
	return func() tea.Msg {
		gh := git.NewGH()
		ctx := context.Background()

		issues, err := gh.ListIssuesJSON(ctx, ".", "open")
		if err != nil {
			return issuesErrorMsg{err: err}
		}

		return issuesLoadedMsg{issues: issues}
	}
}

func (m issueSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			// Only allow quit while loading
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.done = true
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "enter", "ctrl+j":
			if item, ok := m.list.SelectedItem().(issueItem); ok {
				m.selected = &item.issue
				m.done = true
				return m, tea.Quit
			}
		case "n":
			// 'n' for "no issue" - start without an issue
			m.done = true
			return m, tea.Quit
		case "q", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}

	case issuesLoadedMsg:
		m.loading = false

		// Convert issues to list items
		items := make([]list.Item, len(msg.issues))
		for i, issue := range msg.issues {
			items[i] = issueItem{issue: issue}
		}

		m.list.SetItems(items)
		return m, nil

	case issuesErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	// Update list when not loading
	if !m.loading {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m issueSelectModel) View() string {
	if m.done {
		if m.selected != nil {
			return fmt.Sprintf("Selected issue #%d: %s\n", m.selected.Number, m.selected.Title)
		}
		return "Starting without an issue\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error loading issues: %v\n\n(press q to quit)", m.err)
	}

	if m.loading {
		return fmt.Sprintf("%s Loading issues from GitHub...\n\n(press q to quit)", m.spinner.View())
	}

	// Show the list with help text
	help := "\nPress 'n' to start without an issue, 'q' to quit"
	return m.list.View() + help
}

// RunIssueSelectTUI shows the issue selection interface and returns the selected issue (or nil)
func RunIssueSelectTUI() (*git.Issue, error) {
	p := tea.NewProgram(newIssueSelectModel())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}

	if m, ok := final.(issueSelectModel); ok {
		return m.selected, nil
	}

	return nil, nil
}
