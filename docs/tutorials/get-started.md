# Get Started With SESS

This tutorial walks through a complete first session in a git repository.

By the end, you will:

- install `sess`
- start a session
- pause and resume it
- end it and return to the base branch

## Before You Begin

You need:

- `git`
- `gh`
- a git repository on disk

For issue selection and PR creation, make sure `gh` is already authenticated.

## 1. Install SESS

```bash
curl -fsSL https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/install.sh | sudo bash
```

On Windows, download and extract the archive instead:

```powershell
Invoke-WebRequest -Uri "https://github.com/Orctatech-Engineering-Team/sess-cli/releases/latest/download/sess-windows-amd64.zip" -OutFile "sess.zip"
Expand-Archive sess.zip -DestinationPath .
Move-Item .\sess.exe "$env:USERPROFILE\\bin\\sess.exe"
# Add $env:USERPROFILE\bin to PATH if it is not already there
```

Check that it is available:

```bash
sess --version
```

## 2. Move Into a Repository

```bash
cd /path/to/your-repo
```

SESS must run inside a git repository.

## 3. Start a Session

Run:

```bash
sess start
```

SESS will guide you through:

- selecting a GitHub issue or starting without one
- entering a branch name when needed
- choosing a branch type such as `feature`, `bugfix`, or `refactor`
- resolving dirty working tree state before branching

If the repository is already tracked, SESS reuses that tracked project record.
If not, it creates one and stores it locally.

## 4. Check the Session State

Run:

```bash
sess status
```

You should see:

- the current branch
- whether the session is active or paused
- elapsed time
- linked issue information when present

## 5. Pause the Session

If you need to stop working for a while:

```bash
sess pause
```

The session remains tracked, but time stops accumulating.

## 6. Resume the Session

When you are ready to continue:

```bash
sess resume
```

If you are no longer on the saved session branch, SESS checks it out before resuming.

## 7. End the Session

When the work is ready to hand off:

```bash
sess end
```

Depending on the repository state, SESS may:

- ask for a commit message if the working tree is dirty
- ask for PR summary, testing, and notes
- rebase onto the tracked base branch
- push the session branch
- create a new PR or reuse an existing one
- switch back to the base branch
- prompt whether to keep or delete the local session branch

If there is nothing to ship, SESS lets you end the session without creating a PR.

## 8. Review the Result

After `sess end`, you should be back on the base branch.

Check:

```bash
sess status
sess projects
```

Use `sess projects` to see tracked repositories and recent session state across your system.

## Next Steps

- For common day-to-day tasks, see [How-to: Run the session workflow](../how-to/session-workflow.md).
- For exact command behavior, see [Reference: Commands](../reference/commands.md).
- For failure handling and recovery guidance, see [How-to: Troubleshoot common problems](../how-to/troubleshooting.md).
