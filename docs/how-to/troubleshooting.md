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

While a rebase is unfinished, `sess end` stops before switching branches,
prompting, or committing. It prints recovery instructions instead.

To continue:

```bash
git status
# Edit the conflicted files, then stage only the files you resolved.
git add <resolved-files>
git rebase --continue
```

Repeat resolution and `git rebase --continue` if more conflicts appear. Only
after Git finishes the rebase, rerun `sess end`.

To cancel the rebase instead:

```bash
git rebase --abort
```

Aborting restores the branch to its pre-rebase state; it does not end the session.
Rerunning `sess end` retries the rebase and may encounter the same conflict.
SESS does not resolve conflicts or run `--continue` or `--abort` for you. The
session's active or paused state is unchanged; use `sess pause` if you want to
stop its timer while resolving conflicts.

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

SESS does not force-push. If the remote rejects a rewritten branch, inspect the
local and remote history and reconcile it manually before retrying.

## A PR Exists but the Session Is Still Open

A PR can be created before a later checkout or database operation fails.

Fix the reported local problem, then rerun `sess end`. If the PR is still open,
SESS verifies and reuses it without asking for its description again. A saved
push or PR checkpoint skips earlier commit, fetch, rebase, and push phases.

If the matching PR was merged, SESS finishes local cleanup without pushing
again. If it was closed without merging, inspect it and reopen it before
retrying. Missing or ambiguous PR metadata and GitHub access errors leave the
session and checkpoint open.

SESS binds recovery to the saved origin repository and branch head. Restore an
unexpectedly changed origin URL before retrying. Do not add or rewrite session
commits during recovery: unrecognized history changes fail closed. An interrupted
commit must match the saved message and parent; a completed manual rebase must
retain its matching branch reflog entry. There is no automatic checkpoint reset.

Recovery currently requires a single origin URL shared by fetch and push.
Fork-based handoffs and separate push destinations are not supported by this
checkpoint flow.

If only database completion failed, fix the storage problem and rerun from the
saved base branch. The session update, project activity update, and checkpoint
removal are saved together. If the save fails, all three stay unchanged.

## End Workflow Interrupted From the Terminal

Quitting the running end UI cancels SESS's direct Git or `gh` subprocess and waits
for the workflow to stop before returning. Check `git status` and `sess status`
before retrying: earlier commits, pushes, or PR creation may already have succeeded.
Hooks or SSH descendants are not terminated as a process group and may require
separate cleanup.

## The Working Tree Prompt Appears During `sess start`

This means the repository is dirty before branching.

Choose the option that matches your goal:

- stash your changes
- commit them
- discard them
- quit and handle the repository manually

## Branch Created but Session Start Failed

Git branch creation can succeed before SQLite rejects the session write.
SESS returns an error rather than reporting a successful start. It leaves the
branch, commits, and working tree intact; no partial session or start event is
saved.

Inspect both states before retrying:

```bash
git status
sess status
```

If an active or paused session exists, handle that session first. Otherwise fix
the reported storage problem, then repeat `sess start` with the same issue/name
and type and choose `Track existing branch`. Issue context must be selected
again; it is not saved across a failed start.

Alternatively, track the exact local branch without issue metadata:

```bash
sess start --adopt feature/my-change
```

Adoption preserves staged, unstaged, and untracked changes on that branch.
It does not pull, fetch, reset, or create branches. If another branch is dirty,
switch manually before adopting. Finish or abort any rebase with Git first.

Time tracking begins when the retry successfully saves the session, not when
the earlier attempt created the branch.

Quitting the running start UI cancels its workflow and waits for it to stop.
Inspect `sess status` and Git state before retrying: cancellation can occur
after an operation has already succeeded. Hooks and SSH descendants may
require separate cleanup.

## Where SESS Stores State

SESS stores local state in:

```text
~/.sess-cli/sess.db
```

If you are debugging state, this is the database to inspect.

See [Reference: Session model](../reference/session-model.md) for details.
