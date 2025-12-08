# SESS CLI - Architecture Documentation

## Table of Contents

1. [Overview](#overview)
2. [Project Structure](#project-structure)
3. [Architectural Patterns](#architectural-patterns)
4. [Core Components](#core-components)
5. [Data Flow](#data-flow)
6. [Key Abstractions](#key-abstractions)
7. [Technology Stack](#technology-stack)
8. [Design Decisions](#design-decisions)

---

## Overview

SESS (SESS Enables Structured Sessions) is a CLI tool that helps developers manage focused work sessions tied to GitHub issues. The architecture follows clean separation of concerns, with distinct layers for command routing, user interface, and external tool integration.

**Core Philosophy:**
- Interactive workflows guide users through complex git operations
- Real-time feedback keeps users informed during long-running operations
- GitHub integration links work to issues for better context
- Clean repository state is enforced before creating new branches

**Project Statistics:**
- ~1,767 lines of Go code
- 8 source files across 3 packages
- Built on Bubble Tea TUI framework
- Integrates with git and GitHub CLI

---

## Project Structure

```
sess-cli/
├── cmd/sess/              # Application entry point
│   └── main.go           # Minimal bootstrap (7 lines)
│
├── internal/             # Private application code
│   ├── sess/            # CLI command layer (Cobra)
│   │   ├── root.go      # Root command setup (44 lines)
│   │   └── start.go     # "start" subcommand (35 lines)
│   │
│   ├── tui/             # Terminal UI layer (Bubble Tea)
│   │   ├── start.go     # Main session workflow (474 lines)
│   │   ├── issue_select.go  # Issue selection UI (185 lines)
│   │   ├── common.go    # Git message streaming (207 lines)
│   │   └── styles.go    # Shared UI styling (68 lines)
│   │
│   └── git/             # External tool integration
│       ├── git.go       # Git operations wrapper (501 lines)
│       └── gh.go        # GitHub CLI wrapper (252 lines)
│
├── go.mod               # Go module definition
├── go.sum               # Dependency checksums
└── .gitignore          # Git ignore rules
```

### Package Responsibilities

| Package | Responsibility | Thickness | Dependencies |
|---------|---------------|-----------|--------------|
| `cmd/sess` | Entry point | Thin | `internal/sess` |
| `internal/sess` | Command routing | Thin | `internal/tui`, Cobra |
| `internal/tui` | UI orchestration | **Thick** | `internal/git`, Bubble Tea |
| `internal/git` | Tool integration | Medium | Standard library |

---

## Architectural Patterns

### 1. Hexagonal Architecture (Ports & Adapters)

The codebase follows hexagonal architecture principles:

```
┌─────────────────────────────────────────────┐
│           Adapters (Input)                  │
│  ┌─────────────┐     ┌──────────────┐      │
│  │ CLI (Cobra) │────▶│ TUI (Bubble) │      │
│  └─────────────┘     └──────────────┘      │
└─────────────────────┬───────────────────────┘
                      │
         ┌────────────▼────────────┐
         │      Core Domain        │
         │  ┌──────────────────┐   │
         │  │ Session Model    │   │
         │  │ Workflow Logic   │   │
         │  └──────────────────┘   │
         └────────────┬────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│         Adapters (Output)                   │
│  ┌──────────────┐    ┌──────────────┐      │
│  │ Git CLI      │    │ GitHub CLI   │      │
│  └──────────────┘    └──────────────┘      │
└─────────────────────────────────────────────┘
```

**Benefits:**
- Business logic independent of UI framework
- External tools can be swapped (e.g., use go-git instead of git CLI)
- Testing easier with clear boundaries
- UI can be replaced (could add web UI or GUI)

### 2. Layered Architecture

```
┌──────────────────────────────────────┐
│  Presentation Layer (CLI Commands)   │  ← Cobra command definitions
├──────────────────────────────────────┤
│  UI Layer (TUI)                      │  ← Bubble Tea components
├──────────────────────────────────────┤
│  Business Logic (Workflows)          │  ← Session orchestration
├──────────────────────────────────────┤
│  Integration Layer (Git/GitHub)      │  ← Wrapped external tools
├──────────────────────────────────────┤
│  External Systems                    │  ← git, gh CLIs
└──────────────────────────────────────┘
```

**Data Flow:** Top-down only - lower layers never call upper layers

### 3. Message-Driven Architecture (Elm Architecture)

Bubble Tea enforces the Elm Architecture pattern:

```
┌──────────────┐
│    Model     │  State container
└──────┬───────┘
       │
   ┌───▼────┐
   │ Update │────────┐  Message handler
   └───┬────┘        │
       │             │
   ┌───▼────┐    ┌───▼────┐
   │  View  │    │  Cmd   │  Side effects
   └────────┘    └────────┘
```

**Components:**
- **Model:** Holds all UI state
- **Init():** Returns initial state and commands
- **Update(msg):** Processes messages, returns new state
- **View():** Renders current state as string

**Benefits:**
- Predictable state updates
- Easy to reason about async operations
- Time-travel debugging possible
- Testable without UI

---

## Core Components

### 1. Command Layer (`internal/sess`)

**Purpose:** Map CLI commands to application functionality

**Files:**
- [root.go](internal/sess/root.go) - Root command setup with Fang/Cobra
- [start.go](internal/sess/start.go) - "sess start" command definition

**Key Characteristics:**
- Ultra-thin layer - no business logic
- Uses Cobra for command parsing
- Uses Fang for Bubble Tea integration
- Delegates immediately to TUI layer

**Example Flow:**
```go
// Command definition
startCmd := &cobra.Command{
    Use:   "start [feature-name]",
    RunE: func(cmd *cobra.Command, args []string) error {
        featureName := ""
        if len(args) > 0 {
            featureName = args[0]
        }
        return tui.RunStartTUI(featureName)  // Delegate to TUI
    },
}
```

### 2. TUI Layer (`internal/tui`)

**Purpose:** Orchestrate interactive workflows and provide real-time feedback

**Files:**
- [start.go](internal/tui/start.go) - Main session start workflow
- [issue_select.go](internal/tui/issue_select.go) - Issue selection component
- [common.go](internal/tui/common.go) - Git streaming utilities
- [styles.go](internal/tui/styles.go) - Shared Lipgloss styles

**Key Models:**

#### `issueSelectModel` (Issue Selection)
```go
type issueSelectModel struct {
    list     list.Model     // Bubbles list component
    issues   []git.Issue    // Available GitHub issues
    loading  bool           // Loading state
    spinner  spinner.Model  // Loading indicator
    err      error         // Error state
}
```

**State Machine:**
```
Loading → Loaded → Selected
   ↓
Error
```

#### `startModel` (Git Operations)
```go
type startModel struct {
    spinner   spinner.Model
    logs      []string      // Last 10 git output lines
    done      bool
    err       error
    operation string        // Current git command
}
```

**Message Types:**
- `gitLineMsg` - stdout line from git
- `gitErrLineMsg` - stderr line from git
- `gitSuccessMsg` - Operation completed
- `gitErrMsg` - Fatal error occurred

**Workflow Steps:**

1. **Issue Selection** (if no feature name provided)
   - Show prompt: "Select issue" vs "Start without issue"
   - If selecting, load issues from GitHub
   - Display filterable list
   - User selects or skips

2. **Branch Name Input** (if needed)
   - Text input component
   - Sanitize input for branch names
   - Validate non-empty

3. **Branch Type Selection**
   - List: feature/, bugfix/, refactor/
   - Creates prefix for branch name

4. **Repository Cleanliness Check**
   - Check `git status` for changes
   - If dirty, show options:
     - Stash changes
     - Commit changes
     - Discard changes
     - Quit

5. **Git Operations** (with streaming)
   - Checkout base branch (dev)
   - Pull latest changes
   - Create new feature branch
   - Stream output in real-time

### 3. Git Integration Layer (`internal/git`)

**Purpose:** Provide clean abstractions over git and GitHub CLI tools

**Files:**
- [git.go](internal/git/git.go) - Git command wrapper
- [gh.go](internal/git/gh.go) - GitHub CLI wrapper

#### Git Package Design

**Core Pattern:**
```go
// Context-aware execution with timeout
func Run(ctx context.Context, dir string, args ...string) error {
    ctx, cancel := ensureTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = dir
    return cmd.Run()
}
```

**Key Features:**
- All functions accept `context.Context`
- Default 30-second timeout if none provided
- Comprehensive error wrapping
- Streaming support for long operations

**High-Level Operations:**
- `Fetch()`, `Pull()`, `Push()` - Remote operations
- `Checkout()`, `Branch()`, `DeleteBranch()` - Branch management
- `Add()`, `Commit()`, `Status()` - Working tree operations
- `IsDirty()`, `CurrentBranch()` - State queries
- `RunGitWithOutput()` - Streaming stdout/stderr via channels

#### GitHub Package Design

**Issue Model:**
```go
type Issue struct {
    ID    string `json:"id"`
    Title string `json:"title"`
    URL   string `json:"url"`
}
```

**Key Operations:**
- `ListIssuesJSON()` - Get structured issue data
- `CreatePR()`, `MergePR()` - Pull request management
- `CreateIssue()`, `CloseIssue()` - Issue management
- `RepoView()`, `RepoClone()` - Repository operations

**Pattern:** Mirrors git package structure for consistency

---

## Data Flow

### Complete "sess start" Flow

```
User runs: sess start
    │
    ▼
┌─────────────────────────────────────────────┐
│ Cobra Command Handler                       │
│ internal/sess/start.go:startCmd.RunE        │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ TUI Entry Point                             │
│ internal/tui/start.go:RunStartTUI()         │
│ - Initializes Bubble Tea program           │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ Issue Selection Prompt                      │
│ - "Select issue" or "Start without"        │
└────────────────┬────────────────────────────┘
                 │
         ┌───────┴───────┐
         │               │
         ▼               ▼
   Select Issue    Skip Issue
         │               │
         ▼               │
┌──────────────────┐     │
│ Load Issues      │     │
│ from GitHub      │     │
│ (async)          │     │
└────────┬─────────┘     │
         │               │
         ▼               │
┌──────────────────┐     │
│ Show Issue List  │     │
│ (filterable)     │     │
└────────┬─────────┘     │
         │               │
         ▼               │
  Issue Selected         │
    (title→branch)       │
         │               │
         └───────┬───────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ Branch Name Input (if no name yet)         │
│ - Text input component                     │
│ - Sanitize for git                         │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ Branch Type Selection                       │
│ - feature/ bugfix/ refactor/               │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ Check Repository State                      │
│ internal/git/git.go:IsDirty()              │
└────────────────┬────────────────────────────┘
                 │
         ┌───────┴────────┐
         │                │
         ▼                ▼
     Is Dirty         Is Clean
         │                │
         ▼                │
┌──────────────────┐      │
│ Show Options:    │      │
│ - Stash          │      │
│ - Commit         │      │
│ - Discard        │      │
│ - Quit           │      │
└────────┬─────────┘      │
         │                │
         ▼                │
   Execute Choice         │
   (git command)          │
         │                │
         └────────┬───────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│ Start Git Workflow (with streaming)         │
│                                             │
│ Step 1: git checkout dev                   │
│   └─> Stream output to UI                  │
│                                             │
│ Step 2: git pull origin dev                │
│   └─> Stream output to UI                  │
│                                             │
│ Step 3: git checkout -b {type}/{name}      │
│   └─> Stream output to UI                  │
│                                             │
│ internal/tui/common.go:streamStep()         │
└────────────────┬────────────────────────────┘
                 │
         ┌───────┴────────┐
         │                │
         ▼                ▼
     Success          Error
         │                │
         ▼                ▼
   Show Success     Show Error
   Message          Message
```

### Git Streaming Architecture

**Problem:** Long-running git operations block UI

**Solution:** Asynchronous execution with message passing

```go
// internal/tui/common.go
func streamStep(
    program *tea.Program,
    dir string,
    args []string,
    next func() tea.Msg,
) tea.Cmd {
    return func() tea.Msg {
        // Run git in background
        stdout, stderr, errChan := git.RunGitWithOutput(ctx, dir, args...)

        var wg sync.WaitGroup
        wg.Add(2)

        // Stream stdout to UI
        go func() {
            for line := range stdout {
                program.Send(gitLineMsg(line))  // Send to Bubble Tea
            }
            wg.Done()
        }()

        // Stream stderr to UI
        go func() {
            for line := range stderr {
                program.Send(gitErrLineMsg(line))
            }
            wg.Done()
        }()

        // Wait for completion
        if err := <-errChan; err != nil {
            wg.Wait()
            return gitErrMsg{err}
        }

        wg.Wait()
        return next()  // Chain to next step
    }
}
```

**Benefits:**
- Non-blocking UI updates
- Real-time progress feedback
- Sequential command chaining
- Error handling at each step

---

## Key Abstractions

### 1. Session Model

```go
// internal/tui/start.go
type SessionState int

const (
    StateIdle SessionState = iota
    StateActive
    StatePaused
    StateEnded
)

type Model struct {
    Branch       string
    State        SessionState
    StartTime    time.Time
    PauseTime    time.Time
    TotalElapsed time.Duration
}

func (m *Model) Start() { /* ... */ }
func (m *Model) Pause() { /* ... */ }
func (m *Model) Resume() { /* ... */ }
func (m *Model) End() { /* ... */ }
```

**Current Status:** Infrastructure in place, not fully utilized yet

**Future Use:** Time tracking, productivity metrics, session history

### 2. Git Context Abstraction

All git operations accept `context.Context`:

```go
// Enables:
// - Cancellation (Ctrl+C)
// - Timeouts (prevent hanging)
// - Deadline propagation
// - Request-scoped values

func Pull(ctx context.Context, dir, remote, branch string) error
func Checkout(ctx context.Context, dir, branch string) error
func Push(ctx context.Context, dir, remote, branch string) error
```

**Pattern Applied Everywhere:** Consistent error handling and resource management

### 3. Command Streaming Pattern

**Interface:**
```go
func RunGitWithOutput(ctx context.Context, dir string, args ...string) (
    stdout <-chan string,
    stderr <-chan string,
    errChan <-chan error,
)
```

**Usage:**
```go
stdout, stderr, errChan := git.RunGitWithOutput(ctx, ".", "pull", "origin", "dev")

for {
    select {
    case line := <-stdout:
        program.Send(gitLineMsg(line))  // Update UI
    case line := <-stderr:
        program.Send(gitErrLineMsg(line))
    case err := <-errChan:
        if err != nil {
            return gitErrMsg{err}
        }
        return gitSuccessMsg{}
    }
}
```

**Benefits:**
- Real-time feedback
- Memory efficient (streaming vs buffering)
- Responsive UI during long operations

---

## Technology Stack

### Core Dependencies

```go
// CLI Framework
github.com/charmbracelet/fang      // Cobra + Bubble Tea integration
github.com/spf13/cobra             // Command parsing and routing

// TUI Framework
github.com/charmbracelet/bubbletea // Elm architecture for terminal
github.com/charmbracelet/bubbles   // Pre-built UI components
github.com/charmbracelet/lipgloss  // Terminal styling

// Standard Library
context        // Cancellation and timeouts
os/exec        // External command execution
sync           // Goroutine coordination
time           // Time tracking
encoding/json  // GitHub API responses
```

### External Tools

- **git** - Version control operations (required)
- **gh** - GitHub CLI for issue/PR management (required)

### Language and Runtime

- **Go 1.25.4** - Compiled binary, single-file distribution
- **No runtime dependencies** - Just needs git and gh installed

---

## Design Decisions

### 1. Why Bubble Tea for TUI?

**Decision:** Use Bubble Tea instead of alternatives (survey, promptui, termui)

**Rationale:**
- **Elm Architecture:** Predictable state management
- **Message-driven:** Natural fit for async operations
- **Testable:** Pure functions, no side effects in Update()
- **Composable:** Easy to build complex UIs from components
- **Active ecosystem:** Bubbles provides pre-built components

**Trade-offs:**
- Steeper learning curve than survey/promptui
- More verbose for simple prompts
- But: Much better for complex workflows like ours

### 2. Why Wrap Git CLI Instead of go-git?

**Decision:** Shell out to git CLI via `os/exec`

**Rationale:**
- **Reliability:** Git CLI is the reference implementation
- **Feature completeness:** All git features available immediately
- **User expectations:** Same output/behavior as manual git
- **Debugging:** Users can understand what's happening
- **Simplicity:** No need to learn go-git API

**Trade-offs:**
- Slower than native library (but acceptable for our use case)
- Requires git installed (but our users will have it)
- Output parsing can be fragile (but we mostly just stream it)

### 3. Why Context Everywhere?

**Decision:** All git operations accept `context.Context`

**Rationale:**
- **Timeouts:** Prevent hanging on network issues
- **Cancellation:** Ctrl+C can stop operations gracefully
- **Best practice:** Idiomatic Go for I/O operations
- **Future-proof:** Easy to add request-scoped values later

**Trade-offs:**
- More verbose function signatures
- Callers must provide context
- But: Worth it for robustness

### 4. Why Internal Package?

**Decision:** All code in `internal/` directory

**Rationale:**
- **Not a library:** No public API to maintain
- **Freedom to refactor:** Can change anything without breaking users
- **Clear intent:** This is application code, not reusable components
- **Go convention:** Standard pattern for CLI tools

### 5. Why Minimal Command Layer?

**Decision:** Keep `internal/sess` ultra-thin

**Rationale:**
- **Single responsibility:** Just command routing
- **Testability:** Business logic in TUI, easier to test
- **Flexibility:** Can add more UIs (web, GUI) without duplicating logic
- **Clarity:** Easy to see all available commands

**Pattern:**
```go
// Command layer: Just routing
RunE: func(cmd *cobra.Command, args []string) error {
    return tui.RunStartTUI(args[0])  // Delegate immediately
}
```

### 6. Why Sequential Git Operations?

**Decision:** Chain git commands with callbacks, not parallel

**Rationale:**
- **Dependencies:** Each step depends on previous (can't pull before checkout)
- **Error handling:** Stop on first failure
- **User feedback:** Show progress step-by-step
- **Simplicity:** Easier to reason about than parallel execution

**Implementation:**
```go
// Callback chaining
streamStep(program, ".", []string{"checkout", "dev"}, func() tea.Msg {
    return streamStep(program, ".", []string{"pull", "origin", "dev"}, func() tea.Msg {
        return streamStep(program, ".", []string{"checkout", "-b", branch}, func() tea.Msg {
            return gitSuccessMsg{}
        })
    })
})
```

---

## Future Architecture Considerations

### Potential Enhancements

1. **Configuration System**
   - Config file support (YAML/JSON)
   - User preferences (default branch type, base branch)
   - Per-repo settings

2. **State Persistence**
   - Session history storage (SQLite?)
   - Time tracking database
   - Analytics and insights

3. **Plugin System**
   - Custom branch naming strategies
   - Alternative issue trackers (Jira, Linear)
   - Custom git workflows

4. **Testing Infrastructure**
   - Mock git operations for unit tests
   - Snapshot testing for TUI components
   - Integration tests with git fixtures

5. **Alternative Interfaces**
   - Web UI for session dashboard
   - VS Code extension
   - API server for IDE integrations

### Architecture Readiness

The current architecture supports these enhancements:

- **Layered design:** Can add persistence layer below git package
- **Hexagonal architecture:** Can add new adapters (plugins) easily
- **Message-driven UI:** TUI components are composable and testable
- **Context-aware:** Easy to add request-scoped values (user ID, session ID)

---

## Glossary

**Bubble Tea** - Terminal UI framework using Elm architecture

**Cobra** - CLI framework for command parsing and routing

**Elm Architecture** - Pattern with Model, Update, View functions

**Fang** - Library that integrates Cobra commands with Bubble Tea programs

**Lipgloss** - Terminal styling library (colors, borders, layout)

**Bubbles** - Pre-built Bubble Tea components (list, spinner, input)

**Session** - A focused work period tied to a git branch and optionally a GitHub issue

**Streaming** - Sending output incrementally as it's produced, not all at once

**Message** - Data structure sent to Bubble Tea Update() function to trigger state changes

---

## References

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Cobra Documentation](https://cobra.dev/)
- [Elm Architecture Guide](https://guide.elm-lang.org/architecture/)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
