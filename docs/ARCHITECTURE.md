# Architecture

This document describes the current SESS architecture at `v0.3.0`.

SESS is a Go CLI that manages developer work as explicit sessions tied to repositories, branches, optional GitHub issues, and eventually pull requests.

## Overview

The codebase is intentionally layered:

```text
cmd/sess
  -> internal/sess
      -> internal/tui
      -> internal/session
      -> internal/db
      -> internal/git
```

Each layer has a distinct responsibility:

- `cmd/sess`: application entrypoint
- `internal/sess`: Cobra/Fang command wiring
- `internal/tui`: interactive workflows and Bubble Tea models
- `internal/session`: lifecycle rules for projects and sessions
- `internal/db`: SQLite schema and persistence
- `internal/git`: wrappers around `git` and `gh`

## Project Structure

```text
sess-cli/
├── cmd/sess/
│   └── main.go
├── internal/
│   ├── db/
│   │   ├── db.go
│   │   └── db_test.go
│   ├── git/
│   │   ├── gh.go
│   │   └── git.go
│   ├── sess/
│   │   ├── root.go
│   │   ├── start.go
│   │   ├── status.go
│   │   ├── pause.go
│   │   ├── resume.go
│   │   ├── end.go
│   │   └── projects.go
│   ├── session/
│   │   ├── session.go
│   │   └── session_test.go
│   └── tui/
│       ├── common.go
│       ├── end.go
│       ├── end_test.go
│       ├── issue_select.go
│       ├── start.go
│       └── styles.go
└── docs/
```

## Runtime Model

SESS tracks two core concepts:

- `Project`: a tracked repository on disk
- `Session`: a unit of work inside a project

Session states:

- `active`
- `paused`
- `ended`

Only one active or paused session may exist per project at a time.

## Data Flow

### `sess start`

1. Command resolves the current working directory and opens SQLite.
2. TUI validates that the directory is a git repository.
3. Session manager initializes or loads the tracked project.
4. TUI collects issue, branch name, branch type, and dirty-worktree choices.
5. Git wrappers create the session branch from the tracked base branch.
6. Session manager persists the new active session.

### `sess resume`

1. Command loads the tracked project and paused session.
2. Git wrapper checks the current branch.
3. If needed, SESS checks out the saved session branch.
4. Session manager resumes time tracking only after branch alignment succeeds.

### `sess end`

1. Command loads the tracked project and active or paused session.
2. TUI aligns the shell to the saved session branch if necessary.
3. TUI checks whether there is shippable work.
4. If dirty, SESS prompts for a commit message and commits the work.
5. SESS fetches, rebases, and pushes the branch.
6. SESS reuses an existing PR or creates a new one through `gh`.
7. SESS checks out the project base branch.
8. Session manager marks the session ended and stores PR metadata.

Failure rule:

- if an end-workflow step fails before session completion, the session remains open

## Persistence

SESS stores state locally in SQLite at:

```text
~/.sess-cli/sess.db
```

Main persisted data:

- tracked projects and base branches
- session state and elapsed time
- linked issue metadata
- PR number and PR URL for ended sessions

## Design Constraints

The current design prefers:

- explicit state transitions over implicit magic
- shelling out to `git` and `gh` instead of reimplementing those tools
- local persistence instead of a remote service
- failure-safe workflow handling over aggressive automation

## Key Architectural Decisions

### Use SQLite for local state

Why:

- simple deployment
- no daemon or background service
- enough structure for projects, sessions, and history

### Keep git and GitHub integration as wrappers

Why:

- lets SESS reuse standard tooling users already trust
- keeps behavior close to what users would run manually
- reduces the amount of custom protocol logic in the codebase

### Keep UI orchestration in `internal/tui`

Why:

- interactive session flows are stateful and easier to express in Bubble Tea
- command files stay thin
- lifecycle rules can still live below the TUI in `internal/session`

## Current Gaps

The main architectural follow-up after `v0.3.0` is conflict and interrupted-workflow recovery for `sess end`.

See:

- [Next steps](NEXT-STEPS.md)
- [Implementation plan](IMPLEMENTATION-PLAN.md)
