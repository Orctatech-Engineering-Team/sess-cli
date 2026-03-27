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
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | sudo bash
```

This installs `sess` to `/usr/local/bin/sess`.

Install into `/usr/bin` instead:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | sudo env SESS_INSTALL_DIR=/usr/bin bash
```

Install a specific version:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | sudo env SESS_VERSION=v0.3.1 bash
```

Manual install example:

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/sess-linux-amd64.tar.gz | tar xz
sudo install -m 0755 sess /usr/local/bin/sess
```

Windows PowerShell install example:

```powershell
Invoke-WebRequest -Uri "https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/sess-windows-amd64.zip" -OutFile "sess.zip"
Expand-Archive sess.zip -DestinationPath .
Move-Item .\sess.exe "$env:USERPROFILE\\bin\\sess.exe"
# Add $env:USERPROFILE\bin to PATH if it is not already there
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

Not implemented yet:

- resumable conflict recovery for interrupted `sess end` flows
- first-class auth/config commands
- analytics commands

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
sudo install -m 0755 sess /usr/local/bin/sess
```

## Releases

- [Latest release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest)
- [v0.3.1 release](https://github.com/Orctatech-Engineering-Team/sess-cli/releases/tag/v0.3.1)
