# Changelog

This changelog tracks user-visible improvements to SESS.

## v0.5.1 - 2026-10-13

You can now install SESS for your own account without administrator access.
Rerunning the installer updates your existing copy and keeps session data intact.

- New Linux and macOS installs use `~/.local/bin` without sudo. Set
  `SESS_INSTALL_DIR` to choose another directory.
- Rerunning the installer updates an existing writable copy on `PATH`. Protected
  or symlinked installs stop with guidance instead of creating a second copy.
- Updates verify downloads before replacing the binary. Session data stays
  untouched, and the installer reports missing PATH entries or shadowed copies.
- Added a Windows PowerShell installer with user-owned defaults, amd64/arm64
  selection, checksum verification, and in-place updates. It preserves session
  data and prints PATH guidance without changing user settings.
- Windows installer tests now run under PowerShell 5.1, PowerShell 7, and a
  standard account without administrator privileges.

## v0.5.0 - 2026-10-13

SESS now remembers progress when a handoff is interrupted. It also records when
you start, pause, resume, and end a session, so you can see how your work unfolded.

### What's new

- Added saved checkpoints for `sess end`. Retrying resumes saved progress and
  reuses a matching open or merged PR without repeating completed handoff steps.
- Added `sess history <session-id>` with recorded lifecycle events and active
  intervals. History lists include session IDs for opening these details.
- Added `sess start --adopt <branch>` to track an exact existing local branch
  without recreating it or attaching issue metadata. Interactive start also
  offers adoption with the issue context selected during that attempt.

### Reliability fixes

- Failed start persistence now returns an error and preserves the created
  branch for an explicit retry instead of reporting a successful session.
- Session changes, history events, and project activity are saved together.
  A failed save leaves them unchanged. Older writes cannot overwrite newer state.
- End recovery stops on unfinished rebases, unexpected history changes,
  changed origin destinations, or uncertain PR identity and state.
- PR recovery checks the saved repository and branch. It does not associate your
  session with a matching branch from someone else's fork.
- Quitting running start or end workflows cancels their direct subprocesses
  and waits for the workflow to stop.
- Start checkouts and fast-forward base updates refuse to overwrite ignored
  files. Literal branch checks handle same-named tags without confusing them
  with the selected branch.

### Notes

- Existing databases are updated automatically. SESS keeps older elapsed totals
  but does not invent missing events. Older or partial histories are labelled
  incomplete.
- Adoption starts tracking when the session is successfully saved. It does not
  reconstruct time spent before a failed start.
- End recovery currently supports a single origin destination shared by fetch
  and push. Fork-based handoffs and separate push destinations are not supported.
- Cancellation does not terminate hook or SSH descendant process trees; those
  processes may require separate cleanup.
- CI now runs integration tests on Linux, macOS, and Windows, with race checks
  on Linux.

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
