# SESS CLI Design Guidelines

## Core Principles

### 1. Information Density
- **At-a-glance readability**: Most important info in first 1-2 lines
- **Compact format**: Use inline separators (`·`) instead of multi-line blocks
- **No wasted space**: Every line serves a purpose
- **Scannable**: Users should extract key info in under 2 seconds

### 2. Consistency
- **Uniform formatting** across all commands
- **Same information hierarchy**: branch → issue → time → metadata
- **Predictable output structure**: Users know what to expect

### 3. Professional Tone
- **No emojis**: Text-only for terminal compatibility and professionalism
- **No fluff**: No "Happy coding!" or unnecessary encouragement
- **No tips**: Users can read docs; output should be actionable data only
- **Direct language**: "Not a tracked project" not "This directory is not a tracked SESS project"

### 4. Git-Like Familiarity
- Start with `On branch <name>` for status-like commands
- Use relative timestamps (`2 hours ago` not `2024-12-13 15:04:05`)
- Keep output minimal and parseable

---

## Formatting Standards

### Text Formatting

**DO:**
```
On branch feature-auth (feature)
Session active · 2h34m · #123
Started 2 hours ago
```

**DON'T:**
```
🟢 Status: ACTIVE
━━━━━━━━━━━━━━━━━━━━━━━━━━━
Branch: feature-auth (feature)
Elapsed Time: 2 hours 34 minutes 12 seconds
```

### Separators
- Use `·` (middle dot) to separate inline items
- Use single blank lines to separate logical sections
- Use `*` to indicate current item in lists
- Avoid heavy borders, boxes, or decorative elements

### Indentation
- 2 spaces for nested information
- Consistent across all commands

### Timestamps
- **Relative** for recent activity: `2 hours ago`, `yesterday`, `just now`
- **Absolute** for old activity: `2024-12-13` (dates older than 7 days)
- **Format**: ISO-like `YYYY-MM-DD HH:MM:SS` when absolute is needed

### Durations
- **Compact format**: `2h34m`, `45m12s`, `30s`
- **Never**: `2 hours 34 minutes 12 seconds` or `2:34:12`

---

## Command Output Patterns

### Status Commands (current state)

**Structure:**
```
On branch <branch-name> (<type>)
<State> · <duration> · <issue-id>
<Started-timestamp>[, <paused-timestamp>]

  <issue-title>

  <action-1>    <description>
  <action-2>    <description>
```

**Example:**
```
On branch feature-auth (feature)
Session active · 2h34m · #123
Started 2 hours ago

  Add OAuth2 authentication support

  sess pause    Pause the current session
  sess end      End and save the session
```

### Action Commands (state changes)

**Structure:**
```
<Action-result> on <branch-name> (<type>)
[Issue #<id> · <title>]
<Result-metric>: <value>
```

**Example:**
```
Paused session on feature-auth (feature)
Issue #123 · Add OAuth2 authentication support
Total elapsed: 2h34m
```

### List Commands (multiple items)

**Structure:**
```
<List-title> (<count>)

<item-name> [*]
  <item-path>
  <item-status> · <item-details>
  <item-timestamp>

<item-name>
  ...
```

**Example:**
```
Tracked projects (3)

my-app *
  /home/user/projects/my-app
  active · feature-auth · 2h34m · #123
  2 hours ago

website
  /home/user/projects/website
  idle · base main
  3 days ago
```

### Error Messages

**Structure:**
```
Error: <concise-description>

<Optional-suggestion>
```

**Examples:**
```
Error: not a tracked project. Run 'sess start' first
```

```
Error: no paused session found. Run 'sess start' to begin
```

---

## Information Hierarchy

### Primary Information (Line 1)
- Branch name with type
- Current state/action result

### Secondary Information (Line 2)
- Time metrics (elapsed, started)
- Issue reference
- State details

### Tertiary Information (Line 3+)
- Issue titles (can be long)
- Additional context
- Available actions

---

## Branch Display

**Format:** `<branch-name> (<type>)`

**Examples:**
- `feature-auth (feature)`
- `bugfix-login (bugfix)`
- `main` (no type shown for base branch)

---

## Issue Display

**Inline format:** `#<id> · <title>`

**Separate lines for lists:**
```
Issue #123 · Add OAuth2 authentication support
```

**In status line:**
```
Session active · 2h34m · #123
```

---

## State Indicators

**Text-only, lowercase when inline:**
- `active`
- `paused`
- `idle`

**Uppercase when standalone:**
- `ACTIVE`
- `PAUSED`
- `IDLE`

---

## Common Patterns

### Current Directory Indicator
Use `*` suffix: `my-app *`

### Time Formatting
```go
func formatDuration(d time.Duration) string {
    seconds := int64(d.Seconds())
    hours := seconds / 3600
    minutes := (seconds % 3600) / 60
    secs := seconds % 60

    if hours > 0 {
        return fmt.Sprintf("%dh%dm", hours, minutes)
    } else if minutes > 0 {
        return fmt.Sprintf("%dm%ds", minutes, secs)
    }
    return fmt.Sprintf("%ds", secs)
}
```

### Relative Time Formatting
```go
func formatRelativeTime(t time.Time) string {
    duration := time.Since(t)
    days := int(duration.Hours() / 24)
    hours := int(duration.Hours())
    minutes := int(duration.Minutes())

    if days > 7 {
        return t.Format("2006-01-02")
    } else if days > 1 {
        return fmt.Sprintf("%d days ago", days)
    } else if days == 1 {
        return "yesterday"
    } else if hours > 1 {
        return fmt.Sprintf("%d hours ago", hours)
    } else if hours == 1 {
        return "1 hour ago"
    } else if minutes > 1 {
        return fmt.Sprintf("%d minutes ago", minutes)
    } else if minutes == 1 {
        return "1 minute ago"
    }
    return "just now"
}
```

### Branch Display with Type
```go
branchDisplay := sess.Branch
if sess.BranchType != "" {
    branchDisplay = fmt.Sprintf("%s (%s)", sess.Branch, sess.BranchType)
}
```

---

## Anti-Patterns (What NOT to Do)

### ❌ Emojis
```
🟢 Session Active
🎫 Issue: #123
⏱️ Elapsed: 2h34m
```

### ❌ Heavy Borders
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Session Information
━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### ❌ Verbose Durations
```
Elapsed Time: 2 hours, 34 minutes, and 12 seconds
```

### ❌ Unnecessary Encouragement
```
Happy coding! 🚀
Great job! Keep it up! 💪
```

### ❌ Over-Explaining
```
This directory is not currently being tracked by the SESS session management system.
You will need to initialize tracking by running the 'sess start' command first.
```

### ❌ Redundant Headers
```
PROJECT INFORMATION:
  Name: my-app
  Path: /home/user/projects/my-app

SESSION INFORMATION:
  Status: Active
  Branch: feature-auth
```

---

## Checklist for New Commands

- [ ] No emojis used
- [ ] Consistent indentation (2 spaces)
- [ ] Inline separators use `·`
- [ ] Durations in compact format (`2h34m`)
- [ ] Timestamps are relative when recent
- [ ] Branch includes type in parentheses
- [ ] Error messages are concise and actionable
- [ ] Output is scannable (key info in first 2 lines)
- [ ] No unnecessary fluff or encouragement
- [ ] Follows established patterns for similar commands
- [ ] Total output is under 10 lines for simple cases

---

## Examples by Command Type

### Query Command (sess status)
```
On branch feature-auth (feature)
Session active · 2h34m · #123
Started 2 hours ago

  Add OAuth2 authentication support

  sess pause    Pause the current session
  sess end      End and save the session
```

### Action Command (sess pause)
```
Paused session on feature-auth (feature)
Issue #123 · Add OAuth2 authentication support
Total elapsed: 2h34m
```

### List Command (sess projects)
```
Tracked projects (3)

my-app *
  /home/user/projects/my-app
  active · feature-auth · 2h34m · #123
  2 hours ago

api-service
  /home/user/projects/api-service
  idle · base main
  yesterday
```

### Error State
```
No tracked projects

Run 'sess start' in a directory to begin tracking.
```

---

## Summary

**The Golden Rule:** Every character should earn its place. If it doesn't provide actionable information or essential context, remove it.

**Design for:**
- Terminal professionals who value efficiency
- Quick glances during development workflow  
- Scriptability and parseability
- Cross-platform compatibility

**Inspiration:**
- `git status` - Familiar, concise, informative
- `docker ps` - Dense, tabular, scannable
- `kubectl get` - Professional, consistent, minimal