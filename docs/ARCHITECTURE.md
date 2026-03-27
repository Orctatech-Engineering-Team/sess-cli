# Architecture

This document describes the current SESS architecture at `v0.3.1`.

SESS is a Go CLI that manages developer work as explicit sessions tied to repositories, branches, optional GitHub issues, and pull requests.

## System Diagram

```text
+---------------------------+         +-----------------------------+
| User shell / working repo |         | GitHub + origin remote      |
| current branch, worktree  |         | issues, PRs, default branch |
+-------------+-------------+         +--------------+--------------+
              |                                          ^
              v                                          |
+-------------+------------------------------------------+---------+
|                           SESS binary                            |
|                                                                  |
|  +------------------+                                            |
|  | cmd/sess         | entrypoint                                 |
|  +--------+---------+                                            |
|           |                                                      |
|  +--------v---------+                                            |
|  | internal/sess    | Cobra/Fang command layer                   |
|  +-----+-------+----+                                            |
|        |       |                                                 |
|        |       +------------------------+                        |
|        v                                v                        |
|  +-----+-----------+          +---------+----------+             |
|  | internal/tui    | <------> | internal/session   |             |
|  | Bubble Tea UX   |          | lifecycle rules    |             |
|  +-----+-------+---+          +---------+----------+             |
|        |       |                         |                        |
|        |       +-------------+           |                        |
|        v                     v           v                        |
|  +-----+----------+   +------+-------+  +----------------------+  |
|  | internal/git   |   | internal/db  |  | SQLite               |  |
|  | git / gh wraps |   | persistence  |  | ~/.sess-cli/sess.db  |  |
|  +-----+----------+   +------+-------+  +----------------------+  |
+--------+----------------------+-----------------------------------+
         |
         +--> `git` and `gh` commands run against the local repo and GitHub
```

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
- `internal/sess`: Cobra/Fang command wiring, current working directory lookup, and database bootstrap
- `internal/tui`: interactive workflows and Bubble Tea models for `start` and `end`
- `internal/session`: lifecycle rules for projects and sessions
- `internal/db`: SQLite schema and persistence
- `internal/git`: wrappers around `git` and `gh`

In practice:

- thin commands in `internal/sess` usually open the database and then hand control to either:
  - a TUI flow in `internal/tui`, or
  - a small command path that uses `internal/session` and `internal/git` directly
- `internal/tui` is orchestration-heavy: it coordinates prompts, git operations, and session persistence
- `internal/session` owns state transitions such as `active -> paused -> active -> ended`

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

A tracked project stores:

- repository path
- base branch
- last-used timestamp

A session stores:

- branch and branch type
- optional issue metadata
- state
- elapsed-time bookkeeping
- optional PR metadata once ended

Session states:

- `active`
- `paused`
- `ended`

Only one active or paused session may exist per project at a time.

## Data Flow

### `sess start`

1. Command resolves the current working directory and opens SQLite.
2. TUI validates that the directory is a git repository.
3. TUI detects the project base branch from `origin/HEAD`, with fallback to the current branch.
4. Session manager initializes or loads the tracked project.
5. If the stored base branch is invalid, the TUI repairs it before continuing.
6. TUI collects issue, branch name, branch type, and dirty-worktree choices.
7. Git wrappers:
   - check out the tracked base branch
   - pull `origin/<baseBranch>`
   - create the new session branch
8. Session manager persists the new active session.

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

The current schema is centered on two tables:

- `projects`
- `sessions`

`sessions.total_elapsed` and `sessions.current_slice_start` are used together so pause and resume can accumulate time accurately across multiple active slices.

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
- lifecycle rules still live below the TUI in `internal/session`
- git and GitHub side effects stay concentrated in one orchestration layer

## Current Gaps

The main architectural follow-up after `v0.3.1` is conflict and interrupted-workflow recovery for `sess end`.

See:

- [Next steps](NEXT-STEPS.md)
- [Implementation plan](IMPLEMENTATION-PLAN.md)
