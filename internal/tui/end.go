package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

// RunPauseTUI is the entry point for the pause command
func RunEndTUI() error {
	p := tea.NewProgram(initialEndModel())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running end TUI: %w", err)
	}

	// Handle the final model state
	if m, ok := finalModel.(endModel); ok {
		if m.err != nil {
			return m.err
		}
	}

	return nil
}

type endModel struct {
	// Your model fields here
	done bool
	err  error
}

func initialEndModel() endModel {
	return endModel{
		done: false,
	}
}

func (m endModel) Init() tea.Cmd {
	// Return initial commands
	return nil
}

func (m endModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m endModel) View() string {
	if m.done {
		return "Session ended!\n"
	}
	return "Press Enter to end session, or q to cancel.\n"
}
