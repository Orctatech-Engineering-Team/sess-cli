# How SESS Works

SESS treats development work as a session rather than as a loose sequence of git commands.

That difference is the core of the tool.

## The Mental Model

A session is a bounded unit of work:

- it starts from a repository and optional issue context
- it runs on a dedicated branch
- it can be paused and resumed
- it ends with a handoff step that returns you to the base branch

SESS makes that lifecycle explicit.

## Why Use Sessions Instead of Just Branches

Branches describe where code lives.
Sessions describe what you are doing right now.

A branch alone does not tell you:

- whether the work is active or paused
- how long you have been in the task
- whether the branch is the current tracked context for the repository
- whether the end-of-session handoff already happened

SESS adds that missing layer.

## What SESS Automates

SESS automates the repeated workflow around a session:

- branch creation from the project base branch
- issue lookup through `gh`
- session state persistence
- elapsed-time tracking
- branch alignment before resume or end
- end-of-session rebase, push, and PR handling

The goal is to reduce friction without hiding the repository state from you.

## What SESS Leaves Explicit

SESS does not try to replace git.

You still see and control:

- your repository contents
- your commit messages
- your branch names
- your GitHub authentication through `gh`

During `sess end`, SESS also fails closed on important workflow steps.
If the repository cannot be aligned safely, it leaves the session open instead of pretending the handoff succeeded.

## Why `sess end` Returns to the Base Branch

SESS treats session completion as more than “a PR exists.”

For a session to be considered ended:

- the work branch must be pushed
- a PR must be created or reused when work is being shipped
- the shell must be back on the project base branch

That final checkout is part of the workflow contract.
It leaves the repository in a predictable post-session state.

## Why SESS Stores State Locally

SESS keeps project and session state in a local SQLite database.

This gives it a stable memory across commands:

- `sess start` creates the context
- `sess status` reads it
- `sess pause` and `sess resume` update it
- `sess end` completes it
- `sess projects` shows activity across repositories

The database is what makes SESS a session tool rather than a one-shot wrapper around `git checkout`.

## Where SESS Fits

SESS is useful when:

- you work from GitHub issues
- you switch contexts often
- you want branch workflow help without adopting a heavy project-management tool
- you want session state to survive past one terminal command

It is less about replacing your git workflow and more about making that workflow easier to carry consistently.
