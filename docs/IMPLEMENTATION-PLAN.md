# SESS CLI Implementation Plan

This document turns the current code review findings into an execution plan. It focuses on fixing correctness issues first, then closing the most important product gaps, then cleaning up quality and maintainability.

## Phase 1: Correctness and Workflow Safety

These items should be done first because they can leave users in the wrong repo state or make core commands fail in common repositories.

### 1. Make `resume` fail closed on branch checkout problems

**Problem**

`sess resume` can mark a session as active even if switching back to the saved branch failed.

**Files**

- `internal/sess/resume.go`
- `internal/git/git.go`

**Implementation**

- Attempt branch checkout before resuming the session timer.
- If checkout fails, return an error and leave the session paused.
- Only skip checkout when the current branch already matches the saved branch.
- Consider surfacing git stderr directly in the command output.

**Acceptance criteria**

- Resuming from a different branch succeeds only after checkout succeeds.
- If checkout fails, the session remains paused in the database.
- User output clearly states what failed and what state the session is in.

### 2. Respect the project base branch instead of hardcoding `dev`

**Problem**

The start workflow always checks out and pulls `dev`, even though `base_branch` exists in the database model.

**Files**

- `internal/tui/start.go`
- `internal/session/session.go`
- `internal/db/db.go`

**Implementation**

- Load the tracked project's `BaseBranch` and use it for checkout and pull.
- Replace the hardcoded `"dev"` in the git workflow orchestration.
- Keep `"dev"` only as a default for first-time initialization until config exists.
- Audit command output so it prints the real base branch.

**Acceptance criteria**

- `sess start` works in repos whose default branch is `main`.
- The branch used for checkout/pull matches `project.BaseBranch`.
- Existing tracked projects still work after the change.

### 3. Validate git repository state before tracking a project

**Problem**

Running `sess start` in a non-repo directory can create a tracked project entry before git validation fails.

**Files**

- `internal/tui/start.go`
- `internal/git/git.go`

**Implementation**

- Check `git rev-parse --git-dir` early in `RunStartTUI`.
- Fail before creating or updating the project record when the current directory is not a git repo.
- Return a clear error message with the expected next step.

**Acceptance criteria**

- `sess start` outside a git repo does not create a project row.
- The command exits with a direct, actionable error.

### 4. Make dirty-repo discard behavior match dirty detection

**Problem**

Dirty detection includes untracked files, but the current discard action only runs `git reset --hard`.

**Files**

- `internal/tui/start.go`
- `internal/git/git.go`

**Implementation**

- Decide on intended semantics:
  - Option A: discard everything, including untracked files.
  - Option B: rename the option to clarify it only resets tracked changes.
- If choosing full discard, add `git clean -fd`.
- Warn clearly before destructive cleanup.

**Acceptance criteria**

- The chosen discard behavior matches what the prompt says.
- The repo is actually clean after selecting discard.

## Phase 2: GitHub and Start-Flow Fidelity

These items improve the quality of the core `start` workflow and reduce user confusion.

### 5. Use GitHub issue numbers, not opaque IDs

**Problem**

Issue selection currently stores and displays GitHub `id`, which is likely the GraphQL node ID rather than the issue number users expect.

**Files**

- `internal/git/gh.go`
- `internal/tui/issue_select.go`
- `internal/tui/start.go`
- `internal/db/db.go`

**Implementation**

- Change `gh issue list --json ...` to request the issue number field.
- Update the `Issue` struct and all UI formatting to use the human issue number.
- Decide whether existing session rows need migration or can remain as legacy values.

**Acceptance criteria**

- The issue picker shows values like `#123`.
- Saved sessions display human-readable issue identifiers.

### 6. Surface git stderr in the start TUI

**Problem**

The streaming helper emits stderr lines, but the start model ignores them.

**Files**

- `internal/tui/common.go`
- `internal/tui/start.go`

**Implementation**

- Handle `gitErrLineMsg` in the start model and render it in the visible log stream.
- Consider prefixing stderr lines so they are distinguishable.

**Acceptance criteria**

- Informational git stderr output is visible during `sess start`.
- Failure output is easier to diagnose without rerunning commands manually.

### 7. Update project activity timestamps consistently

**Problem**

`last_used_at` is updated on session start but not consistently on other meaningful interactions.

**Files**

- `internal/session/session.go`
- `internal/db/db.go`

**Implementation**

- Decide which actions count as project activity: start, pause, resume, end, maybe status.
- Update `last_used_at` consistently through the session manager.
- Keep the update logic centralized to avoid drift.

**Acceptance criteria**

- `sess projects` ordering reflects recent real usage.
- Timestamp updates are consistent across the lifecycle.

## Phase 3: Complete the Session Lifecycle

This is the main product gap after the safety fixes.

### 8. Implement `sess end`

**Problem**

The session manager supports ending a session, but the CLI does not complete the end-to-PR workflow.

**Files**

- `internal/sess/`
- `internal/session/session.go`
- `internal/git/git.go`
- `internal/git/gh.go`
- `internal/tui/`

**Implementation**

- Add a `sess end` command.
- Define the desired flow explicitly:
  - validate repo state
  - commit or prompt for uncommitted changes
  - update from base branch
  - push branch
  - open PR via `gh`
  - switch back to base branch
  - mark session ended
- Decide failure semantics for each step so the DB state always matches the actual repo state.

**Acceptance criteria**

- A normal session can be started, paused/resumed, and ended fully through the CLI.
- The resulting PR targets the configured base branch.
- Session state is correct even if a later step fails.

### 9. Add conflict and interrupted-workflow recovery

**Problem**

Rebase, push, or PR creation failures during `end` need explicit handling instead of partial silent failure.

**Files**

- `internal/sess/`
- `internal/tui/`
- `internal/session/session.go`

**Implementation**

- Define recoverable workflow states.
- Preserve enough state to let the user resolve conflicts and continue.
- Add clear user-facing guidance for recovery paths.

**Acceptance criteria**

- Conflict scenarios do not leave the session state ambiguous.
- The user can recover without manually editing the database.

## Phase 4: Quality and Maintainability

These items reduce long-term risk and clean up obvious rough edges.

### 10. Add tests around session state transitions

**Problem**

The repo currently has no Go test files.

**Files**

- `internal/session/`
- `internal/db/`
- `internal/git/` as needed

**Implementation**

- Start with unit tests for:
  - start
  - pause
  - resume
  - end
  - elapsed time calculation
- Add DB-backed tests for migrations and active-session queries.
- Mock or isolate shelling to git/gh where practical.

**Acceptance criteria**

- Critical session transitions are covered by automated tests.
- Regressions in timing and state updates are caught by CI.

### 11. Remove CLI scaffolding leftovers and tighten polish

**Problem**

There are still template remnants and output inconsistencies.

**Files**

- `internal/sess/root.go`
- `internal/sess/`
- `internal/tui/`

**Implementation**

- Change the root command `Use` string to `sess`.
- Remove the unused `--toggle` flag.
- Align output with the design direction in `docs/cli_design_guide.md`.
- Remove emoji and keep phrasing compact and git-like.

**Acceptance criteria**

- Help output matches the actual binary name.
- No obvious scaffolding remains in user-facing commands.

### 12. Fix version string handling for future pseudo-versions

**Problem**

Version formatting logic only recognizes pseudo-versions containing `-2024` or `-2025`.

**Files**

- `internal/sess/root.go`

**Implementation**

- Replace the year-specific string checks with a general pseudo-version detection approach.
- Keep tagged release output unchanged.

**Acceptance criteria**

- Dev builds continue to render a clean version string in future years.

## Recommended Execution Order

1. Resume safety
2. Base branch support
3. Non-repo validation
4. Dirty discard semantics
5. GitHub issue number fix
6. Git stderr visibility
7. Project activity timestamps
8. `sess end`
9. Conflict recovery
10. Tests
11. CLI polish
12. Version formatting cleanup

## Notes

- Phases 1 and 2 are the highest leverage because they harden the existing command set.
- `sess end` should be built only after the repo-state and branch-state correctness issues are fixed.
- The absence of tests means each phase should add coverage before moving further down the lifecycle.
