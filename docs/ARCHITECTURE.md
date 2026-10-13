# Architecture

This document describes the current SESS architecture, including guarded
end-workflow recovery.

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
   - check out the tracked base branch without overwriting ignored files
   - fetch `origin/<baseBranch>` and fast-forward without overwriting ignored files
   - create the new session branch
8. Verify the current branch and head, then atomically persist the active session,
   start event, and project activity before reporting success.

An existing exact local branch may be adopted with explicit interactive consent
or `sess start --adopt <branch>`. Adoption skips base updates and branch creation.
Persistence failure leaves Git state intact for retry; no pending-start checkpoint
is stored. Quitting the running workflow cancels it and waits before returning.

### `sess resume`

1. Command loads the tracked project and paused session.
2. Git wrapper checks the current branch.
3. If needed, SESS checks out the saved session branch.
4. Session manager resumes time tracking only after branch alignment succeeds.

### `sess end`

1. Command loads the tracked project and active or paused session.
2. TUI rejects an unfinished rebase before aligning to the saved session branch.
3. TUI checks whether there is shippable work.
4. If dirty, SESS prompts for a commit message. PR details are prompted only when
   no open PR exists for the session branch.
5. SESS fetches, rebases, and pushes the branch.
6. SESS reuses an existing PR or creates a new one through `gh`.
7. SESS checks out the project base branch.
8. Session manager marks the session ended and stores PR metadata.

Failure rule:

- if an end-workflow step fails before session completion, the session remains open
- an unfinished rebase is resolved with Git before retrying `sess end`
- exiting the end workflow UI cancels subprocess work and waits for it to stop

The end executor checks rebase state again before committing, and commits dirty
work before fetching. PR lookup is repeated after push, so a retry can discover
a PR created by an earlier attempt. End progress and prompt inputs are saved
before continuing to later side effects.

## Persistence

SESS stores state locally in SQLite at:

```text
~/.sess-cli/sess.db
```

Main persisted data:

- tracked projects and base branches
- session state and elapsed time
- successful lifecycle transitions
- linked issue metadata
- PR number and PR URL for ended sessions
- interrupted end-workflow progress and saved prompt inputs

The current schema includes:

- `projects`
- `sessions`
- `session_events`
- `end_workflows`

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

## Lifecycle History

`sessions` remains the current-state and elapsed-total authority. An append-only
`session_events` journal records successful starts, pauses, resumes, and ends.
Lifecycle writes commit the session, event, and project activity together;
completion also removes its end checkpoint. Guarded timer syncs emit no event.

`internal/session` derives a detail timeline from a consistent snapshot of the
session and ID-ordered events. Only observed start/resume events anchor active
intervals. Old sessions are not backfilled, and incomplete history is explicit.
This is a lifecycle journal, not event sourcing or focus telemetry.

## End Recovery Limits

End recovery uses an `end_workflows` SQLite checkpoint keyed by session ID.
It records immutable prompt inputs and repository identity, completed phases,
the expected branch head, rebase intent, and discovered PR identity.
The TUI reconciles Git and GitHub state before continuing side effects.
Session completion, project activity, and checkpoint deletion share a transaction.

Recovery supports matching open and merged PRs, but stops for closed-unmerged
PRs or uncertain identity. Conflicts remain manually resolved. Fork-based PRs,
different origin fetch/push destinations, and automatic checkpoint reset are
not supported.

See:

- [Next steps](NEXT-STEPS.md)
- [Implementation plan](IMPLEMENTATION-PLAN.md)
