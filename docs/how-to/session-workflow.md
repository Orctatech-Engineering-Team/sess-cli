# Run the Session Workflow

Use this guide when you already know what SESS is and want the quickest path through the normal workflow.

## Start a Session From an Issue

Run:

```bash
sess start
```

Then:

1. choose `Select issue`
2. pick the issue from the list
3. choose the branch type

SESS uses the issue title as the initial branch-name input and creates a branch such as `feature/my-change`.

## Start a Session Without an Issue

Run either:

```bash
sess start
```

and choose `Start without issue`, or:

```bash
sess start my-feature
```

Then choose the branch type.

## Start When the Repository Is Dirty

If the working tree is dirty, SESS asks what to do before it creates the session branch.

Available actions:

- `Stash changes`
- `Commit changes`
- `Discard changes`
- `Quit`

Choose `Discard changes` only when you want to remove tracked and untracked changes from the working tree.

## Track an Existing Local Branch

Inspect the branch and current session first:

```bash
git status
sess status
sess start --adopt feature/my-change
```

Adoption starts a new session without issue metadata. It preserves the branch's
commits and working tree, and does not fetch, pull, reset, or recreate the branch.
If you are on a different dirty branch, switch manually before adopting.

To select issue context instead, run the usual interactive start flow with the
same issue/name and branch type, then choose `Track existing branch`.
The branch must exist locally and must not be the project's base branch.
An existing active or paused session must be handled first.

## Check the Current Session

Run:

```bash
sess status
```

Use this when you need to confirm:

- the active branch
- whether the session is active or paused
- elapsed time
- linked issue information

## Pause and Resume Safely

Pause:

```bash
sess pause
```

Resume:

```bash
sess resume
```

When resuming, SESS checks out the saved session branch first if necessary.
If branch checkout fails, the session stays paused.

## End a Session With Dirty Changes

Run:

```bash
sess end
```

If the working tree is dirty, SESS asks for a commit message and commits the work before continuing.

After that, SESS:

- fetches the tracked base branch from `origin`
- rebases onto `origin/<base-branch>`
- pushes the session branch
- creates or reuses a PR
- switches back to the base branch
- ends the session in the local database

## End a Session With No Shippable Work

If there are no uncommitted changes and the branch is not ahead of the base branch, `sess end` offers:

- `End session anyway`
- `Cancel`

Choose `End session anyway` when you want to close out the session without creating a PR.

## Reuse an Existing PR

If an open PR already exists for the session branch, `sess end` reuses it instead of creating another one.

SESS skips the PR summary, testing, and notes prompts for an existing open PR.
You still go through the normal branch alignment, rebase, push, and session
completion flow.

## Recover From an Interrupted End

If a rebase conflicts, the session stays open. Resolve the files, stage them with
`git add <resolved-files>`, and run `git rebase --continue`. Repeat until Git
finishes the rebase, then rerun `sess end`.

While the rebase is unfinished, `sess end` stops before branch checkout or
committing. To abandon the rebase instead, run `git rebase --abort`; this leaves
the session open.

For push, GitHub, or checkout failures, fix the reported problem and rerun
`sess end`. SESS restores saved commit/PR details and resumes from its saved
checkpoint. Completed phases are not repeated.

A matching open PR is reused. A matching merged PR proceeds to local cleanup.
A closed, unmerged PR leaves the session open; inspect it and reopen it before
retrying. Authentication errors, uncertain PR identity, changed origin URLs, and
unexpected branch history also stop recovery without ending the session.

If checkout succeeded but saving completion failed, rerun `sess end` from the
saved base branch. SESS saves completion and removes the checkpoint together.
If that save fails, both stay unchanged.

See [Troubleshoot common problems](troubleshooting.md) for recovery details.

## Keep or Delete the Local Branch

At the end of a successful `sess end` run, SESS asks whether to:

- keep the local session branch
- delete the local session branch

Branch deletion is a cleanup step after the session is already ended.

## See All Tracked Projects

Run:

```bash
sess projects
```

This shows:

- tracked repositories
- whether each is idle, active, or paused
- the active branch when present
- recent activity timing

## Inspect a Session's Lifecycle

Find its ID in the history list:

```bash
sess history
# Or list sessions across tracked projects:
sess history --all
```

Then inspect that ID:

```bash
sess history 42
```

The detail view shows successful lifecycle events and recorded active intervals,
including an ongoing interval when applicable. Timestamps are UTC. The ID lookup
also works outside the project directory.

Older sessions keep their tracked totals but may have no recorded lifecycle
history. A session that started before journaling was available has partial
history even if subsequent pauses, resumes, or completion are recorded.
