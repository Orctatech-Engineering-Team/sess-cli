# SESS CLI — Next Steps

This document consolidates the open GitHub issues and remaining roadmap phases so contributors can quickly find what to work on next.

---

## Open Issues

| # | Title | Phase | Priority |
|---|-------|-------|----------|
| [#4](../../issues/4) | Surface and recover from rebase or push conflicts | Phase 3 | 🔴 High |
| [#5](../../issues/5) | Align CLI output with design guide | Cross-cutting | 🟡 Medium |
| [#6](../../issues/6) | Add GitHub auth command | Phase 4 | 🟡 Medium |
| [#7](../../issues/7) | Introduce configuration initialization | Phase 4 | 🟡 Medium |
| [#8](../../issues/8) | Add session analytics commands | Phase 5 | 🟢 Low |

---

## Recommended Work Order

### 1. Phase 3 — Finish Conflict Recovery (Issue #4)

`sess end` and checkpoint-based interrupted handoff recovery are implemented.

Unfinished rebases are detected before mutation. Saved prompts, repository
identity, and completed phases survive restarts. Matching open or merged PRs
are reconciled before cleanup; closed-unmerged PRs leave the session open. See the
[current milestone](IMPLEMENTATION-PLAN.md#current-milestone-interrupted-sess-end-recovery)
for behavior and limits.

Start persistence failures now return errors while preserving the created branch.
Retrying the same interactive selection offers explicit adoption; `sess start
--adopt <branch>` tracks an exact existing local branch without issue metadata.
Session creation and its lifecycle event commit together.

### 2. Cross-Cutting — Output Quality (Issue #5)

Before Phase 4 work begins, existing command output should conform to [docs/cli_design_guide.md](cli_design_guide.md) — no emojis, compact durations, git-like phrasing.

- [#5 — Align CLI output with design guide](../../issues/5): update `start`, `status`, `pause`, `resume`, `projects` output.

### 3. Phase 4 — Auth & Configuration (Issues #6, #7)

Reduce setup friction and allow per-repo customization.

- [#6 — `sess auth login`](../../issues/6): GitHub OAuth / PAT storage, OS keychain integration, fallback to `gh` auth.
- [#7 — `sess config init`](../../issues/7): interactive wizard for global and per-repo config, consumed by start/end workflows.

### 4. Phase 5 — Analytics (Issue #8)

Surface the session data already persisted in SQLite.

- [#8 — Session analytics commands](../../issues/8): extend the new `sess history`, `sess stats`, and `sess report` foundation with richer exports and visualizations.

---

## Future Phases (Not Yet Tracked as Issues)

These items from the roadmap do not yet have GitHub issues. Open issues as work is ready to begin.

### Phase 5 — Visualizations

- Session timeline (Gantt chart in terminal)
- Focus time heatmap
- Issue completion velocity

### Phase 6 — Advanced Features

- **Multi-Session Support** — track multiple branches with `sess switch`
- **Hooks & Extensibility** — pre-start / post-end hooks, plugin system
- **Alternative Issue Trackers** — Jira, Linear, generic webhooks
- **Team Features** — shared templates, team analytics dashboard, session handoff

---

## See Also

- [README.md](../README.md)
- [docs/README.md](README.md)
- [docs/cli_design_guide.md](cli_design_guide.md)
- [docs/technical-spec.md](technical-spec.md)
- [docs/MVP1-SUMMARY.md](MVP1-SUMMARY.md)
- [docs/IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md)
