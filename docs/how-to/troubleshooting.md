# Troubleshoot Common Problems

Use this guide when SESS does not behave the way you expect.

## `not a git repository`

SESS only works inside a git repository.

Check your current directory:

```bash
pwd
git status
```

If this is not a repository yet, initialize one first or move into the correct project directory.

## `not a tracked project. Run 'sess start' first`

SESS only tracks repositories after you use `sess start` there.

Fix:

1. move into the repository you want to track
2. run `sess start`

After that, `sess status`, `sess pause`, `sess resume`, `sess end`, and `sess projects` can use the stored project record.

## `no paused session found`

This means there is no paused session to resume in the current tracked project.

Check:

```bash
sess status
```

If the session is active, you do not need `sess resume`.
If there is no session at all, start one with `sess start`.

## Resume or End Switches Branches First

SESS may check out the saved session branch before `resume` or `end`.

This is intentional.
It prevents the session timer or end-of-session workflow from continuing on the wrong branch.

If checkout fails, fix the git problem first, then rerun the command.

## GitHub Issues Do Not Load

Issue selection depends on `gh`.

Check:

```bash
gh auth status
```

If `gh` is not authenticated, authenticate it first.

You can still start a session without selecting an issue.

## Pull Request Creation Fails During `sess end`

Common reasons:

- `gh` is not authenticated
- the remote branch push failed
- the repository permissions do not allow PR creation

What SESS does:

- it leaves the session open instead of marking it ended
- it prints the failure
- it keeps the pushed branch state intact when push succeeded earlier

Fix the GitHub or remote problem, then rerun:

```bash
sess end
```

## Rebase Fails During `sess end`

SESS fetches and rebases onto the tracked base branch before PR handoff.

If rebase fails:

- the session stays open
- the workflow stops before session completion

You can inspect the repository and either:

- resolve the rebase manually and rerun `sess end`
- or abort the rebase:

```bash
git rebase --abort
```

Conflict-resume support inside SESS itself is still a follow-up item.

## Push Fails During `sess end`

If push fails, SESS does not end the session.

Check:

- remote access
- branch protection or permission issues
- network/auth state

Then rerun:

```bash
sess end
```

## The Working Tree Prompt Appears During `sess start`

This means the repository is dirty before branching.

Choose the option that matches your goal:

- stash your changes
- commit them
- discard them
- quit and handle the repository manually

## Where SESS Stores State

SESS stores local state in:

```text
~/.sess-cli/sess.db
```

If you are debugging state, this is the database to inspect.

See [Reference: Session model](../reference/session-model.md) for details.
