# SESS - Session-Based Git CLI

> **S**ESS **E**nables **S**tructured **S**essions

A CLI tool that helps developers manage focused work sessions tied to GitHub issues. SESS externalizes your executive function by tracking session states, automating git workflows, and reducing context-switching friction.

## Philosophy: State-Based Work

Every project tracked by SESS exists in one of four states:

```
┌──────┐    start    ┌────────┐    pause     ┌────────┐
│ IDLE │────────────>│ ACTIVE │<────────────>│ PAUSED │
└──────┘             └────────┘    resume    └────────┘
                          │
                          │ end
                          ▼
                     ┌────────┐
                     │ ENDED  │
                     └────────┘
```

**State Definitions:**

- **IDLE** - No active session. Repository is clean, on base branch (e.g., `dev`)
- **ACTIVE** - Work in progress. Branch created, time being tracked, changes happening
- **PAUSED** - Session temporarily suspended. Branch exists, timer stopped, state preserved
- **ENDED** - Session complete. Changes committed, PR created, back to base branch

**Why This Matters:**

Instead of managing git branches, commits, and PRs manually, SESS makes you think in terms of **work sessions**. Start when you begin focused work, pause when interrupted, resume when ready, end when done. The tool handles the git ceremony.

---

## Current Features (MVP1 - v0.2.1)

### Session Management

- **Start Sessions** - Create feature branches with full context tracking
- **Pause/Resume** - Interrupt work and resume later with preserved state
- **View Status** - Check active session details with elapsed time
- **List Projects** - See all tracked projects across your system

### Interactive Workflows

- **GitHub Issue Integration** - Select issues directly from the CLI
- **Branch Type Selection** - Automatic prefixes: `feature/`, `bugfix/`, `refactor/`
- **Repository State Management** - Ensures clean state before branching (stash/commit/discard)
- **Real-time Git Streaming** - Live output during long-running operations

### Persistence & Tracking

- **SQLite Database** - Global session tracking at `~/.sess-cli/sess.db`
- **Project Tracking** - Automatically tracks all repositories you use SESS in
- **Time Tracking** - Cumulative elapsed time across pause/resume cycles
- **Session History** - All sessions stored for future analytics

### Demo

```bash
# Start a new session
$ sess start

? What would you like to do?
  > Select a GitHub issue
    Start without an issue

? Select an issue:
  > #123 - Add user authentication
    #124 - Fix login bug
    #125 - Refactor database layer

? Enter branch name: user-authentication

? Select branch type:
  > feature/
    bugfix/
    refactor/

✓ Checking out dev
✓ Pulling latest changes
✓ Creating branch feature/user-authentication

Session started! Happy coding.
```

---

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd sess-cli

# Build
go build -o sess ./cmd/sess

# Move to PATH (optional)
sudo mv sess /usr/local/bin/
```

### Prerequisites

- **Git** - [Install Git](https://git-scm.com/downloads)
- **GitHub CLI (gh)** - [Install gh](https://cli.github.com/)
- **Go 1.25+** (for building from source)

---

## Quick Start

### 1. Initialize in Your Repository

```bash
cd your-project
sess start
```

### 2. Start a Session

```bash
# Interactive mode - select issue from GitHub
sess start

# Quick mode - provide feature name directly
sess start user-profile-page
```

### 3. Work on Your Feature

```bash
# Make changes, commit as usual
git add .
git commit -m "Implement user profile API"
```

### 4. Check Session Status

```bash
# See active session details
sess status
# Output:
# Project: sess-cli
# Path: /path/to/sess-cli
# Base Branch: dev
#
# State: ACTIVE
# Branch: feature/user-profile-page
# Issue: #123 - Add user authentication
# Elapsed: 1h 23m 45s
# Started: 2025-12-08 14:30:00
```

### 5. Pause When Interrupted

```bash
sess pause
# Session paused
# Branch: feature/user-profile-page
# Total elapsed: 1h 23m 45s
#
# Resume anytime with: sess resume
```

### 6. Resume Later

```bash
sess resume
# Checking out branch: feature/user-profile-page
# Branch checked out
#
# Session resumed
# Branch: feature/user-profile-page
# Issue: #123 - Add user authentication
# Total elapsed: 1h 23m 45s
```

### 7. View All Projects

```bash
sess projects
# Tracked Projects (3)
#
# 1. sess-cli (current)
#    /path/to/sess-cli
#    Base: dev
#    Session: active on feature/user-profile
#    Elapsed: 1h 23m
#    Last used: just now
#
# 2. my-app
#    /path/to/my-app
#    Base: main
#    No active session
#    Last used: 2 hours ago
```

### 8. End Session and Create PR (Coming in Phase 3)

```bash
sess end
# Prompts for PR description
# Rebases onto dev
# Pushes branch
# Opens PR linked to issue
# Switches back to dev
```

---

## Commands Reference

| Command | Status | Description |
|---------|--------|-------------|
| `sess start [name]` | ✅ **MVP1** | Start a new session, optionally linked to GitHub issue |
| `sess status` | ✅ **MVP1** | Show current session status with elapsed time |
| `sess pause` | ✅ **MVP1** | Pause current session and stop time tracking |
| `sess resume` | ✅ **MVP1** | Resume paused session and continue time tracking |
| `sess projects` | ✅ **MVP1** | List all tracked projects across the system |
| `sess end` | 🚧 [Phase 3 — #3](../../issues/3) | End session, commit, push, open PR |
| `sess auth` | 🚧 [Phase 4 — #6](../../issues/6) | Authenticate with GitHub (currently uses `gh` auth) |
| `sess config` | 🚧 [Phase 4 — #7](../../issues/7) | Initialize CLI in repo with custom settings |

---

## Roadmap

### Phase 1: Core Session Management ✅ **COMPLETE**

- [x] Interactive TUI for session start
- [x] GitHub issue selection
- [x] Branch creation with type prefixes
- [x] Repository state validation
- [x] Real-time git operation feedback
- [x] Basic workflow automation

### Phase 2: State Persistence ✅ **COMPLETE (MVP1 - v0.2.0)**

**Goal:** Track session state across commands

- [x] **Database Integration**
  - [x] SQLite database at `~/.sess-cli/sess.db`
  - [x] Projects table for repository tracking
  - [x] Sessions table for state persistence
  - [x] Pure Go implementation (no CGO)

- [x] **Session Model Activation**
  - [x] Store active session in database
  - [x] Persist: branch, issue, start time, state (active/paused)
  - [x] Load session state on any command
  - [x] Track elapsed time across pause/resume cycles

- [x] **`sess status` Command**
  - [x] Show current session state (idle/active/paused)
  - [x] Display branch, linked issue, elapsed time
  - [x] Show project information

- [x] **`sess pause` Command**
  - [x] Mark session as paused
  - [x] Stop time tracking
  - [x] Preserve branch and context in database

- [x] **`sess resume` Command**
  - [x] Resume paused session
  - [x] Continue time tracking
  - [x] Auto-checkout session branch if needed

- [x] **`sess projects` Command**
  - [x] List all tracked projects globally
  - [x] Show session status for each project
  - [x] Display last used timestamps

### Phase 3: End-to-End Workflow 🚧 **NEXT**

**Goal:** Complete the session lifecycle from start to PR

- [ ] **`sess end` Command** — [Issue #3](../../issues/3)
  - [ ] Interactive PR description input (use PR template if exists)
  - [ ] Commit all changes with user message
  - [ ] Rebase onto base branch (`dev`)
  - [ ] Conflict detection and resolution guidance
  - [ ] Push branch to remote
  - [ ] Create GitHub PR via `gh` CLI
  - [ ] Link PR to issue automatically
  - [ ] Switch back to base branch
  - [ ] Mark session as ended
  - [ ] Show session summary (duration, commits, PR link)

- [ ] **Conflict Handling** — [Issue #4](../../issues/4)
  - [ ] Detect rebase conflicts
  - [ ] Pause workflow, provide resolution instructions
  - [ ] Resume PR creation after resolution

### Phase 4: Authentication & Configuration

**Goal:** Flexible setup and secure authentication

- [ ] **`auth login` Command** — [Issue #6](../../issues/6)
  - [ ] GitHub OAuth flow
  - [ ] Store token securely (OS keychain or encrypted config)
  - [ ] Fallback to `gh` CLI authentication

- [ ] **`config init` Command** — [Issue #7](../../issues/7)
  - [ ] Interactive setup wizard
  - [ ] Configure: organization, default base branch, branch naming
  - [ ] Store in `.sess-cli/config.json`

- [ ] **Per-Repo Configuration**
  - [ ] Override global settings per repository
  - [ ] Custom branch prefixes
  - [ ] Default issue labels
  - [ ] PR template path

### Phase 5: Time Tracking & Analytics 🔄 **PARTIALLY COMPLETE**

**Goal:** Understand productivity patterns

- [x] **Session History** ✅ (MVP1)
  - [x] Store completed sessions in SQLite database
  - [x] Track: duration, issue, branch, state
  - [ ] Track: commits, PR link (Phase 3)

- [ ] **Analytics Commands** — [Issue #8](../../issues/8)
  - [ ] `sess history` - Show recent sessions
  - [ ] `sess stats` - Time spent per issue, average session length
  - [ ] `sess report` - Weekly/monthly productivity insights

- [ ] **Visualizations**
  - [ ] Session timeline (gantt chart in terminal)
  - [ ] Focus time heatmap
  - [ ] Issue completion velocity

### Phase 6: Advanced Features

**Goal:** Power user capabilities

- [ ] **Multi-Session Support**
  - [ ] Track multiple sessions across branches
  - [ ] Switch between sessions with `sess switch`

- [ ] **Hooks & Extensibility**
  - [ ] Pre-start, post-end hooks
  - [ ] Custom scripts for workflow steps
  - [ ] Plugin system for custom integrations

- [ ] **Alternative Issue Trackers**
  - [ ] Jira integration
  - [ ] Linear integration
  - [ ] Generic webhook support

- [ ] **Team Features**
  - [ ] Shared session templates
  - [ ] Team analytics dashboard (web UI)
  - [ ] Session handoff (pair programming)

---

## Architecture

SESS follows a clean, layered architecture:

```
┌─────────────────────────────────────┐
│  CLI Layer (Cobra)                  │  Commands and routing
├─────────────────────────────────────┤
│  TUI Layer (Bubble Tea)             │  Interactive workflows
├─────────────────────────────────────┤
│  Business Logic                     │  Session orchestration
├─────────────────────────────────────┤
│  Integration Layer                  │  Git/GitHub wrappers
├─────────────────────────────────────┤
│  External Tools (git, gh)           │  Shell commands
└─────────────────────────────────────┘
```

**Key Technologies:**

- **Bubble Tea** - Terminal UI framework (Elm architecture)
- **Cobra** - Command routing and parsing
- **Lipgloss** - Terminal styling
- **Go Standard Library** - Context, exec, sync

For detailed architecture documentation, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## Development

### Building from Source

```bash
# Clone repository
git clone <repository-url>
cd sess-cli

# Install dependencies
go mod download

# Build
go build -o sess ./cmd/sess

# Run
./sess start
```

### Project Structure

```
sess-cli/
├── cmd/sess/              # Application entry point
├── internal/
│   ├── sess/             # CLI commands (Cobra)
│   ├── tui/              # Terminal UI (Bubble Tea)
│   └── git/              # Git/GitHub integration
├── ARCHITECTURE.md       # Architecture documentation
├── DEVELOPMENT.md        # Developer guide
└── technical-spec.md     # Original specification
```

### Contributing

We welcome contributions! Please see [DEVELOPMENT.md](DEVELOPMENT.md) for:

- Development setup and workflow
- Adding new commands and TUI components
- Code style guidelines
- Testing strategies

**Before You Commit:**

1. Format code: `gofmt -w .`
2. Run linter: `golangci-lint run`
3. Test manually in a real repository
4. Write clear commit messages

---

## Design Principles

### 1. Clarity Over Cleverness

Commands are predictable and self-descriptive. No hidden behaviors.

```bash
# Good: Obvious what this does
sess start my-feature

# Bad: Magic behavior
sess my-feature  # Does it start? Create? Switch?
```

### 2. State Awareness

The CLI always tells you what state you're in.

```bash
$ sess status
Session: ACTIVE
Branch: feature/user-auth
Issue: #123 - Add user authentication
Duration: 1h 23m
State: Working on feature implementation
```

### 3. Human-Centered Language

Commands map to mental models of work, not git internals.

```bash
# Human: I want to start working
sess start

# Not: sess create-branch-and-checkout
```

### 4. Progressive Disclosure

Simple by default, powerful when needed.

```bash
# Simple: Interactive prompts guide you
sess start

# Advanced: Skip prompts with flags
sess start --issue 123 --type feature --branch user-auth
```

### 5. Safe Defaults

Always assume sensible defaults, allow overrides.

- Default base branch: `dev`
- Default branch type: `feature/`
- Default PR template: Repository's `.github/pull_request_template.md`

### 6. Feedback and Confidence

Every step is confirmed. No ambiguity.

```bash
✓ Checking out dev
✓ Pulling latest changes
✓ Creating branch feature/user-auth
Session started at 14:56 GMT
```

---

## Why SESS?

### The Problem

Modern software development involves constant context switching:

- Check issue tracker → remember what to build
- Create git branch → remember naming convention
- Code for hours → interrupt for meeting
- Resume coding → what was I doing?
- Finish feature → remember rebase, commit, push, PR workflow
- Open PR → manually link to issue, fill template

**Result:** Cognitive overhead, lost productivity, forgotten steps.

### The Solution

SESS externalizes your executive function:

- **One command to start:** `sess start` handles branch creation, issue linking, setup
- **State preservation:** Pause and resume without losing context
- **Workflow automation:** `sess end` handles the entire PR creation ceremony
- **Time tracking:** Know exactly how long you spent on each feature
- **History:** Review past sessions, understand productivity patterns

### Who Is This For?

- **Solo developers** who want structure and productivity tracking
- **Teams** using GitHub issues and a git-flow style workflow
- **ADHD developers** who benefit from externalized task management
- **Anyone** tired of remembering git workflow steps

---

## Examples

### Scenario 1: Quick Feature Branch

```bash
# Start with just a name
$ sess start user-profile

? Select branch type:
  > feature/

✓ Created branch feature/user-profile
Session started!

# Work happens...

# End session (when implemented)
$ sess end
? Enter PR description: Added user profile page with avatar upload
✓ Opened PR #45
Session ended (duration: 2h 15m)
```

### Scenario 2: Bug Fix from Issue

```bash
$ sess start

? What would you like to do?
  > Select a GitHub issue

? Select an issue:
  > #124 - Login fails on mobile

? Select branch type:
  > bugfix/

✓ Created branch bugfix/login-fails-on-mobile
✓ Linked to issue #124

# Fix the bug, test

$ sess end
? Enter PR description: Fixed mobile viewport issue in auth component
✓ Rebased onto dev
✓ Pushed branch
✓ Opened PR #46 linked to issue #124
✓ Switched back to dev
Session ended (duration: 45m)
```

### Scenario 3: Interrupted Work

```bash
$ sess start refactoring-auth

# Start working...

# Meeting interruption!
$ sess pause
Session paused. Resume anytime with: sess resume

# After meeting
$ sess resume
Session resumed. Elapsed time: 32m

# Continue working

$ sess end
```

---

## Configuration

### Default Configuration (Coming in Phase 4)

SESS will look for configuration in these locations:

1. **Repository-specific:** `.sess-cli/config.json` (highest priority)
2. **User global:** `~/.config/sess-cli/config.json`
3. **Built-in defaults**

### Example Config

```json
{
  "baseBranch": "dev",
  "branchTypes": ["feature/", "bugfix/", "refactor/", "chore/"],
  "defaultBranchType": "feature/",
  "organization": "your-org",
  "prTemplate": ".github/pull_request_template.md",
  "hooks": {
    "preStart": "./scripts/pre-start.sh",
    "postEnd": "./scripts/notify-team.sh"
  }
}
```

---

## FAQ

### How is this different from `git-flow` or `GitHub Flow`?

SESS adds a **session-oriented layer** on top of your git workflow:

- **git-flow/GitHub Flow:** Focus on branch management
- **SESS:** Focus on work sessions with automatic branch management

SESS can work **with** any git workflow model. It automates the repetitive parts.

### Do I have to use GitHub Issues?

Currently, yes. Future versions will support Jira, Linear, and generic issue trackers.

### Can I use this without issues?

Yes! Run `sess start my-feature` to create a branch without linking to an issue.

### What if I already have a branch?

Currently, SESS creates new branches. Future versions will support adopting existing branches as sessions.

### Does SESS replace git?

No! SESS wraps git and GitHub CLI. You can still use git commands directly. SESS just automates common workflows.

### What data does SESS store?

Locally only:

- Session state (`.sess-cli/session.json`)
- Session history (`.sess-cli/sessions.db`)
- Configuration (`.sess-cli/config.json`)

Nothing is sent to external servers (except GitHub API calls via `gh`).

---

## Troubleshooting

### "command not found: gh"

Install GitHub CLI: <https://cli.github.com/>

### "Not a git repository"

SESS must be run inside a git repository. Run `git init` first.

### "Failed to fetch issues"

Ensure you're authenticated with GitHub:

```bash
gh auth login
```

### TUI looks broken / weird characters

Your terminal might not support UTF-8 or 256 colors. Try a modern terminal:

- macOS: iTerm2, Warp
- Linux: GNOME Terminal, Alacritty
- Windows: Windows Terminal

### Session state not persisting

Session persistence is not yet implemented (Phase 2). Currently, sessions only exist during the `sess start` command.

---

## Inspiration

SESS is inspired by:

- **Pomodoro Technique** - Time-boxed focused work sessions
- **GTD (Getting Things Done)** - Externalized task tracking
- **Git-flow** - Structured branching model
- **GitHub CLI** - Terminal-first GitHub integration
- **Bubble Tea** - Delightful terminal UIs

---

## License

[MIT License](LICENSE) (or your chosen license)

---

## Support

- **Issues:** [GitHub Issues](https://github.com/your-org/sess-cli/issues)
- **Discussions:** [GitHub Discussions](https://github.com/your-org/sess-cli/discussions)
- **Documentation:** [ARCHITECTURE.md](ARCHITECTURE.md) | [DEVELOPMENT.md](DEVELOPMENT.md)

---

## Acknowledgments

Built with:

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) by Charm
- [Cobra](https://github.com/spf13/cobra) by spf13
- [GitHub CLI](https://cli.github.com/) by GitHub

---

**Start managing your work sessions today:**

```bash
sess start
```

Focus on the work. Let SESS handle the ceremony.
