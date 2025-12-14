# SESS CLI - Developer Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Development Workflow](#development-workflow)
3. [Project Structure](#project-structure)
4. [Adding New Features](#adding-new-features)
5. [Working with the TUI](#working-with-the-tui)
6. [Git Integration](#git-integration)
7. [Testing](#testing)
8. [Code Style](#code-style)
9. [Common Tasks](#common-tasks)
10. [Troubleshooting](#troubleshooting)

---

## Getting Started

### Prerequisites

- **Go 1.25.4 or later** - [Install Go](https://go.dev/dl/)
- **Git** - [Install Git](https://git-scm.com/downloads)
- **GitHub CLI (gh)** - [Install gh](https://cli.github.com/)

### Initial Setup

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd sess-cli
   ```

2. **Install dependencies**
   ```bash
   go mod tidy
   ```

3. **Build the project**
   ```bash
   go build -o sess ./cmd/sess
   ```

4. **Run the CLI**
   ```bash
   ./sess --help
   ```

### Development Tools

**Recommended:**
- **VS Code** with Go extension
- **gopls** - Go language server (usually auto-installed by VS Code)

**Optional:**
- **air** - Live reload for Go apps: `go install github.com/air-verse/air@latest`
- **delve** - Go debugger: `go install github.com/go-delve/delve/cmd/dlv@latest`

---

## Development Workflow

### Quick Development Loop

1. **Make changes** to source files
2. **Build** the binary: `go build -o sess ./cmd/sess`
3. **Test** your changes: `./sess start`
4. **Iterate** - repeat as needed

### Live Reload with Air

Create `.air.toml` in project root:

```toml
[build]
  cmd = "go build -o ./tmp/sess ./cmd/sess"
  bin = "./tmp/sess"
  include_ext = ["go"]
  exclude_dir = ["tmp"]
```

Run: `air`

### Running Specific Commands

```bash
# Test the start workflow
./sess start

# Test with pre-filled branch name
./sess start my-feature

# Test in a different directory
cd /path/to/test/repo
/path/to/sess-cli/sess start
```

---

## Project Structure

### Directory Layout

```
sess-cli/
├── cmd/sess/              # Application entry point
│   └── main.go           # Minimal bootstrap
│
├── internal/             # Private application code (not importable)
│   ├── sess/            # Command layer (Cobra)
│   ├── tui/             # UI layer (Bubble Tea)
│   └── git/             # Integration layer (Git/GitHub)
│
├── .gitignore           # Git ignore rules
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
├── ARCHITECTURE.md      # Architecture documentation
├── DEVELOPMENT.md       # This file
└── README.md            # User-facing documentation
```

### Understanding `internal/`

The `internal/` directory is a Go convention that prevents other projects from importing your packages.

**Why use it?**
- We're building a CLI tool, not a library
- No public API to maintain
- Freedom to refactor without breaking external users
- Clear signal: "This is application code, not a reusable component"

### Package Dependency Rules

```
cmd/sess
  └─> internal/sess
       └─> internal/tui
            └─> internal/git

# NEVER reverse these dependencies!
# Example: internal/git should NEVER import internal/tui
```

**Key Principle:** Lower layers don't know about upper layers

---

## Adding New Features

### Adding a New Command

**Example:** Add a `sess pause` command to pause the current session

#### Step 1: Create Command File

Create `internal/sess/pause.go`:

```go
package sess

import (
    "github.com/spf13/cobra"
    "github.com/Orctatech-Engineering-Team/Sess/internal/tui"
)

var pauseCmd = &cobra.Command{
    Use:   "pause",
    Short: "Pause the current session",
    Long:  "Pauses the current work session and records the pause time.",
    RunE: func(cmd *cobra.Command, args []string) error {
        return tui.RunPauseTUI()  // Delegate to TUI layer
    },
}

func init() {
    rootCmd.AddCommand(pauseCmd)
}
```

#### Step 2: Create TUI Component

Create the UI flow in `internal/tui/pause.go`:

```go
package tui

import (
    "fmt"
    tea "github.com/charmbracelet/bubbletea"
)

// RunPauseTUI is the entry point for the pause command
func RunPauseTUI() error {
    p := tea.NewProgram(initialPauseModel())

    finalModel, err := p.Run()
    if err != nil {
        return fmt.Errorf("error running pause TUI: %w", err)
    }

    // Handle the final model state
    if m, ok := finalModel.(pauseModel); ok {
        if m.err != nil {
            return m.err
        }
    }

    return nil
}

type pauseModel struct {
    // Your model fields here
    done bool
    err  error
}

func initialPauseModel() pauseModel {
    return pauseModel{
        done: false,
    }
}

func (m pauseModel) Init() tea.Cmd {
    // Return initial commands
    return nil
}

func (m pauseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m pauseModel) View() string {
    if m.done {
        return "Session paused!\n"
    }
    return "Press Enter to pause session, or q to cancel.\n"
}
```

#### Step 3: Add Business Logic

If you need git operations, add them to `internal/git/`:

```go
// internal/git/git.go

// GetCurrentSession reads session data from git branch
func GetCurrentSession(ctx context.Context, dir string) (*Session, error) {
    branch, err := CurrentBranch(ctx, dir)
    if err != nil {
        return nil, err
    }

    // Parse branch name to get session info
    // ...

    return &Session{Branch: branch}, nil
}
```

#### Step 4: Test

```bash
go build -o sess ./cmd/sess
./sess pause
```

### Adding a New TUI Component

**Example:** Add a confirmation dialog component

#### Step 1: Define the Model

```go
// internal/tui/confirm.go

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
```

#### Step 2: Implement Bubble Tea Interface

```go
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
```

#### Step 3: Use in a Workflow

```go
// In another TUI component's Update() method

case someMsg:
    // Show confirmation dialog
    return m, func() tea.Msg {
        p := tea.NewProgram(newConfirmModel("Are you sure?"))
        result, _ := p.Run()

        if cm, ok := result.(confirmModel); ok && cm.selected {
            return confirmedMsg{}
        }
        return cancelledMsg{}
    }
```

---

## Working with the TUI

### Bubble Tea Basics

**Three Core Methods:**

1. **Init()** - Returns initial command to run
2. **Update(msg)** - Handles messages, returns new model + command
3. **View()** - Renders current state as string

**Message Flow:**

```
User Input → Update() → Model Change → View() → Terminal
     ↑                                             │
     └──────────── Program Loop ───────────────────┘
```

### Common Patterns

#### 1. Async Operations with Messages

```go
// Define a message type
type dataLoadedMsg struct {
    data []string
}

// In Init() or Update(), return a command that loads data
func loadDataCmd() tea.Msg {
    // Do async work
    data := fetchDataFromAPI()

    // Return a message
    return dataLoadedMsg{data: data}
}

// In Update(), handle the message
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case dataLoadedMsg:
        m.data = msg.data
        m.loading = false
        return m, nil
    }
    return m, nil
}
```

#### 2. Composing Sub-Components

```go
type parentModel struct {
    list   list.Model    // Bubbles list component
    input  textinput.Model  // Bubbles text input
}

func (m parentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    // Update sub-components
    newList, cmd := m.list.Update(msg)
    m.list = newList.(list.Model)
    cmds = append(cmds, cmd)

    newInput, cmd := m.input.Update(msg)
    m.input = newInput
    cmds = append(cmds, cmd)

    // Batch commands
    return m, tea.Batch(cmds...)
}
```

#### 3. Sequential Workflows

```go
type step int

const (
    stepBranchName step = iota
    stepBranchType
    stepConfirm
)

type model struct {
    currentStep step
    branchName  string
    branchType  string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.currentStep {
    case stepBranchName:
        // Handle branch name input
        // On Enter, move to next step
        if keyMsg.String() == "enter" {
            m.currentStep = stepBranchType
        }

    case stepBranchType:
        // Handle branch type selection
        // On Enter, move to confirm
        if keyMsg.String() == "enter" {
            m.currentStep = stepConfirm
        }

    case stepConfirm:
        // Handle confirmation
        // On Enter, complete workflow
    }
    return m, nil
}
```

### Using Bubbles Components

#### List Component

```go
import "github.com/charmbracelet/bubbles/list"

// Create items
items := []list.Item{
    item{title: "Option 1", desc: "Description"},
    item{title: "Option 2", desc: "Description"},
}

// Create list
l := list.New(items, list.NewDefaultDelegate(), 80, 20)
l.Title = "Select an option"

// In Update()
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            selected := m.list.SelectedItem().(item)
            // Use selected item
        }
    }

    // Let list handle its own updates
    newList, cmd := m.list.Update(msg)
    m.list = newList
    return m, cmd
}
```

#### Text Input Component

```go
import "github.com/charmbracelet/bubbles/textinput"

// Create input
ti := textinput.New()
ti.Placeholder = "Enter branch name"
ti.Focus()
ti.CharLimit = 100

// In Update()
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "enter" {
            value := m.input.Value()
            // Use the input value
        }
    }

    // Let input handle its own updates
    newInput, cmd := m.input.Update(msg)
    m.input = newInput
    return m, cmd
}
```

#### Spinner Component

```go
import "github.com/charmbracelet/bubbles/spinner"

// Create spinner
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// In Init()
func (m model) Init() tea.Cmd {
    return m.spinner.Tick  // Start spinning
}

// In Update()
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    return m, cmd
}

// In View()
func (m model) View() string {
    return fmt.Sprintf("%s Loading...", m.spinner.View())
}
```

### Styling with Lipgloss

```go
import "github.com/charmbracelet/lipgloss"

// Define styles
var (
    TitleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("205")).
        MarginBottom(1)

    SelectedStyle = lipgloss.NewStyle().
        Background(lipgloss.Color("62")).
        Foreground(lipgloss.Color("230")).
        Padding(0, 1)

    ErrorStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("196")).
        Bold(true)
)

// Use in View()
func (m model) View() string {
    title := TitleStyle.Render("SESS CLI")

    if m.err != nil {
        return ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
    }

    return title + "\n" + m.content
}
```

---

## Git Integration

### Adding Git Operations

#### Step 1: Add Function to git Package

```go
// internal/git/git.go

// Rebase rebases current branch onto target
func Rebase(ctx context.Context, dir, target string) error {
    ctx, cancel := ensureTimeout(ctx, 30*time.Second)
    defer cancel()

    return Run(ctx, dir, "rebase", target)
}
```

#### Step 2: Use from TUI Layer

```go
// internal/tui/rebase.go

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    case confirmMsg:
        return m, func() tea.Msg {
            ctx := context.Background()
            if err := git.Rebase(ctx, ".", "main"); err != nil {
                return errorMsg{err}
            }
            return successMsg{}
        }
}
```

### Streaming Git Output

For long-running operations, use the streaming pattern:

```go
// In internal/tui/common.go - already exists!
func streamStep(
    program *tea.Program,
    dir string,
    args []string,
    next func() tea.Msg,
) tea.Cmd {
    // Runs git command
    // Streams stdout/stderr to UI
    // Calls next() on success
}

// Usage in your TUI component:
return m, streamStep(
    program,
    ".",
    []string{"rebase", "main"},
    func() tea.Msg {
        return rebaseCompleteMsg{}
    },
)
```

### Adding GitHub Operations

#### Step 1: Add Function to gh Package

```go
// internal/git/gh.go

// ListPullRequests returns PRs for the current repo
func (c *GH) ListPullRequests(ctx context.Context, state string) ([]PullRequest, error) {
    ctx, cancel := ensureTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--state", state, "--json", "number,title,url")
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("gh pr list failed: %w", err)
    }

    var prs []PullRequest
    if err := json.Unmarshal(output, &prs); err != nil {
        return nil, fmt.Errorf("failed to parse PR JSON: %w", err)
    }

    return prs, nil
}

type PullRequest struct {
    Number int    `json:"number"`
    Title  string `json:"title"`
    URL    string `json:"url"`
}
```

#### Step 2: Use in TUI

```go
// Load PRs asynchronously
func loadPRsCmd() tea.Msg {
    ctx := context.Background()
    prs, err := git.GH.ListPullRequests(ctx, "open")
    if err != nil {
        return prsErrorMsg{err}
    }
    return prsLoadedMsg{prs}
}
```

---

## Testing

### Unit Testing

#### Testing Pure Functions

```go
// internal/tui/utils_test.go

func TestSanitizeBranchName(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"My Feature", "my-feature"},
        {"Fix Bug #123", "fix-bug-123"},
        {"  spaces  ", "spaces"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            result := sanitizeBranchName(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

#### Testing Bubble Tea Components

```go
// internal/tui/confirm_test.go

func TestConfirmModel_Update(t *testing.T) {
    m := newConfirmModel("Are you sure?")

    // Test right arrow selects yes
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
    if !m.(confirmModel).selected {
        t.Error("expected selected to be true")
    }

    // Test left arrow selects no
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
    if m.(confirmModel).selected {
        t.Error("expected selected to be false")
    }
}
```

### Integration Testing

#### Testing Git Operations

```go
// internal/git/git_test.go

func TestGitOperations(t *testing.T) {
    // Create temp git repo
    tmpDir := t.TempDir()

    ctx := context.Background()

    // Initialize repo
    if err := Run(ctx, tmpDir, "init"); err != nil {
        t.Fatalf("git init failed: %v", err)
    }

    // Test checkout
    if err := Checkout(ctx, tmpDir, "-b", "test-branch"); err != nil {
        t.Errorf("checkout failed: %v", err)
    }

    // Verify branch
    branch, err := CurrentBranch(ctx, tmpDir)
    if err != nil {
        t.Errorf("current branch failed: %v", err)
    }
    if branch != "test-branch" {
        t.Errorf("got branch %q, want %q", branch, "test-branch")
    }
}
```

### Manual Testing Checklist

Before committing changes, test these scenarios:

- [ ] `sess start` with no arguments
- [ ] `sess start my-feature` with feature name
- [ ] Start session in clean repository
- [ ] Start session in dirty repository (test stash/commit/discard)
- [ ] Select issue from GitHub
- [ ] Start without selecting issue
- [ ] Test with no internet connection (should fail gracefully)
- [ ] Test in non-git directory (should show clear error)
- [ ] Test keyboard navigation (arrows, enter, esc, ctrl+c)
- [ ] Test with very long branch names

---

## Code Style

### Go Conventions

**Follow standard Go style:**
- Use `gofmt` to format code
- Use `golangci-lint` for linting
- Follow [Effective Go](https://go.dev/doc/effective_go)

**Naming:**
- Exported: `PascalCase`
- Unexported: `camelCase`
- Acronyms: `HTTPServer` not `HttpServer`

**Error Handling:**
```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to checkout branch: %w", err)
}

// Bad: Lose context
if err != nil {
    return err
}
```

**Context Usage:**
```go
// Good: Accept context as first parameter
func DoSomething(ctx context.Context, name string) error

// Bad: No context
func DoSomething(name string) error
```

### Project-Specific Conventions

**1. TUI Components**

File naming: `{feature}.go` (e.g., `issue_select.go`, `branch_input.go`)

Structure:
```go
// 1. Model definition
type featureModel struct { }

// 2. Constructor
func newFeatureModel() featureModel { }

// 3. Bubble Tea interface
func (m featureModel) Init() tea.Cmd { }
func (m featureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { }
func (m featureModel) View() string { }

// 4. Helper methods
func (m featureModel) helperMethod() { }

// 5. Entry point (if top-level)
func RunFeatureTUI() error { }
```

**2. Git Operations**

All functions should:
- Accept `context.Context` as first parameter
- Return error as last return value
- Use `ensureTimeout()` for default timeout
- Wrap errors with context

```go
func GitOperation(ctx context.Context, dir string, args ...string) error {
    ctx, cancel := ensureTimeout(ctx, 30*time.Second)
    defer cancel()

    if err := Run(ctx, dir, args...); err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}
```

**3. Message Types**

Use descriptive names:
```go
// Good
type issuesLoadedMsg struct { issues []git.Issue }
type gitSuccessMsg struct{}
type gitErrMsg struct { error }

// Bad
type dataMsg struct { data interface{} }
type msg1 struct{}
```

**4. Comments**

Add package comments:
```go
// Package tui provides terminal user interface components
// for the SESS CLI. It uses the Bubble Tea framework.
package tui
```

Add function comments for exported functions:
```go
// RunStartTUI launches the interactive session start workflow.
// If featureName is provided, it will be used as the default branch name.
func RunStartTUI(featureName string) error {
```

---

## Common Tasks

### Building for Distribution

**Single Platform:**
```bash
# Current platform
go build -o sess ./cmd/sess

# Linux
GOOS=linux GOARCH=amd64 go build -o sess-linux ./cmd/sess

# macOS
GOOS=darwin GOARCH=amd64 go build -o sess-macos ./cmd/sess

# Windows
GOOS=windows GOARCH=amd64 go build -o sess.exe ./cmd/sess
```

**All Platforms:**
```bash
# Build script
#!/bin/bash
platforms=("linux/amd64" "darwin/amd64" "darwin/arm64" "windows/amd64")

for platform in "${platforms[@]}"; do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output="sess-$GOOS-$GOARCH"

    if [ $GOOS = "windows" ]; then
        output+=".exe"
    fi

    echo "Building $output..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o $output ./cmd/sess
done
```

### Adding Dependencies

```bash
# Add a new dependency
go get github.com/some/package

# Update dependencies
go get -u ./...

# Tidy up go.mod
go mod tidy
```

### Debugging

**Print Debugging in Bubble Tea:**

Problem: `fmt.Println()` doesn't work because Bubble Tea controls the terminal.

Solution: Log to a file:

```go
// In your component
f, _ := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
defer f.Close()

fmt.Fprintf(f, "Debug: %+v\n", someVariable)
```

**Using Delve Debugger:**

```bash
# Install
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug ./cmd/sess -- start

# In debugger
(dlv) break internal/tui/start.go:100
(dlv) continue
(dlv) print myVariable
```

### Profiling Performance

```go
// Add to main.go
import (
    "runtime/pprof"
    "os"
)

func main() {
    // CPU profiling
    f, _ := os.Create("cpu.prof")
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    sess.Execute()
}
```

```bash
# Run and analyze
go build -o sess ./cmd/sess
./sess start
go tool pprof cpu.prof
```

---

## Troubleshooting

### Common Issues

#### 1. "command not found: git" or "command not found: gh"

**Problem:** External dependencies not installed

**Solution:**
```bash
# Check if installed
which git
which gh

# Install if missing
# git: https://git-scm.com/downloads
# gh: https://cli.github.com/
```

#### 2. TUI Not Updating Properly

**Problem:** Forgot to return command from Update()

**Solution:**
```go
// Bad
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    m.spinner.Update(msg)  // Lost the command!
    return m, nil
}

// Good
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    newSpinner, cmd := m.spinner.Update(msg)
    m.spinner = newSpinner
    return m, cmd  // Return the command
}
```

#### 3. Context Deadline Exceeded

**Problem:** Operation taking longer than timeout

**Solution:**
```go
// Increase timeout for specific operation
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

err := git.SomeLongOperation(ctx, ".")
```

#### 4. Race Conditions in Channels

**Problem:** Sending to channel after program quit

**Solution:**
```go
// Use WaitGroup to ensure goroutines complete
var wg sync.WaitGroup
wg.Add(1)

go func() {
    defer wg.Done()
    for line := range stdout {
        program.Send(gitLineMsg(line))
    }
}()

wg.Wait()  // Wait before returning
return gitSuccessMsg{}
```

#### 5. Terminal Messed Up After Crash

**Problem:** Bubble Tea doesn't restore terminal on crash

**Solution:**
```bash
# Reset terminal
reset

# Or
stty sane
```

---

## Development Tips

### 1. Use the TUI Wisely

**Do:**
- Keep Update() logic simple
- Use messages for all state changes
- Compose components for complex UIs
- Test components in isolation

**Don't:**
- Put business logic in Update()
- Mutate model directly without returning it
- Block in Update() - use commands for async work
- Forget to handle tea.KeyMsg for quit (ctrl+c)

### 2. Git Operations

**Do:**
- Always pass context
- Wrap errors with helpful messages
- Use streaming for long operations
- Check for git availability early

**Don't:**
- Assume git commands succeed
- Ignore stderr output
- Run git without timeout
- Parse complex git output (use porcelain commands)

### 3. Error Handling

**Do:**
```go
// Provide actionable error messages
if err != nil {
    return fmt.Errorf("failed to create branch '%s': %w\nMake sure you're in a git repository", branch, err)
}
```

**Don't:**
```go
// Swallow errors silently
if err != nil {
    // TODO: handle this
}

// Vague error messages
if err != nil {
    return fmt.Errorf("something went wrong: %w", err)
}
```

### 4. Testing Strategy

**Test Pyramid:**
```
        /\
       /  \  Few E2E tests (manual)
      /____\
     /      \ Some integration tests
    /________\
   /          \ Many unit tests
  /____________\
```

**Focus on:**
- Unit testing pure functions (sanitization, parsing)
- Integration testing git operations
- Manual testing full workflows

---

## Resources

### Documentation

- [Bubble Tea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Bubbles Components](https://github.com/charmbracelet/bubbles)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Cobra User Guide](https://cobra.dev/)
- [Effective Go](https://go.dev/doc/effective_go)

### Example Projects

- [Glow](https://github.com/charmbracelet/glow) - Markdown reader
- [Soft Serve](https://github.com/charmbracelet/soft-serve) - Git server
- [gh-dash](https://github.com/dlvhdr/gh-dash) - GitHub dashboard

### Getting Help

1. Check [ARCHITECTURE.md](ARCHITECTURE.md) for design decisions
2. Read the source code - it's well-structured!
3. Look at similar components in `internal/tui/`
4. Search Bubble Tea examples and discussions
5. Ask in project issues/discussions

---

## Contributing Guidelines

### Before You Commit

1. **Format code:** `gofmt -w .`
2. **Run linter:** `golangci-lint run`
3. **Test manually:** Try your changes in a real repo
4. **Check git status:** Don't commit debug logs or binaries
5. **Write clear commit messages:**
   ```
   Add pause command to suspend sessions

   - Created pause command in internal/sess
   - Implemented pause TUI workflow
   - Added session state persistence
   ```

### Pull Request Checklist

- [ ] Code is formatted with `gofmt`
- [ ] No linter warnings
- [ ] Manual testing completed
- [ ] Updated ARCHITECTURE.md if adding new patterns
- [ ] Updated this file if adding new development workflows
- [ ] Commit messages are descriptive

### Code Review Focus

Reviewers will look for:
- Proper error handling
- Context usage in git operations
- Message-driven TUI patterns
- Clear separation of concerns
- User-friendly error messages

---

Happy coding! If you have questions, check the architecture docs or open an issue.
