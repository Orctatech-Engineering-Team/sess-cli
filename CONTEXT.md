# SESS Work Sessions

SESS tracks units of development work tied to repository branches and GitHub
issues, including their lifecycle and handoff.

## Language

**Project**:
A tracked repository with a base branch and its own session history.

**Session**:
A tracked unit of work on a project branch, optionally linked to an issue.

**Lifecycle event**:
A recorded fact that a session successfully started, paused, resumed, or ended.
_Avoid_: Command attempt, workflow step

**Lifecycle journal**:
The ordered record of a session's lifecycle events, which may cover only part
of an older session.

**Active interval**:
A period between an observed start or resume and the following pause or end,
or an ongoing period for a currently active session.
_Avoid_: Focus block

**Tracked active time**:
Time accumulated while a session is active, not evidence of uninterrupted
attention or unique working time across overlapping sessions.
_Avoid_: Focus time, productive time

**End checkpoint**:
Saved progress through a session's Git and pull-request handoff, retained until
session completion succeeds.
_Avoid_: Lifecycle event, ended session
