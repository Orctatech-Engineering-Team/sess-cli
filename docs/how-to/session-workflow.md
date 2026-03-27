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

You still go through the normal branch alignment, push, and session completion flow.

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
