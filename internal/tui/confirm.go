
type confirmModel struct {
    prompt   string
    selected bool  // true = yes, false = no
    done     bool
}

func newConfirmModel(prompt string) confirmModel {
    return confirmModel{
        prompt:   prompt,
        selected: false,  // Default to "No"
        done:     false,
    }
}

func (m confirmModel) Init() tea.Cmd {
    return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            m.done = true
            return m, tea.Quit

        case "left", "h":
            m.selected = false  // Move to "No"

        case "right", "l":
            m.selected = true  // Move to "Yes"

        case "enter":
            m.done = true
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m confirmModel) View() string {
    if m.done {
        return ""  // Don't show anything after selection
    }

    // Render yes/no buttons
    yesButton := "[ Yes ]"
    noButton := "[ No ]"

    if m.selected {
        yesButton = SelectedStyle.Render(yesButton)
        noButton = UnselectedStyle.Render(noButton)
    } else {
        yesButton = UnselectedStyle.Render(yesButton)
        noButton = SelectedStyle.Render(noButton)
    }

    return fmt.Sprintf(
        "%s\n\n%s  %s\n\n(Use arrow keys to select, Enter to confirm)\n",
        m.prompt,
        noButton,
        yesButton,
    )
}