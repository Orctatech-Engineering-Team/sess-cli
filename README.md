# SESS

SESS is a session-based Git CLI for developers working in issue-driven repositories.
It turns a unit of work into an explicit session: start it, pause it, resume it, and end it with a branch push and pull request handoff.

SESS is designed to reduce git ceremony and preserve context across interruptions.

## What SESS Does

- Starts tracked work sessions in a repository
- Creates and resumes session branches
- Tracks active and paused session state in SQLite
- Integrates with GitHub issues through `gh`
- Ends sessions by pushing the branch and creating or reusing a pull request

## Install

Install the latest release:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | bash
```

New installs use `~/.local/bin/sess`, without sudo. If that directory is not on
your `PATH`, the installer shows what to add to your shell configuration:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Choose another directory:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | env SESS_INSTALL_DIR="$HOME/bin" bash
```

Install a specific version:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | env SESS_VERSION=v0.5.0 bash
```

Manual install example:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/sess-linux-amd64.tar.gz | tar xz
mkdir -p "$HOME/.local/bin"
install -m 0755 sess "$HOME/.local/bin/sess"
```

Windows PowerShell install:

```powershell
Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.ps1" -OutFile install.ps1
& .\install.ps1
```

New Windows installs use `%LOCALAPPDATA%\Programs\SESS` without administrator
access. The installer selects amd64 or arm64 and checks the archive checksum.
It prints user PATH instructions without changing your settings.

Choose a directory or version:

```powershell
& .\install.ps1 -InstallDir "$env:USERPROFILE\bin"
& .\install.ps1 -Version v0.5.0
```

Windows PowerShell 5.1 and PowerShell 7 are supported. If your execution policy
blocks the script, follow your organization's policy; the installer does not
change it.

### Update an Existing Install

Rerun the installer to get the latest release. It updates the first `sess`
executable on `PATH` in place, unless you set `SESS_INSTALL_DIR`.
Downloads and checksums are verified before replacing the binary.
Your sessions and history in `~/.sess-cli/sess.db` stay untouched.

If your old copy is in a protected directory such as `/usr/local/bin`, the
installer stops rather than silently creating a second copy. You can choose
`SESS_INSTALL_DIR="$HOME/.local/bin"` and put it first on `PATH`, or update the
system-wide copy explicitly:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | sudo env SESS_INSTALL_DIR=/usr/local/bin bash
```

For package-managed or symlinked installs, use the original package manager.
After an update, open a new terminal or run `hash -r`, then check:

```bash
command -v sess
sess --version
```

On Windows, download the latest installer again and rerun it. It updates the
first `sess.exe` on PATH, or the directory you pass with `-InstallDir`.
Close running SESS processes before updating. Protected files, symlinks, and
junctions stop with guidance rather than creating a second copy.
Use `Get-Command sess` and `sess --version` to check the result.
You can also update manually by replacing the executable from the ZIP archive.

### Enable Bash Completion

Completion is optional and is not enabled by the installer. For the current
Bash session:

```bash
source <(sess completion bash)
```

On Linux, save it for future terminals without sudo:

```bash
completion_dir="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
mkdir -p "$completion_dir"
sess completion bash > "$completion_dir/sess"
source "$completion_dir/sess"
```

With `bash-completion` installed and loaded by your shell, new terminals load
this file when you complete a `sess` command. If that loader is not available,
follow your distribution's setup instructions, or add the `source` command
with the saved file's path to `~/.bashrc`.

Try `sess sta` followed by Tab to see command suggestions. Regenerate the file
after upgrading to get completion changes.

SESS also generates completion scripts for Zsh, Fish, and PowerShell:

```bash
sess completion --help
```

## Prerequisites

- `git`
- `gh`
- a git repository on disk

For GitHub issue selection and PR creation, `gh` must already be authenticated.

Verify the install:

```bash
sess --version
```

## Quick Start

In a repository:

```bash
sess start
sess status
sess pause
sess resume
sess end
```

Typical workflow:

1. Run `sess start` and select an issue or provide a feature name.
2. Work on the created branch.
3. Use `sess pause` and `sess resume` when interrupted.
4. Run `sess end` to commit dirty work if needed, rebase onto the tracked base branch, push, create or reuse a PR, and return to that base branch.

## Commands

| Command | Purpose |
| --- | --- |
| `sess start [feature-name]` | Start a new session |
| `sess status` | Show the current session state |
| `sess pause` | Pause the active session |
| `sess resume` | Resume a paused session |
| `sess end` | End a session and hand off work through a PR |
| `sess projects` | List tracked projects |
| `sess history` | Show recent session history for the current project or all tracked projects |
| `sess stats` | Show aggregate session statistics for the current project or all tracked projects |
| `sess report` | Show a compact session report with summary metrics and recent sessions |
| `sess context --json` | Emit read-only structured session context |
| `sess attempt` | Manage human, agent, and automation work attempts |
| `sess check record` | Record revision-bound validation evidence |
| `sess handoff create` | Record an append-only session handoff |

## Documentation

User docs:

- [Documentation index](docs/README.md)
- [Tutorial: Get started with SESS](docs/tutorials/get-started.md)
- [How-to: Run the session workflow](docs/how-to/session-workflow.md)
- [How-to: Troubleshoot common problems](docs/how-to/troubleshooting.md)
- [Reference: Commands](docs/reference/commands.md)
- [Reference: Session model](docs/reference/session-model.md)
- [Explanation: How SESS works](docs/explanation/how-sess-works.md)

Contributor and project docs:

- [Architecture](docs/ARCHITECTURE.md)
- [Development](docs/DEVELOPMENT.md)
- [CLI design guide](docs/cli_design_guide.md)
- [Implementation plan](docs/IMPLEMENTATION-PLAN.md)
- [Next steps](docs/NEXT-STEPS.md)
- [Releasing](docs/RELEASING.md)

## Current Scope

Implemented:

- `sess start`
- `sess status`
- `sess pause`
- `sess resume`
- `sess end`
- `sess projects`
- `sess history`
- `sess stats`
- `sess report`
- `sess context --json`
- `sess attempt`
- `sess check record`
- `sess handoff create`

Interrupted `sess end` workflows resume from saved checkpoints. Resolve Git
conflicts manually, then retry; saved PR inputs and completed handoff phases
survive restarts.

`sess history <session-id>` shows recorded lifecycle events and active intervals.
Session changes and history events are saved together. Older sessions keep their
totals without invented history. Session IDs appear in history and report lists.

If a start attempt creates a branch but cannot save the session, the branch stays
intact. Retry the same selection and choose `Track existing branch`, or run
`sess start --adopt <branch>` without issue metadata.

Not implemented yet:

- automatic conflict resolution and fork-based end recovery
- first-class auth/config commands
- richer analytics exports and visualizations

## Data Storage

SESS stores state locally in SQLite at:

```text
~/.sess-cli/sess.db
```

Tracked data includes projects, session state, elapsed time, linked issue metadata, and PR metadata for ended sessions.

## Building From Source

```bash
git clone https://github.com/Orctatech-Engineering-Team/sess-cli.git
cd sess-cli
go build -o sess ./cmd/sess
mkdir -p "$HOME/.local/bin"
install -m 0755 sess "$HOME/.local/bin/sess"
```

## Releases

- [Latest release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest)
- [v0.5.1 release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.5.1)
- [v0.5.0 release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.5.0)
- [v0.4.0 release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.4.0)
- [v0.3.1 release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.3.1)
