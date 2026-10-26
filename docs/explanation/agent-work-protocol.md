# Structure AI agent work with SESS

SESS can give humans and coding agents a shared, durable structure for software
work. The goal is not to record an agent's private reasoning. It is to preserve
the facts another human or agent needs to continue safely.

The north star is:

> **SESS becomes the durable work-session layer shared by humans and coding
> agents.**

## The problem

Coding agents often start from a prompt and repository checkout, then lose useful
context when the run ends. A replacement agent has to reconstruct the objective,
ownership, completed work, validation, and remaining risks from chat history and
Git state.

The same task may also involve several agents. Without a shared protocol, they
can duplicate work, edit the same scope differently, repeat external side
effects, or report completion without enough evidence.

SESS already models part of this problem. A session has an issue, branch, state,
elapsed time, lifecycle history, and a recoverable handoff. These concepts can
support agent work without tying SESS to one model or agent vendor.

## Proposed model

The session remains the durable unit of product work. Humans and agents
participate through attempts:

- **Session**: the issue, branch, objective, and lifecycle of the work
- **Attempt**: one bounded period in which a human or agent works on the session
- **Participant identity**: stable self-asserted attribution across attempts,
  similar to Git author identity but not proof of identity
- **Claim**: an advisory scope an attempt intends to change
- **Check**: recorded validation evidence, including the command and result
- **Handoff**: completed work, remaining work, risks, and the next useful action
- **Checkpoint**: saved progress through external side effects that must not be
  repeated blindly

An attempt does not replace a session. A session may include several attempts,
including implementation, review, and release work by different participants.

```text
issue → session → attempt → changes and checks → handoff
```

## Protocol v0 design

Design the protocol before adding commands or database tables. Protocol v0 has
four records: context, attempt, check, and handoff. The design must define their
meaning, relationships, required fields, lifecycle, and privacy rules.

### Context

Context is a read-only snapshot of the work a participant is about to join. It
answers:

- Which session and project is this?
- What is the objective and linked issue?
- Which branch, base branch, and repository revision are current?
- What state is the session in?
- What was the latest handoff?
- Which checks are required or already known?
- Is any information missing, legacy, or uncertain?

Context is derived from durable records. It is not an editable note and does not
claim that repository state stayed unchanged after the snapshot.

### Attempt

An attempt is one participant's bounded contribution to a session. It records:

- a stable attempt ID and parent session ID
- participant kind and optional participant name
- role, such as implementation, review, or release
- start and finish times
- starting and resulting repository revisions
- state: active, blocked, completed, cancelled, or failed
- optional advisory scope claims

An attempt belongs to exactly one session. Finishing an attempt does not end the
session. A cancelled or failed attempt remains part of the record.

### Check

A check is validation evidence produced during an attempt. It records:

- check ID and parent attempt ID
- command or named procedure
- start and finish times
- outcome: passed, failed, cancelled, or unavailable
- repository revision tested
- a bounded, redacted summary or reference to retained output

A passing check applies only to the recorded revision and environment. It is not
an approval and does not prove that untested behavior works.

### Handoff

A handoff is a participant's durable summary for whoever continues the session.
It records:

- handoff ID, session ID, and source attempt ID
- completed work
- remaining work
- blockers and known risks
- next useful action
- repository revision at handoff
- checks that support the handoff

A session may have several handoffs. New context uses the latest valid handoff
but retains older handoffs as history. A handoff does not silently change
session state or approve a release.

### Protocol invariants

Protocol v0 should preserve these rules:

1. Every attempt belongs to one session.
2. Every check belongs to one attempt and one repository revision.
3. Every handoff identifies its source attempt and repository revision.
4. Context states when source data is missing, stale, or uncertain.
5. Checks are immutable and handoff corrections append new records; attempt
   lifecycle updates cannot rewrite their evidence.
6. Implementation and independent review are different attempt roles.
7. No record contains hidden reasoning, secrets, or complete tool transcripts by
   default.
8. Creating protocol records does not perform Git or GitHub side effects.

### Design deliverables

Before implementation, the protocol design should produce:

- a versioned JSON representation for each record
- example records for human-only, agent-only, and mixed sessions
- state-transition rules for attempts
- redaction, output-size, and retention rules for checks
- rules for selecting the latest valid handoff
- compatibility behavior for existing sessions
- a threat and privacy review
- an implementation sequence made of thin, testable slices

## What this structure provides

### A reliable starting contract

An agent should be able to read machine-friendly context before editing:

```bash
sess context --json
```

The response could include the session identity, objective, branch, base branch,
constraints, active claims, previous handoff, and required checks. This reduces
guessing from chat fragments and stale documentation.

### Advisory ownership

An attempt can claim a narrow scope such as `internal/db` or the release
workflow. Claims should warn about overlap rather than lock the repository.
Humans must remain able to resolve exceptions.

### Durable handoffs

An attempt can record what changed, what remains, known risks, and the next
action. A new participant should not need the full transcript to continue.

```bash
sess handoff create \
  --done "Guarded lifecycle writes against stale state" \
  --next "Expose the recorded intervals in session history" \
  --risk "Older sessions have no event backfill"
```

### Completion backed by evidence

SESS can distinguish implementation from verification:

```bash
sess check run "go test ./..."
sess check list
```

A check should record the command, outcome, time, attempt, and repository
revision. A passing check is evidence about that revision, not a permanent
property of the session.

### Safe retries

Attempts may stop because of cancellation, network failure, or tool crashes.
Recorded checkpoints can prevent a replacement agent from repeating a commit,
push, pull request, migration, or release without checking current state.

### Independent review

Implementation and review should be separate roles. An implementing attempt may
record its checks, but it cannot turn its own claim into independent review
approval.

## Implementation after protocol approval

Do not begin implementation with a general agent orchestration system. After
Protocol v0 is reviewed and accepted, start with a read-only context command:

```bash
sess context --json
```

The first version should:

- read the current project and session without changing repository state
- return a versioned JSON document with stable field names
- include session, issue, branch, base branch, and lifecycle state
- include the latest recorded handoff when that model exists
- explain missing or legacy data explicitly
- work for humans, scripts, and any agent vendor

After the context contract is proven in use, attempts are added as the next thin
end-to-end slice. Checks and handoffs follow separately. Each slice should
implement the approved protocol rather than inventing its contract inside
command code.

## What SESS should not store

SESS should not collect prompts, hidden reasoning, complete tool transcripts, or
vendor-specific conversation state by default. That data is noisy, sensitive,
and difficult to use safely.

Store durable work facts instead:

- participant identity and kind, when supplied
- claimed scope
- starting and resulting repository revisions
- checks and outcomes
- blockers and risks
- handoff summary
- external effects that already occurred

## Product boundaries

This direction should preserve existing SESS principles:

- **Local-first**: agent work records remain in the local SESS database unless a
  separate sharing feature is explicitly designed
- **Agent-agnostic**: commands and JSON contracts do not depend on a particular
  model provider
- **Human-controlled**: claims are advisory and release approval remains explicit
- **Fail closed**: uncertain external state is reported rather than guessed
- **Evidence, not surveillance**: analytics describe attempts and outcomes, not
  developer or agent productivity

## Future design questions

Protocol v0 resolves record ownership, attribution, claims, check evidence,
handoff correction, and initial retention. Later versions still need to decide:

1. How could a self-asserted participant ID gain optional verification?
2. How should delegated agent identities represent a human principal, scope,
   expiration, and revocation?
3. Which records may eventually be shared through Git?
4. How should backup, export, explicit deletion, and compaction work?
5. What additional evidence is required for independent review approval?

The answers should be tested against concrete human-only, agent-only, and mixed
handoff scenarios before those capabilities expand the protocol.

The concrete record contracts, state transitions, limits, and scenarios are in
the [Agent Work Protocol v0 reference draft](../reference/agent-work-protocol-v0.md).
