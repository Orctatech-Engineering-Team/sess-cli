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
```

### Behavior

- fails if the current directory is not a git repository
- fails if the project already has an active or paused session
- optionally loads GitHub issues through `gh`
- prompts for dirty working tree handling before branch creation
- creates a new active session in the local SQLite database

## `sess status`

Shows the current session status for the tracked project in the current directory.

### Usage

```bash
sess status
```

### Behavior

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

## Root Command

Run:

```bash
sess --help
sess --version
```

The version output is formatted as:

- tagged release, for example `SESS v0.3.0`
- development build, for example `SESS dev-abc1234`
