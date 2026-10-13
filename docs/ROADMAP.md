# SESS Roadmap

This document describes the phased roadmap for SESS. It covers what is done, what is next, and where the project is heading long-term. For the detailed execution plan of in-progress work, see [IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md). For the contributor backlog, see [NEXT-STEPS.md](NEXT-STEPS.md).

## Current State (v0.5.0)

SESS supports start, status, pause, resume, and end with PR handoff. It tracks projects across repositories and shows history, stats, and reports from local SQLite data.

What works today:
- Full session lifecycle with git branch management
- GitHub issue selection and PR creation via `gh`
- Time tracking across pause/resume cycles
- Project tracking across multiple repositories
- CLI analytics: history, stats, report
- Interactive TUI for start and end workflows
- Local SQLite persistence
- Saved checkpoints for interrupted end workflows
- Recorded lifecycle events and active intervals in `sess history <session-id>`
- Explicit adoption of existing local branches after a failed start

## Phase 3: Finish the Lifecycle

**Goal:** Make `sess end` robust against real-world interruptions.

- Implemented: interrupted-workflow recovery for `sess end` (GitHub issue #4). Resolve Git conflicts manually, then retry from saved progress
- Align all CLI output with the design guide (GitHub issue #5): no emojis, compact durations, git-like phrasing across every command

**Success criteria:** A user can hit a rebase conflict during `sess end`, resolve it, and complete the session without data loss.

**Remaining work:** Review output consistency. Automatic conflict resolution and fork-based handoffs are not supported.

## Phase 4: Auth and Configuration

**Goal:** Remove the dependency on `gh` being pre-authenticated and let users configure SESS to their workflow.

- `sess auth login` (GitHub issue #6): GitHub OAuth or PAT storage, OS keychain integration
- `sess config init` (GitHub issue #7): interactive wizard for global and per-repo configuration (base branch defaults, branch naming conventions, default branch type)
- `sess config set` / `sess config get`: direct key-value configuration

**Success criteria:** A new user can run `sess auth login` and `sess config init` and be fully set up without reading documentation.

**Dependencies:** Phase 3 complete.

## Phase 5: Richer Analytics

**Goal:** Turn session data into insight about work patterns.

- Session timeline: a terminal-rendered Gantt chart showing active/paused sessions across a day or week
- Focus time heatmap: visual representation of when focused work happens
- Issue completion velocity: time from session start to PR creation, tracked per issue and per project
- Export formats: JSON, CSV for use in external tools
- Comparison views: this week vs. last week, this project vs. that project

**Success criteria:** A user can run `sess report --week` and immediately understand how their time was spent, where context switches happened, and what shipped.

**Dependencies:** Phase 3 complete. Phase 4 is orthogonal (auth does not affect analytics).

## Phase 6: Web Dashboard

**Goal:** A lightweight web interface for browsing session history and patterns.

Start with a personal time ledger. Layer on analytical insight. End with team features.

### Layer 1: Personal Ledger
- Session history with filtering by project, date, state
- Time per issue, time per project, streaks
- PR links and session details
- Single-user, local server (serves data from the existing SQLite DB)

### Layer 2: Insights
- Focus time blocks and context switch visualization
- Session duration distributions
- Goal tracking (target focus hours, session completion rates)
- Trends over time

### Layer 3: Team Features
- Multi-user session visibility (via shared git data)
- Team activity feed
- Shared goals and project-level metrics

**Success criteria:** A user can open the dashboard after a week of using SESS and see a clear picture of their work patterns without any manual input.

**Dependencies:** Phase 5 analytics inform what the dashboard should display. Phase 7 (team features) feeds into Layer 3.

## Phase 7: Shared Visibility (Team Features)

**Goal:** Let teams see each other's session context without a central server.

- Session data shared via git: push session state as structured commits or a dedicated branch
- `sess share` / `sess pull`: push and pull session context to/from a shared git remote
- Team awareness: see who is working on what, which sessions are active, what shipped today
- Handoff visibility: when a session ends and creates a PR, teammates see the context

**Success criteria:** Two developers on the same repo can see each other's active sessions without asking in Slack.

**Dependencies:** Phase 6 Layer 3 provides the visual interface. The git-based sharing mechanism is independent.

## Phase 8: Nudge System

**Goal:** Gentle, research-backed coaching that helps developers work more intentionally.

All nudges are configurable and opt-in. The system respects the user's preferences and does not nag.

- **Time-based:** "You have been on this session for 3 hours — maybe take a break?"
- **WIP awareness:** "You have started 4 sessions today but finished none — consider finishing before starting new"
- **Stale session detection:** "Your last 3 sessions had no PR — is this work getting lost?"
- **Daily preflight:** Morning briefing of what you left off, what is pending, suggested focus
- **Weekly reflection:** End-of-week summary with patterns and suggestions
- Configurable thresholds and enable/disable per nudge type
- Nudge delivery: terminal output on `sess status`, optional shell notifications

**Success criteria:** A user can enable the nudges they want, configure thresholds, and receive timely, non-intrusive reminders that help them maintain focus.

**Dependencies:** Phase 5 analytics provide the data patterns that drive nudges. Phase 6 could deliver nudges via the dashboard.

## Phase 9: Extensibility

**Goal:** Make SESS adaptable to different workflows and toolchains.

- Pre-start / post-end hooks: run custom scripts at session lifecycle transitions
- Plugin system: extend SESS with custom commands or integrations
- Alternative issue tracker adapters: Jira, Linear, generic webhooks (built on the tracker-agnostic data model)
- Custom nudge sources: let users define their own nudge conditions

**Success criteria:** A user can integrate SESS with their existing toolchain without forking the project.

**Dependencies:** Phases 3-8 provide the stable foundation that extensions build on.

## Phase Ordering

```
Phase 3: Finish Lifecycle     (recovery implemented; output review remains)
Phase 4: Auth & Config        (after Phase 3)
Phase 5: Richer Analytics     (after Phase 3, parallel with Phase 4)
Phase 6: Web Dashboard        (after Phase 5)
Phase 7: Team Features        (after Phase 6 Layer 2)
Phase 8: Nudge System         (after Phase 5, parallel with Phase 6)
Phase 9: Extensibility        (after Phases 3-8 stable)
```

Phases 4 and 5 can run in parallel after Phase 3. Phases 6 and 8 can run in parallel after Phase 5. Phase 9 comes last because it requires a stable API surface to extend.
