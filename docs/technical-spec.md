# Technical Specification

This document is a historical specification for SESS.

It captures the original intent for the project before the current user docs, architecture docs, and implementation work were finalized.

## Status

Use this document as background context, not as the current source of truth.

For current behavior, prefer:

- [README.md](../README.md)
- [docs/README.md](README.md)
- [docs/reference/commands.md](reference/commands.md)
- [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- [docs/IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md)

## Original Product Direction

SESS was conceived as a CLI for managing focused work sessions tied to GitHub issues directly from the terminal.

The core ideas were:

- treat work as a session rather than a loose series of git commands
- track start, pause, resume, and end states explicitly
- integrate GitHub issue context into the workflow
- reduce the operational overhead around branch and PR handoff

## Concepts That Remain Valid

The following parts of the original spec still match the current product direction:

- session-oriented workflow
- issue-linked start flow
- explicit pause and resume
- end-of-session PR handoff
- local state tracking
- emphasis on clear command naming

## Areas Where Implementation Diverged or Evolved

The current implementation differs from the early spec in several ways:

- SESS uses SQLite at `~/.sess-cli/sess.db` rather than a simpler JSON store
- GitHub integration is handled through `gh`, not a custom GitHub API client
- `sess end` now uses an internal prompt-driven PR body flow rather than relying on a repo PR template
- auth/config commands are still planned rather than implemented
- the base branch is stored per tracked project instead of being only a conceptual default

## Original Command Vision

The original command surface aimed for:

- `sess start`
- `sess pause`
- `sess resume`
- `sess end`
- `sess status`
- auth/config commands

The currently implemented command surface is:

- `sess start`
- `sess status`
- `sess pause`
- `sess resume`
- `sess end`
- `sess projects`

## Why Keep This File

This file is still useful for:

- understanding the original design intent
- comparing planned and implemented behavior
- preserving the early product framing for contributors

It should not be treated as a complete or current implementation reference.
