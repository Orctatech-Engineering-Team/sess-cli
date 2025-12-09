# Technical Specification: Session-Based Git CLI (SESS)

## Purpose

Enable developers to manage **focused work sessions** tied to GitHub issues directly from the terminal. Each session represents a unit of deep work, with start/pause/end states, and integrates seamlessly into the Git workflow.

## Core Features

- **Authentication**
    - GitHub OAuth via personal access token
    - Store credentials securely (local config file or OS keychain).
- **Session Management**
    - Start a session linked to a GitHub issue or create a GitHub issue
    - Temporarily stop tracking work.
    - End session, commit changes, and prepare PR. ( go through contribution git flow )
- **Git Workflow Integration**
    - Automatically create a branch from `dev`. ( feat, bug-fix, refactor )
    - Rebase onto `dev` before PR creation to minimize conflicts.
    - Switch back to `dev` branch after session ends.
- **Pull Request Automation**
    - Collect final comment message from user. ( use pull request template format )
    - Push branch to remote.
    - Open a GitHub pull request via API.
- **Tracking**
    - Local session log  to record start/pause/end times.
    - Optional metadata: duration, linked issue, branch name.

## CLI Commands

| Command | Description |
| --- | --- |
| `auth login` | Authenticate with GitHub and store token |
| `start` | Start a new session linked to issue |
| `pause` | Pause current session |
| `end` | End session, commit, push, open PR |
| `status` | Show active session details |
| `config init` | Initialize CLI in a repo (set org, default branch) |
|  |  |

## Architecture

- **Language:** Go
- **Modules:**
    - `auth`: Handles GitHub OAuth/token storage.
    - `git`: Wraps Git commands (branching, rebasing, commits).
    - `github`: API client for issues and PRs.
    - `config`: Reads/writes CLI configuration.
- **Data Storage:**
    - Local `.sess-cli/config.json` for settings.
    - Local `.sess-cli/sessions.json` or SQLite for session logs.

## Workflow Example

1. Developer runs `sess start` → CLI:
    - Authenticates with GitHub.
    - Creates branch from `dev`.
    - Logs session start.
2. Developer codes, commits locally.
3. Developer runs `sess end` → CLI:
    - Prompts for final comment.
    - Rebases branch onto `dev`.
    - Pushes branch.
    - Opens PR linked to issue.
    - Switches back to `dev`.
    - Logs session end.

# CLI Command Structure & Philosophy

## Design Philosophy

- **Clarity over cleverness**: Commands should be predictable, self-descriptive, and avoid hidden behaviors.
- **Consistency**: Every command follows the same verb–noun pattern (`sess start`, `sess end`).
- **Minimal friction**: Reduce cognitive load - developers shouldn’t need to remember complex flags.
- **Progressive disclosure**: Start simple, allow advanced options via flags/config.
- **State awareness**: CLI should always tell the user what state they’re in (active, paused, ended).
- **Human-centered**: Commands map to mental models of work (start, pause, end) rather than Git internals.

## Core Command Surface

| Command | Purpose | Principle |
| --- | --- | --- |
| `sess start` | Begin a focused unit of work tied to a GitHub issue | **Direct mapping**: “start” matches mental model of beginning work |
| `sess pause` | Temporarily stop tracking work | **Visibility of system state**: user knows session is paused |
| `sess resume` | Continue paused session | **Consistency**: mirrors pause |
| `sess end` | End session, commit, push, open PR | **Closure**: clear finish to a unit of work |
| `sess status` | Show current session details (branch, issue, elapsed time) | **Feedback**: system tells user what’s happening |
| `auth login` | Authenticate with GitHub | **User control**: explicit login step |
| `config init` | Initialize CLI in repo (set org, default branch) | **Discoverability**: easy setup |

## Command Syntax Principles

- **Verb–noun structure**: `sess start`, `session end` (easy to parse mentally).
- **Short flags for speed, long flags for clarity**:
    - `sess start 123 -m "Exploring fix"` vs `session start --issue 123 --message "Exploring fix"`.
- **Idempotence**: Running `sess start` twice should warn, not break.
- **Safe defaults**: Always branch from `dev`, unless overridden.
- **Explicitness**: Never assume issue linkage unless user specifies.

## Interaction Flow

### Starting a Session

```bash
$ session start 123
✔ Linked to GitHub issue #123
✔ Created branch session/123 from dev
Session started at 14:56 GMT

```

- **Feedback**: Immediate confirmation.
- **Visibility**: Shows branch + issue linkage.
- **Minimalism**: No extra info unless requested (`-verbose`).

### Ending a Session

```bash
$ session end
Enter final comment: Fix applied to auth middleware
✔ Rebasing onto dev
✔ Pushed branch session/123
✔ Opened PR #45 linked to issue #123
Switched back to dev
Session ended (duration: 3h 12m)

```

- **Closure**: Clear finish.
- **Feedback**: Each step shown.
- **Human language**: “Session ended” instead of “Process complete”.

## API Surface Design

- **Commands as verbs**: `start`, `pause`, `resume`, `end`.
- **Flags for context**:
    - `-issue <id>` (link to GitHub issue).
    - `-message <text>` (commit/PR message).
    - `-branch <name>` (override default branch).
- **Config-driven defaults**:
    - Default org, default base branch stored in config.
- **Interactive prompts (Bubble Tea)**:
    - For final comment at `session end`.
    - For status display (`session status`).

Perfect, Kirk — let’s codify both pieces: the **CLI help screen (API surface)** and the **Git workflow spec** so you have a concrete, human-centered design to build from.

# Git Workflow

**Branching Model**

- `main` → production-ready code.
- `dev` → integration branch, all work branches stem from here.
- `eg. feat/<issue-id>` → ephemeral feature branches tied to sessions.

**Session Lifecycle**

- **Start**:
    - Create branch  from `dev`.
    - Log session metadata (issue ID, start time).
- **Pause**:
    - Mark session paused in local log.
    - No Git action.
- **Resume**:
    - Continue work on same branch.
- **End**:
    - Prompt for final comment.
    - Commit changes.
    - Rebase branch onto `dev`.
    - Push branch to remote.
    - Open PR linked to issue.
    - Switch back to `dev`.
    - Log session end (duration, PR ID).

**Pull Request Rules**

- PR title: `<issue-id>: <message>`.
- PR body: follow pr template
- Always linked to GitHub issue via API.

**Conflict Handling**

- On `sess end`, CLI runs `git fetch && git rebase dev`.
- If conflicts → CLI pauses workflow, instructs user to resolve manually.
- After resolution → resume PR creation.

**Hooks & Automation**

- Pre-commit hooks (via Husky) enforce lint/tests.
- Optional GitHub Actions: auto-close session when PR merged.

# Philosophy Embedded

- **Sessions = units of deep work** → externalized executive function.
- **Branching = clarity** → every session isolated, traceable to an issue.
- **Automation = reduced friction** → developers focus on value, not ceremony.
- **Feedback = confidence** → CLI confirms every step, avoids ambiguity.