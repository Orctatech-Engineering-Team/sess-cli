# Structure AI agent work with SESS

SESS can give humans and coding agents a shared, durable structure for software
work. The goal is not to record an agent's private reasoning. It is to preserve
the facts another human or agent needs to continue safely.

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

## Minimal first slice

Do not begin with a general agent orchestration system. Start with a read-only
context command:

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

After the context contract is proven, add attempts, checks, and handoffs as thin
end-to-end slices.

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

## Open questions

Before attempts become persistent, decide:

1. Is an attempt always nested inside one session?
2. How is a participant identified without requiring accounts?
3. Which claims may overlap, and how are stale claims cleared?
4. Which check outputs are retained, and what information must be redacted?
5. How does review approval prove independence from implementation?
6. Which records may eventually be shared through Git?
7. What is the retention policy for agent-generated work records?

The answers should be tested against concrete human-only, agent-only, and mixed
handoff scenarios before expanding the database model.
