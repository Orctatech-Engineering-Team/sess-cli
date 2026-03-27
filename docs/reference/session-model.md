# Session Model Reference

This page describes the concepts SESS stores and uses internally.

## Project

A project is a tracked git repository.

SESS records:

- repository name
- absolute repository path
- base branch
- created time
- last used time

Projects are stored globally so SESS can show activity across repositories.

## Session

A session is a tracked unit of work inside a project.

SESS records:

- branch name
- branch type
- linked issue ID
- linked issue title
- state
- start time
- pause time
- end time
- accumulated elapsed time
- current active slice start
- PR number for ended sessions when applicable
- PR URL for ended sessions when applicable

## Session States

### Idle

No active or paused session exists for the tracked project.

### Active

The session is currently running and elapsed time is still accumulating.

### Paused

The session remains tracked, but active time accumulation is stopped.

### Ended

The session is complete.
It no longer appears as the project’s active session, but it remains in session history.

## Time Tracking Model

SESS does not rely on one continuous start-to-end duration.

Instead, it stores:

- `total_elapsed` for all completed active slices
- `current_slice_start` for the slice currently in progress

This allows pause and resume to preserve timing accurately.

## Base Branch

Each tracked project has a base branch.

SESS uses it when:

- creating a new session branch during `sess start`
- rebasing during `sess end`
- switching back after a successful session end

If no base branch is already stored, the default is `dev`.

## GitHub Metadata

Issue metadata is stored with the session:

- issue number as text
- issue title

For ended sessions, PR metadata may also be stored:

- PR number
- PR URL

## Storage Location

SESS stores project and session state in SQLite at:

```text
~/.sess-cli/sess.db
```

## Active Session Rules

For a given project:

- only one active or paused session may exist at a time
- a new session cannot start while another session is active or paused
- an ended session remains in history but no longer blocks new sessions
