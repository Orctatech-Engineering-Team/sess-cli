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

## Lifecycle Journal

The session record remains authoritative for current state and elapsed totals.
The append-only `session_events` journal records successful transitions:

- `session_started`
- `session_paused`
- `session_resumed`
- `session_ended`

Each event records its ordered ID, session ID, UTC timestamp, resulting state,
and accumulated elapsed nanoseconds. Event ID determines ordering even when
timestamps are equal.

Session state, its lifecycle event, and project activity commit in one
transaction. Completion also removes the matching end checkpoint in that
transaction. Failed or repeated transitions do not append events. Timer syncs
are accounting updates, not lifecycle transitions.

Existing sessions receive no fabricated events during migration. Their totals
remain available, but detailed history is unavailable until observed transitions
are recorded, and then remains partial if the start was not recorded.

`sess history <session-id>` derives intervals only from recorded start/resume
events and their subsequent pause/end events. An active session may have an
ongoing interval. Missing history is not inferred from aggregate totals.
Backward timestamps mark the timeline incomplete rather than creating negative
interval durations.

Tracked active time is not measured focus time. Different projects can have
overlapping active sessions; their summed durations are not unique working time.

## End Workflow Checkpoint

An interrupted end attempt retains the session's active or paused state.
Its separate `end_workflows` record stores:

- session ID, branch, and saved base branch
- commit message, PR title/body, and no-PR choice
- canonical GitHub repository and an origin URL fingerprint
- expected branch head and immutable rebase target
- discovered PR number and URL
- last completed phase: `prepared`, `committed`, `rebased`, `pushed`,
  `pr_created`, or `base_checked_out`

Prompt inputs and identity remain fixed across retries. Git and GitHub state
must match the checkpoint before recovery continues. The raw origin URL is not
stored in the checkpoint.

Successful completion updates the session and project activity and deletes the
matching checkpoint in one SQLite transaction. A failed transaction retains all
three previous records.

## Base Branch

Each tracked project has a base branch.

SESS uses it when:

- creating a new session branch during `sess start`
- rebasing during `sess end`
- switching back after a successful session end

If no base branch is already stored, SESS detects it from `origin/HEAD` and falls back to the current branch when the remote default branch is unavailable.

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
