# AGENTS.md

## Purpose

This repository is `SESS`, a Go CLI for session-based Git workflows tied to GitHub issues. The current implemented command surface is:

- `sess start`
- `sess status`
- `sess pause`
- `sess resume`
- `sess projects`

The full `sess end` workflow is planned but not complete.

## Repo Map

- `cmd/sess/`: binary entrypoint
- `internal/sess/`: Cobra/Fang command layer
- `internal/tui/`: Bubble Tea workflow and interactive UI
- `internal/session/`: session lifecycle business logic
- `internal/db/`: SQLite persistence
- `internal/git/`: wrappers around `git` and `gh`
- `docs/`: architecture, development, roadmap, and implementation planning

## How To Work In This Repo

Prefer small, surgical changes. Preserve the current layering:

- `cmd/sess` may depend on `internal/sess`
- `internal/sess` may orchestrate `internal/tui`, `internal/session`, and `internal/db`
- `internal/tui` may call into `internal/session`, `internal/db`, and `internal/git`
- `internal/git` and `internal/db` must stay low-level and not depend on UI packages

Do not introduce reverse dependencies between these layers.

## Go Conventions

Follow idiomatic Go. In practice:

- Prefer simple, direct control flow over abstraction-heavy patterns
- Wrap errors with context using `fmt.Errorf("...: %w", err)`
- Do not ignore errors unless the operation is explicitly best-effort
- Keep zero values useful where possible
- Accept interfaces when they help call sites; return concrete types
- Keep package APIs narrow and obvious

## CLI and UX Rules

User-facing command output should follow [docs/cli_design_guide.md](docs/cli_design_guide.md):

- no emojis
- compact, git-like output
- use `·` separators for dense inline information
- prefer direct wording over tutorial-style copy

If changing status/action output, keep it consistent across `start`, `status`, `pause`, `resume`, and `projects`.

## Development Commands

Common local commands:

```bash
go build -o sess ./cmd/sess
./sess --help
```

If working in a sandboxed environment where Go cannot write to the default cache, use:

```bash
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...
```

Note: this environment may still block network access for downloading modules.

## Testing Expectations

There are currently no Go test files in the repo. When fixing lifecycle or persistence logic, add focused tests where practical, especially around:

- session state transitions
- elapsed time accounting
- DB reads/writes for active and paused sessions
- branch and repo state validation

## Known High-Priority Work

Current implementation priorities are tracked in [docs/IMPLEMENTATION-PLAN.md](docs/IMPLEMENTATION-PLAN.md). The highest-value fixes are:

1. Make `sess resume` fail closed if branch checkout fails
2. Respect the tracked project's base branch instead of hardcoding `dev`
3. Validate git repo state before tracking a project
4. Make dirty-repo discard behavior match what the prompt says
5. Use GitHub issue numbers instead of opaque IDs

## Important Behavioral Notes

- Project/session state is stored in SQLite at `~/.sess-cli/sess.db`
- `base_branch` exists in the data model, but parts of the current workflow still hardcode `dev`
- `gh` integration is used for issue selection
- The current codebase is functional but still in active refinement; prefer correctness over feature expansion unless the task explicitly asks for new functionality
