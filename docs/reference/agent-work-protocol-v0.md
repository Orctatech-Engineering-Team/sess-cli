# Agent Work Protocol v0

**Status:** Draft for review  
**Issue:** [#10](https://github.com/Orctatech-Engineering-Team/sess-cli/issues/10)

This document defines a small, local-first protocol for humans and coding agents
working in the same SESS session. It defines data contracts only. It does not
authorize CLI commands, database migrations, or agent automation.

## Design goals

Protocol v0 must:

- let a participant understand current work without reading a full transcript
- preserve bounded attempts, validation evidence, and handoffs across processes
- work for humans, agents, scripts, and different model providers
- identify stale evidence instead of presenting it as current
- retain useful facts without storing private reasoning or raw transcripts
- add structure without locking the repository or assigning work automatically

Protocol v0 does not coordinate agent execution, resolve conflicting edits,
approve releases, or synchronize records between machines.

## Common encoding rules

All protocol documents use these rules:

- `protocol_version` is the string `sess.agent-work/v0`
- IDs are opaque, non-empty strings; consumers must not parse their format
- timestamps use UTC RFC 3339 with fractional seconds
- omitted optional fields and explicit `null` have the same meaning
- unknown JSON fields must be ignored
- unknown enum values must fail closed for writes and remain visible for reads
- arrays preserve recorded order unless a field states another ordering rule
- human-readable text is UTF-8 and must be bounded by the limits below

Every top-level document uses an envelope:

```json
{
  "protocol_version": "sess.agent-work/v0",
  "kind": "context",
  "generated_at": "2026-10-18T10:30:00.123456789Z",
  "data": {}
}
```

`kind` is one of `context`, `attempt`, `check`, or `handoff`.
For context, `generated_at` is the snapshot time. For persisted records, it is
the record's last durable transition time, so serializing the same database
state produces the same value.

## Repository state

Checks and handoffs need more than a branch name. Protocol v0 uses this shared
repository-state object:

```json
{
  "repository": "github.com/Orctatech-Engineering-Team/sess-cli",
  "branch": "feature/agent-context",
  "head_oid": "0123456789abcdef0123456789abcdef01234567",
  "worktree": "clean",
  "captured_at": "2026-10-18T10:30:00.123456789Z"
}
```

Fields:

- `repository` is the canonical host/owner/name when known; otherwise `null`
- `branch` is the exact local branch, or `null` for detached HEAD
- `head_oid` is the full commit object ID
- `worktree` is `clean`, `changed`, or `unknown`
- `captured_at` is when SESS observed this state

Protocol v0 does not hash uncommitted file contents. Evidence recorded against a
`changed` or `unknown` worktree is marked non-reproducible. A later clean state
at the same `head_oid` does not make that earlier evidence current.

## Context

Context is a derived, read-only snapshot. It is not persisted as an independent
historical record.

```json
{
  "protocol_version": "sess.agent-work/v0",
  "kind": "context",
  "generated_at": "2026-10-18T10:30:00.123456789Z",
  "data": {
    "availability": "session",
    "project": {
      "id": "project:7",
      "name": "sess-cli",
      "path": "/work/sess-cli",
      "base_branch": "main"
    },
    "session": {
      "id": "session:42",
      "state": "active",
      "branch": "feature/agent-context",
      "issue": {
        "id": "10",
        "title": "Design Agent Work Protocol v0"
      }
    },
    "repository_state": {
      "repository": "github.com/Orctatech-Engineering-Team/sess-cli",
      "branch": "feature/agent-context",
      "head_oid": "0123456789abcdef0123456789abcdef01234567",
      "worktree": "clean",
      "captured_at": "2026-10-18T10:30:00.123456789Z"
    },
    "active_attempts": [],
    "latest_handoff": null,
    "checks": [],
    "warnings": []
  }
}
```

`availability` is:

- `no_repository`: the current directory is not inside a Git repository
- `untracked_project`: Git is available but SESS does not track the repository
- `idle_project`: the project is tracked but has no active or paused session
- `session`: an active or paused session exists

Unavailable objects are `null`, not empty objects. `warnings` reports facts such
as partial legacy history, detached HEAD, branch mismatch, stale handoff, dirty
evidence, and unknown enum values.

Context reads local state only. It performs no fetch, push, GitHub lookup,
session transition, timer sync, attempt write, or worktree mutation.

## Attempt

An attempt is one participant's bounded contribution to one session.

```json
{
  "protocol_version": "sess.agent-work/v0",
  "kind": "attempt",
  "generated_at": "2026-10-18T10:35:00Z",
  "data": {
    "id": "attempt:01",
    "session_id": "session:42",
    "participant": {
      "id": "participant:7c553bb2-8b35-4fb9-b8cb-209a9d1f8ae1",
      "kind": "agent",
      "label": "reviewer",
      "identity_source": "configured"
    },
    "role": "review",
    "state": "active",
    "started_at": "2026-10-18T10:35:00Z",
    "updated_at": "2026-10-18T10:35:00Z",
    "finished_at": null,
    "start_repository_state": {
      "repository": "github.com/Orctatech-Engineering-Team/sess-cli",
      "branch": "feature/agent-context",
      "head_oid": "0123456789abcdef0123456789abcdef01234567",
      "worktree": "clean",
      "captured_at": "2026-10-18T10:35:00Z"
    },
    "result_repository_state": null,
    "claims": [
      {
        "type": "path",
        "value": "docs/reference/**"
      }
    ],
    "blocked_reason": null
  }
}
```

### Participant identity

A participant ID provides stable attribution across attempts. It is analogous
to Git author identity: useful attribution, but not authentication.

Participant fields are:

- `id`: a stable opaque ID, unique within SESS records
- `kind`: `human`, `agent`, or `automation`
- `label`: optional user-supplied display text
- `identity_source`: `configured`, `orchestrator`, or `ephemeral`

A configured human or agent profile reuses its participant ID across attempts.
An agent orchestrator may supply a stable ID for a named agent profile. When no
stable identity is available, SESS may create an ephemeral participant ID that
is valid for one attempt and records `identity_source: ephemeral`.

Attempt IDs still identify individual runs. A participant may therefore own
many attempts, while every attempt has its own ID.

Protocol v0 does not require an email address and does not derive identity from
Git configuration. It stores no provider account, prompt, conversation ID, or
model identifier by default. Future designs may associate external identities
or signatures with a participant ID without changing historical attempt IDs.

Participant IDs are self-asserted. They do not prove that a human, model, or
orchestrator is who it claims to be. Interfaces must not display them as verified
identity.

Attempt `role` is `implementation`, `review`, `research`, `release`, or `other`.
Role describes the attempt's purpose; it grants no permission.

Attempt `state` transitions are:

```text
active  → blocked | completed | cancelled | failed
blocked → active  | completed | cancelled | failed
```

`completed`, `cancelled`, and `failed` are terminal. A correction creates a new
attempt; it does not reopen or rewrite the old attempt.

`finished_at` and `result_repository_state` are required for terminal states.
`blocked_reason` is required only while blocked and is cleared on resume.
`updated_at` changes on a valid state transition or claim update.

Claims are advisory:

- `type` is `path`, `workflow`, or `other`
- `value` is bounded text interpreted literally by humans
- path values may use Git-style glob notation for display and overlap warnings
- overlapping claims produce warnings but never block an attempt or file edit
- claims expire when the attempt reaches a terminal state
- an abandoned active claim is stale after 24 hours without an attempt update

The 24-hour stale threshold is a v0 default, not proof that an attempt stopped.

## Check

A check records validation evidence from an attempt.

```json
{
  "protocol_version": "sess.agent-work/v0",
  "kind": "check",
  "generated_at": "2026-10-18T10:50:12Z",
  "data": {
    "id": "check:01",
    "attempt_id": "attempt:01",
    "name": "Go test suite",
    "procedure": {
      "type": "command",
      "value": "go test ./..."
    },
    "outcome": "passed",
    "started_at": "2026-10-18T10:49:30Z",
    "finished_at": "2026-10-18T10:50:12Z",
    "exit_code": 0,
    "repository_state": {
      "repository": "github.com/Orctatech-Engineering-Team/sess-cli",
      "branch": "feature/agent-context",
      "head_oid": "fedcba9876543210fedcba9876543210fedcba98",
      "worktree": "clean",
      "captured_at": "2026-10-18T10:50:12Z"
    },
    "reproducibility": "revision",
    "summary": "All packages passed.",
    "output_reference": null
  }
}
```

`procedure.type` is `command` or `manual`. A manual check names the procedure
performed but does not pretend a command ran.

`outcome` is `passed`, `failed`, `cancelled`, or `unavailable`.

`reproducibility` is:

- `revision`: the check ran against a clean worktree at the recorded commit
- `workspace`: the check ran with uncommitted changes and applies only to that
  observed workspace
- `unknown`: SESS could not establish repository state

Checks are immutable. A retry creates a new check. Attempt state transitions may
update the attempt record, but they cannot mutate checks or handoffs.

Protocol v0 stores no raw stdout or stderr by default. `summary` is optional,
explicitly supplied, and limited to 4 KiB. `procedure.value` is limited to
2 KiB. `output_reference` is reserved for a future local artifact design and is
`null` in v0.

SESS must reject summaries containing a NUL byte. It cannot guarantee automatic
secret detection. Callers must redact secrets before recording text.

## Handoff

A handoff records what the next participant needs to continue.

```json
{
  "protocol_version": "sess.agent-work/v0",
  "kind": "handoff",
  "generated_at": "2026-10-18T11:00:00Z",
  "data": {
    "id": "handoff:01",
    "session_id": "session:42",
    "attempt_id": "attempt:01",
    "created_at": "2026-10-18T11:00:00Z",
    "repository_state": {
      "repository": "github.com/Orctatech-Engineering-Team/sess-cli",
      "branch": "feature/agent-context",
      "head_oid": "fedcba9876543210fedcba9876543210fedcba98",
      "worktree": "clean",
      "captured_at": "2026-10-18T11:00:00Z"
    },
    "completed": [
      "Defined Protocol v0 record boundaries and state transitions."
    ],
    "remaining": [
      "Review privacy and retention choices."
    ],
    "risks": [
      "Workspace checks are not reproducible from a commit alone."
    ],
    "next_action": "Review the draft with a human maintainer.",
    "check_ids": [
      "check:01"
    ],
    "supersedes_id": null
  }
}
```

`completed`, `remaining`, and `risks` contain bounded entries rather than one
unstructured transcript. Each array has at most 20 entries; each entry is at
most 1 KiB. `next_action` is at most 2 KiB.

A handoff may be created from an active, blocked, or terminal attempt. It does
not finish the attempt or change session state.

Corrections are append-only. A corrected handoff sets `supersedes_id` to an
earlier handoff from the same session. Cycles and cross-session supersession are
invalid.

The latest valid handoff is the newest handoff by `(created_at, id)` that is not
superseded by another valid handoff. Context marks it stale when its repository
state differs from current state. Stale handoffs remain visible and are never
silently discarded.

## Retention and deletion

Protocol records remain local in the SESS database and follow their session's
retention. Protocol v0 adds no automatic deletion or remote synchronization.

Deleting a project or session is a separate product decision and must state what
happens to protocol records before implementation. Until then, no new command
may delete attempts, checks, or handoffs.

## Threat and privacy review

Protocol data can expose file paths, commands, issue titles, branch names, and
human-written summaries. Implementations must:

- never record environment variables, command output, prompts, or transcripts by
  default
- treat all stored text as untrusted when rendering
- use structured process execution rather than shell interpolation
- restrict text sizes before database writes
- keep JSON generation deterministic and escaped by the standard encoder
- avoid network access while creating or reading local protocol records
- avoid claiming that a participant label is authenticated identity
- make stale and uncertain data visible

The protocol does not provide authorization. Local database access implies the
same trust boundary as existing SESS session data.

## Compatibility with existing sessions

Existing sessions have no attempts, checks, or handoffs. Context returns empty
arrays, `latest_handoff: null`, and a warning:

```json
{
  "code": "protocol_history_unavailable",
  "message": "This session predates Agent Work Protocol records."
}
```

No events are backfilled or inferred. Existing session state and elapsed totals
remain unchanged.

## Scenarios

### Human-only session

1. A human starts a SESS session.
2. Context reports the session with no protocol history.
3. The human opens an implementation attempt and claims `internal/db/**`.
4. The human records a passing test check at a clean commit.
5. The human completes the attempt and records a handoff.

The protocol adds continuity without requiring an agent.

### Agent-only continuation

1. An agent reads context for an existing session.
2. It opens an implementation attempt with a narrow advisory claim.
3. It records checks against the resulting revision.
4. It records remaining risks in a handoff and completes its attempt.
5. A replacement agent receives the latest handoff and stale-state warning if
   the branch changed afterward.

The replacement does not need the first agent's full transcript.

### Mixed implementation and review

1. An agent completes an implementation attempt and records its checks.
2. A human or different agent opens a review attempt.
3. The review attempt may cite implementation checks but does not inherit review
   approval from them.
4. Review findings produce a handoff back to implementation.
5. A later independent review attempt records its own checks against the revised
   commit.

The record distinguishes implementation evidence from independent review.

## Decisions requiring approval

The maintainer approved these choices on 2026-10-18:

1. Context is derived and read-only, not a persisted record.
2. Attempts always belong to exactly one session.
3. Participant identity is stable attribution, not authentication. It uses an
   opaque participant ID, kind, optional label, and identity source.
4. Concurrent attempts and overlapping claims are allowed with warnings.
5. Checks store structured outcomes and a short redacted summary, not raw output.
6. Checks against dirty worktrees are explicitly non-reproducible.

These choices still require approval:

7. Handoffs are append-only and corrected through `supersedes_id`.
8. Protocol records have no automatic deletion in v0.

Implementation issues remain blocked until the pending choices and the complete
Protocol v0 draft are approved.
