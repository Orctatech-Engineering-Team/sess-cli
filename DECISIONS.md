# Decisions

This file summarizes the stable design decisions for SESS. The full records are in
[docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md).

## Tracker-agnostic session model

**Context and decision:**
Sessions are not tied to GitHub. The session record stores `issue_id` and `issue_title` as generic fields, not GitHub-specific identifiers. This allows future support for Jira, Linear, or other trackers without changing the core data model.

**Options considered and reason:**
Locking the data model to GitHub would make Phase 9 (alternative issue trackers) a rewrite instead of an extension. The cost is negligible — the fields are already generic strings.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Local SQLite persistence

**Context and decision:**
Session data lives in `~/.sess-cli/sess.db`. No remote database, no cloud sync. The database is a cache of what git and GitHub already know, optimized for fast queries and offline access.

**Options considered and reason:**
A central server would introduce authentication, availability, and privacy concerns. SQLite is zero-config, single-file, and fast enough for any individual or small team. Team sharing goes through git, not a database.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Time slice model

**Context and decision:**
Elapsed time is tracked as accumulated nanoseconds in `total_elapsed`, with a `current_slice_start` for the active slice. Pause records the slice boundary. Resume starts a new slice.

**Options considered and reason:**
This model handles multiple pause/resume cycles accurately without storing every timestamp. It is simple to query (total = sum of completed slices + current slice) and does not require complex interval arithmetic.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Shell out to git and gh

**Context and decision:**
SESS does not reimplement git operations. It runs `git` and `gh` as subprocesses, captures output, and interprets results.

**Options considered and reason:**
Reimplementing git in Go would be a multi-year project with no user benefit. Git's CLI is stable, well-tested, and already installed. The subprocess overhead is negligible compared to network operations. The trade-off is that SESS requires git and gh to be installed, but these are standard developer tools.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Bubble Tea for TUI

**Context and decision:**
Interactive workflows (start, end) use the Bubble Tea framework with Lipgloss styling.

**Options considered and reason:**
Bubble Tea is the standard Go TUI framework, well-maintained by Charm. It handles terminal resizing, keyboard input, and rendering correctly. The alternative (raw terminal handling) would be error-prone and platform-specific.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Layered architecture with strict dependency rules

**Context and decision:**
The dependency graph flows one direction:

```
cmd/sess -> internal/sess -> internal/tui -> internal/session -> internal/db
                                                             -> internal/git
```

No reverse dependencies. `internal/git` and `internal/db` are leaf packages with no UI imports.

**Options considered and reason:**
This makes each layer testable in isolation. Business logic does not depend on the terminal. Database code does not depend on git. Violating these rules would create circular dependencies and make testing harder.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Migration-first schema

**Context and decision:**
The database layer applies migrations automatically on startup. Old databases get new columns added. No manual migration steps.

**Options considered and reason:**
SESS is a local tool. Users should not have to run migration commands after upgrading. Automatic migrations are safe because the schema only grows (new columns with defaults), never shrinks.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## CLI for actions, web for browsing

**Context and decision:**
The terminal is where work happens. The web dashboard is where patterns emerge. SESS keeps these concerns separate: the CLI handles starting, pausing, resuming, and ending sessions. The dashboard handles browsing history, viewing trends, and seeing team activity.

**Options considered and reason:**
Developers live in the terminal. Forcing them to open a browser for core workflows would be a step backward. But visual patterns — focus heatmaps, session timelines, context switch frequency — are genuinely easier to see in a web interface.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Git-like output conventions

**Context and decision:**
All CLI output follows the design guide: no emojis, compact durations, `·` separators, relative timestamps, direct wording. Inspired by `git status`.

**Options considered and reason:**
Developers already know how to read git output. Matching that convention reduces cognitive load. The design guide enforces consistency across commands so users never have to relearn formatting.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Interactive by default, flags for automation

**Context and decision:**
`sess start` is interactive by default but accepts flags for scripting. `sess end` prompts for confirmation but can be silenced. The tool is designed for humans first, automation second.

**Options considered and reason:**
The primary use case is a developer at their terminal. Automation is a secondary use case that should not complicate the default experience. Flags like `--yes` or `--quiet` handle scripting without cluttering the interactive flow.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Fail closed on session lifecycle

**Context and decision:**
If `sess end` fails partway through (rebase conflict, push failure, network error), the session stays open. It is not marked as ended. The user can resolve the issue and try again.

**Options considered and reason:**
Marking a session as ended when the PR was not created or the branch was not pushed would lie about the state. A user who sees "ended" assumes the handoff succeeded. Keeping the session open is honest and recoverable.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Explicit state transitions

**Context and decision:**
Session states are: active, paused, ended. Transitions are deliberate commands (`sess pause`, `sess resume`, `sess end`). There is no automatic state changes based on git activity.

**Options considered and reason:**
Automatic state changes based on git commits or pushes would be unreliable. A commit does not mean work is done. A push does not mean a session should end. Explicit transitions keep the user in control.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Base branch detection from origin/HEAD

**Context and decision:**
The base branch is detected from `origin/HEAD` at session start, not hardcoded. It is stored per-project in the database.

**Options considered and reason:**
Different repositories use different default branches (main, master, dev, develop). Hardcoding `dev` breaks most projects. Detecting from origin and storing per-project handles the common case correctly.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Dirty working tree handling

**Context and decision:**
When starting a session with a dirty working tree, SESS offers three options: stash, commit, or discard. It does not silently clean the working tree.

**Options considered and reason:**
Losing uncommitted work without asking is unacceptable. Each option is explicit and the user chooses based on their situation. Discard runs both `git reset --hard` and `git clean -fd` to match what the prompt describes.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Local-first for team features

**Context and decision:**
Team visibility will work through git. No central server. Session data shared as structured commits or a dedicated branch. Anyone with repo access can see team session context.

**Options considered and reason:**
This is a core principle from [VISION.md](VISION.md). A central server would introduce auth, availability, and privacy concerns that contradict the local-first philosophy.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## GitHub first, agnostic data model

**Context and decision:**
GitHub is the primary integration today. The data model is designed to support other trackers later without changing the session schema.

**Options considered and reason:**
GitHub covers the majority of open source and many private projects. Supporting it first is practical. The agnostic data model means adding Jira or Linear is an adapter, not a rewrite.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Configurable nudges over mandatory coaching

**Context and decision:**
The nudge system (Phase 8) will be opt-in with configurable thresholds. No nudges fire by default. Users enable and tune what they want.

**Options considered and reason:**
Mandatory nudges are annoying. Opt-in nudges are helpful. The research on developer productivity shows that self-directed improvements stick better than imposed ones.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)

## Web dashboard starts personal, layers on team features

**Context and decision:**
The web dashboard (Phase 6) begins as a single-user personal ledger. Team features come later in Phase 7, using git-based data sharing.

**Options considered and reason:**
Building the personal layer first proves the concept and provides value immediately. Team features require the sharing mechanism to exist first. Building both simultaneously would delay both.

**Full record:** [docs/DESIGN-DECISIONS.md](docs/DESIGN-DECISIONS.md)
