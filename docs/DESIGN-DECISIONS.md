# Design Decisions

This document records architectural and design choices that should remain stable as SESS grows. When evaluating new features or changes, refer back to these decisions to stay consistent with the project's direction.

For the technical architecture, see [ARCHITECTURE.md](ARCHITECTURE.md). For CLI output conventions, see [cli_design_guide.md](cli_design_guide.md).

## Data Model Decisions

### Tracker-agnostic session model

Sessions are not tied to GitHub. The session record stores `issue_id` and `issue_title` as generic fields, not GitHub-specific identifiers. This allows future support for Jira, Linear, or other trackers without changing the core data model.

**Rationale:** Locking the data model to GitHub would make Phase 9 (alternative issue trackers) a rewrite instead of an extension. The cost is negligible — the fields are already generic strings.

### Local SQLite persistence

Session data lives in `~/.sess-cli/sess.db`. No remote database, no cloud sync. It stores session state, elapsed time, history events, and saved handoff progress. Git and GitHub cannot reconstruct this history, so keep the database when moving or backing up your work.

**Rationale:** A central server would introduce authentication, availability, and privacy concerns. SQLite is zero-config, single-file, and fast enough for any individual or small team. Team sharing goes through git, not a database.

### Time slice model

Elapsed time is tracked as accumulated nanoseconds in `total_elapsed`, with a `current_slice_start` for the active slice. Pause records the slice boundary. Resume starts a new slice.

**Rationale:** This model handles multiple pause/resume cycles and keeps elapsed totals available for older sessions. A separate lifecycle journal records new transitions for detailed history. Missing older events are not invented.

## Architecture Decisions

### Shell out to git and gh

SESS does not reimplement git operations. It runs `git` and `gh` as subprocesses, captures output, and interprets results.

**Rationale:** Reimplementing git in Go would be a multi-year project with no user benefit. Git's CLI is stable, well-tested, and already installed. The subprocess overhead is negligible compared to network operations. The trade-off is that SESS requires git and gh to be installed, but these are standard developer tools.

### Bubble Tea for TUI

Interactive workflows (start, end) use the Bubble Tea framework with Lipgloss styling.

**Rationale:** Bubble Tea is the standard Go TUI framework, well-maintained by Charm. It handles terminal resizing, keyboard input, and rendering correctly. The alternative (raw terminal handling) would be error-prone and platform-specific.

### Layered architecture with strict dependency rules

The dependency graph flows one direction:

```
cmd/sess -> internal/sess -> internal/tui -> internal/session -> internal/db
                                                             -> internal/git
```

No reverse dependencies. `internal/git` and `internal/db` are leaf packages with no UI imports.

**Rationale:** This makes each layer testable in isolation. Business logic does not depend on the terminal. Database code does not depend on git. Violating these rules would create circular dependencies and make testing harder.

### Migration-first schema

The database layer applies migrations automatically on startup. Old databases get new columns added. No manual migration steps.

**Rationale:** SESS is a local tool. You should not have to run migration commands after upgrading. Migration tests check that existing records survive schema updates.

## UX Decisions

### CLI for actions, web for browsing

The CLI handles starting, pausing, resuming, and ending sessions. It also shows history and reports. A future web dashboard will offer another way to browse history, trends, and shared context; it is not available yet.

**Rationale:** Developers live in the terminal. Forcing them to open a browser for core workflows would be a step backward. But visual patterns — focus heatmaps, session timelines, context switch frequency — are genuinely easier to see in a web interface.

### Git-like output conventions

All CLI output follows the design guide: no emojis, compact durations, `·` separators, relative timestamps, direct wording. Inspired by `git status`.

**Rationale:** Developers already know how to read git output. Matching that convention reduces cognitive load. The design guide enforces consistency across commands so users never have to relearn formatting.

### Interactive by default, flags for automation

`sess start` is interactive by default. `sess start --adopt <branch>` can track an existing local branch without prompts. `sess end` still uses interactive prompts. The tool is designed for humans first, automation second.

**Rationale:** The primary use case is a developer at their terminal. Future automation flags should not complicate the default experience.

## Workflow Decisions

### Fail closed on session lifecycle

If `sess end` fails partway through (rebase conflict, push failure, network error), the session stays open. It is not marked as ended. The user can resolve the issue and try again.

**Rationale:** Marking a session as ended when the PR was not created or the branch was not pushed would lie about the state. A user who sees "ended" assumes the handoff succeeded. Keeping the session open is honest and recoverable.

### Explicit state transitions

Session states are: active, paused, ended. Transitions are deliberate commands (`sess pause`, `sess resume`, `sess end`). Git activity does not automatically change session state.

**Rationale:** Automatic state changes based on git commits or pushes would be unreliable. A commit does not mean work is done. A push does not mean a session should end. Explicit transitions keep the user in control.

### Base branch detection from origin/HEAD

The base branch is detected from `origin/HEAD` at session start, not hardcoded. It is stored per-project in the database.

**Rationale:** Different repositories use different default branches (main, master, dev, develop). Hardcoding `dev` breaks most projects. Detecting from origin and storing per-project handles the common case correctly.

### Dirty working tree handling

When starting a session with a dirty working tree, SESS offers three options: stash, commit, or discard. It does not silently clean the working tree.

**Rationale:** Losing uncommitted work without asking is unacceptable. Each option is explicit and the user chooses based on their situation. Discard runs both `git reset --hard` and `git clean -fd` to match what the prompt describes.

## Future Decisions (Pre-committed)

These decisions are locked in for the roadmap. They constrain what features we build and how.

### Local-first for team features

Team visibility will work through git. No central server. Session data shared as structured commits or a dedicated branch. Anyone with repo access can see team session context.

**Rationale:** This is a core principle from [VISION.md](VISION.md). A central server would introduce auth, availability, and privacy concerns that contradict the local-first philosophy.

### GitHub first, agnostic data model

GitHub is the primary integration today. The data model is designed to support other trackers later without changing the session schema.

**Rationale:** GitHub covers the majority of open source and many private projects. Supporting it first is practical. The agnostic data model means adding Jira or Linear is an adapter, not a rewrite.

### Configurable nudges over mandatory coaching

The nudge system (Phase 8) will be opt-in with configurable thresholds. No nudges fire by default. Users enable and tune what they want.

**Rationale:** Mandatory nudges are annoying. Opt-in nudges are helpful. The research on developer productivity shows that self-directed improvements stick better than imposed ones.

### Web dashboard starts personal, layers on team features

The web dashboard (Phase 6) begins as a single-user personal ledger. Team features come later in Phase 7, using git-based data sharing.

**Rationale:** Building the personal layer first proves the concept and provides value immediately. Team features require the sharing mechanism to exist first. Building both simultaneously would delay both.
