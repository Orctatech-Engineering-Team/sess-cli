# Command Reference

This page describes the current SESS command surface.

## `sess start [feature-name]`

Starts a new session in the current repository.

### Purpose

- track the repository as a project if needed
- select issue context or start without one
- choose a branch type
- create the session branch from the tracked base branch

### Usage

```bash
sess start
sess start user-profile-page
sess start --adopt feature/user-profile-page
```

### Behavior

- fails if the current directory is not a git repository
- fails if the project already has an active or paused session
- detects the base branch from `origin/HEAD` and falls back to the current branch when needed
- repairs an invalid stored base branch before branch creation
- optionally loads GitHub issues through `gh`
- prompts for dirty working tree handling before branch creation
- checks out the tracked base branch, fetches it from origin, and updates it fast-forward-only before creating the session branch
- refuses to overwrite ignored files during checkout or base updates
- creates a new active session in the local SQLite database
- reports success only after the session and its start event are saved together
- fails during an unfinished rebase

If the selected branch already exists locally, the interactive flow offers
`Track existing branch` before dirty-worktree handling. This starts a new session
using the selected issue context without recreating or updating the branch.

`--adopt <branch>` selects an exact existing local branch without prompts or issue
metadata. It cannot be combined with a feature name. Adoption does not fetch,
pull, reset, or create a branch; it rejects the project's base branch and
remote-only branches. Uncommitted work is preserved on the selected branch.
If another branch is checked out and dirty, switch manually before adopting.

If session persistence fails after branch creation, the command fails and leaves
the branch intact. Inspect `sess status` before retrying; see
[start recovery](../how-to/troubleshooting.md#branch-created-but-session-start-failed).

## `sess status`

Shows the current session status for the tracked project in the current directory.

### Usage

```bash
sess status
```

### Behavior

- prints a guidance message when the current directory is not a tracked project
- prints idle state when the project is tracked but has no active or paused session
- prints active or paused session details when a session exists
- includes elapsed time and linked issue information when available

## `sess pause`

Pauses the active session for the current tracked project.

### Usage

```bash
sess pause
```

### Behavior

- only works when the project has an active session
- fails if the current directory is not a tracked project
- stops active time accumulation
- preserves the branch and issue context in the database

## `sess resume`

Resumes a paused session.

### Usage

```bash
sess resume
```

### Behavior

- only works when the project has a paused session
- fails if the current directory is not a tracked project
- checks out the saved session branch first if the shell is on a different branch
- fails closed if branch checkout fails
- resumes time tracking only after branch alignment succeeds

## `sess end`

Ends the current session and completes the handoff workflow.

### Usage

```bash
sess end
```

### Behavior

- works for active and paused sessions
- fails if the current directory is not a tracked project
- checks out the saved session branch first when needed
- prompts for a commit message when the working tree is dirty
- checks whether there is shippable work on the branch
- prompts for PR summary, testing, and notes when a PR may be created
- fetches and rebases onto `origin/<base-branch>`
- pushes the session branch to `origin`
- reuses an existing open PR for the branch or creates a new one
- checks out the base branch before marking the session ended
- prompts whether to keep or delete the local session branch

### Failure Semantics

- if checkout, commit, rebase, push, PR lookup, or PR creation fails, the session stays open
- if checkout back to the base branch fails after PR creation, the PR may exist but the session is not marked ended

## `sess projects`

Lists tracked projects across the local system.

### Usage

```bash
sess projects
```

### Behavior

- shows tracked repository path
- shows whether the project is idle, active, or paused
- includes branch, elapsed time, and issue number for active or paused sessions
- marks the current working directory with `*`

## `sess history`

Shows recent session history for the tracked project in the current directory.

### Usage

```bash
sess history
sess history --limit 20
sess history --all
sess history 42
```

### Behavior

- the default list requires the current directory to be a tracked project
- `--all` shows recent sessions across all tracked projects instead of the current directory
- shows the newest sessions first
- includes active, paused, and ended sessions
- shows session IDs, elapsed time, issue metadata, and PR metadata when available
- limits output to the most recent sessions, defaulting to 10
- a positive session ID shows that session's recorded events and active intervals
- detail lookup works outside a tracked project; IDs belong to the local database
- detail mode cannot be combined with `--all` or an explicitly supplied `--limit`
- detail timestamps are exact UTC timestamps; legacy or partial history is labeled

## `sess stats`

Shows aggregate session statistics for the tracked project in the current directory.

### Usage

```bash
sess stats
sess stats --all
```

### Behavior

- fails if the current directory is not a tracked project
- `--all` aggregates across all tracked projects instead of the current directory
- summarizes total sessions and accumulated elapsed time
- includes average session duration and longest recorded session
- counts active, paused, and ended sessions
- counts sessions with linked PR metadata
- reports the first and most recent recorded session start times

## `sess report`

Shows a compact session report with summary metrics and recent sessions.

### Usage

```bash
sess report
sess report --limit 8
sess report --all
```

### Behavior

- fails if the current directory is not a tracked project
- `--all` reports across all tracked projects instead of the current directory
- combines aggregate stats with a recent-session summary
- includes longest-session metadata and recent issue or PR context when available
- limits the embedded recent-session list, defaulting to 5

## Root Command

Run:

```bash
sess --help
sess --version
```

The version output is formatted as:

- tagged release, for example `SESS v0.3.1`
- development build, for example `SESS dev-abc1234`
