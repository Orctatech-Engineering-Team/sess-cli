# Development Guide

This guide is for contributors working on the SESS codebase.

## Prerequisites

- Go 1.25+
- `git`
- `gh`

Optional but useful:

- `gofmt`
- `golangci-lint`

## Local Setup

```bash
git clone https://github.com/Orctatech-Engineering-Team/sess-cli.git
cd sess-cli
go build -o sess ./cmd/sess
./sess --help
```

In restricted environments, use local caches for test/build commands:

```bash
env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...
```

## Development Workflow

Typical loop:

1. make a focused change
2. run `gofmt`
3. run targeted tests
4. run `go test ./...`
5. verify the CLI in a throwaway repository when behavior changes

## Package Boundaries

Keep the layering stable:

- `cmd/sess` may depend on `internal/sess`
- `internal/sess` may orchestrate `internal/tui`, `internal/session`, and `internal/db`
- `internal/tui` may call into `internal/session`, `internal/db`, and `internal/git`
- `internal/db` and `internal/git` should remain low-level

Avoid reverse dependencies between these layers.

## Working in This Repo

Prefer:

- small, surgical changes
- explicit error handling
- direct control flow
- localized tests near changed code

Do not:

- add unnecessary abstraction
- hide important git or session state transitions
- mutate user data or repo state before validation

## Testing Expectations

Current test coverage is focused on:

- `internal/db`
- `internal/session`
- `internal/sess`
- `internal/tui` helper logic

When changing lifecycle behavior, add coverage for:

- session state transitions
- database reads and writes
- failure semantics
- command-facing helper logic where practical

## Manual Verification

Changes to session workflows should usually be verified in a throwaway repository.

Recommended manual scenarios:

- start in a clean repo
- start in a dirty repo
- pause and resume on another branch
- end a session with dirty changes
- end a session with no shippable work
- end a session that reuses an existing PR
- failure cases for rebase, push, or `gh`

## Important Local Files

- [README.md](../README.md): landing page
- [docs/README.md](README.md): docs index
- [docs/ARCHITECTURE.md](ARCHITECTURE.md): architecture overview
- [docs/cli_design_guide.md](cli_design_guide.md): output rules
- [docs/IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md): current execution backlog
- [docs/NEXT-STEPS.md](NEXT-STEPS.md): active follow-up items

## Release Workflow

Release tags are built through GitHub Actions.

See:

- [docs/RELEASING.md](RELEASING.md)

## Historical Context

Older planning and milestone documents still exist for background:

- [docs/MVP1-SUMMARY.md](MVP1-SUMMARY.md)
- [docs/technical-spec.md](technical-spec.md)

Treat them as historical context, not the current source of truth.
