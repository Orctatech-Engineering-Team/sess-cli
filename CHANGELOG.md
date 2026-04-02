# Changelog

This changelog tracks user-visible improvements to SESS.

## v0.4.0 - 2026-04-02

SESS now includes a first pass at built-in session analytics, so you can see what you worked on without opening the SQLite database by hand.

### What's new

- Added `sess history` to show recent sessions for the current project.
- Added `sess stats` to show totals such as session count, total time, average duration, and longest session.
- Added `sess report` as a compact summary view that combines stats with recent session activity.
- Added `--all` support to `sess history`, `sess stats`, and `sess report` for cross-project views across every tracked repository on the machine.

### Why it matters

- You can answer simple questions like "what was I working on last week?" or "which repo has consumed most of my time lately?" directly from the CLI.
- Cross-project analytics make SESS more useful when you regularly move between multiple repositories.
- The new report command gives you a fast status summary without stitching together multiple commands.

### Notes

- This release builds on the existing session workflow introduced in earlier versions. It does not change `sess start`, `sess pause`, `sess resume`, or `sess end` behavior.
- Analytics are still local-only and read from the existing SQLite database at `~/.sess-cli/sess.db`.

## v0.3.1

Previous release. See the GitHub release page for details:
https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.3.1
