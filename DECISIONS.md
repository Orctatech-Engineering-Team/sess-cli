# Decisions

## 2026-10-18 — Model work as resumable sessions

SESS treats a unit of work as an explicit session that can be started, paused,
resumed, and ended. Session state is persisted in SQLite so interruptions do
not erase context.

## 2026-10-18 — Use GitHub issues and pull requests as the handoff boundary

SESS integrates with `gh` for issue selection and ends a completed session by
pushing its branch and creating or reusing a pull request.

## 2026-10-18 — Keep the CLI repository-first

The tool operates on the current Git repository and requires Git and an
authenticated `gh` installation rather than introducing a separate hosted
work-management service.
