# SESS Vision

## What SESS Is

SESS is a session-based work tool for developers who want intentional, visible, calmer work. It lives in your terminal, understands your git workflow, and adds the one layer that branches alone cannot provide: **what you are doing right now, why, and for how long.**

## The Problem

Every developer knows this: you open a terminal, you're on a branch, and you have to reconstruct context. Which issue was this for? Was I active or did I pause? Did I already push a PR? Where was I?

Branches tell you where code lives. They do not tell you what you are doing. Time trackers like Toggl know how long you worked, but they do not know which branch you were on or whether your PR shipped. GitHub knows about issues and PRs, but it does not know you paused for lunch or that you have been context-switching all week.

SESS fills this gap. It ties your work to explicit sessions with branches, issues, PRs, and time. When you come back, you can check where you left off. When you finish, the handoff to GitHub is one command.

The CLI and local session history work today. The dashboard, team sharing, and nudges described below are planned.

## Core Principles

**Intentionality over autopilot.** Every session is a conscious choice. SESS does not silently track your work in the background. You start a session when you begin focused work, you pause when you step away, and you end when the work ships. This is not a time tracker that runs all day. It is a tool that respects the difference between being at your desk and actually doing focused work.

**Local-first, git-native.** No SESS servers or accounts. Your session data lives in SQLite on your machine. Planned team visibility will use git rather than a central service. GitHub operations use your existing GitHub authentication.

**Opinionated but opt-in.** SESS has a point of view about what makes work calmer and easier, grounded in research on focus, context switching, and developer productivity. But it does not force that point of view on you. Nudges are configurable. Workflows can be overridden. The opinions are there when you want them, invisible when you do not.

**Every character earns its place.** Terminal output is compact, professional, and direct. No emojis, no fluff, no tips that waste your screen space. Inspired by `git status`, `docker ps`, and `kubectl get`. If it does not help you act or understand, it does not appear.

**Fail closed, not open.** When something goes wrong, SESS keeps your session open rather than silently marking it complete. You always know the true state. Safe defaults over aggressive automation.

**Explicit state over implicit magic.** SESS does not guess what you are doing. It asks, and you tell it. Session states are clear: active, paused, ended. Transitions are deliberate. You never have to wonder what SESS thinks is happening.

## Who It Is For

SESS is built for individual developers at heart. The solo engineer juggling three repos and five issues. The person who wants to end the day knowing what they accomplished and where things stand.

It is also built to be usable by small teams and budding startups. Not as a manager's surveillance tool, but as shared context. "I'm working on auth" is more useful when your teammate can see it without asking in Slack.

## What SESS Is Not

SESS is not a time tracker. It does not run in the background all day counting hours. It tracks active, intentional work sessions.

SESS is not a project manager. It does not have boards, sprints, or backlogs. It connects to GitHub issues, where that work already lives.

SESS is not a team chat or standup replacement. It provides visibility through data, not conversation.

SESS is the session layer between your editor and GitHub. Nothing more, nothing less.

## The Three Pillars

### 1. Session Lifecycle

The foundation. Start a session when you begin work. Pause when you step away. Resume when you return. End when the work ships, with a single command that handles commit, push, PR creation, and branch cleanup.

Sessions understand git. They know your branch, your base branch, your issue, and your PR. They survive terminal restarts. They track time accurately across multiple pause/resume cycles.

### 2. Work Overview

The ability to see your patterns. CLI commands for history, stats, and reports answer questions like "what was I working on last week?" and "which repo consumed most of my time?"

A planned web dashboard will start as a personal time ledger and grow into views of work patterns and goals. Tracked active time is not a measurement of uninterrupted focus. The dashboard is for browsing and seeing patterns. The terminal is for doing.

### 3. Shared Visibility

The team layer, built on git. No central server, no accounts, no SaaS. Team awareness works through distributed data — session context shared via git, visible to anyone with repo access.

See who is working on what, whether a session is active or paused, and what shipped today. Not a manager's dashboard, but a teammate's context. The kind of visibility that replaces "what are you working on?" standup questions with data that is always there.

## North Star: A Developer's Ideal Day with SESS

You open your terminal. SESS shows a morning preflight: what you left off yesterday, which sessions are paused, what PRs are awaiting review. You pick up where you left off with `sess resume`.

You work. SESS tracks your session — the branch, the issue, the time. If you have been heads-down for three hours, it gently suggests a break. If you have started four sessions but finished none, it notices.

When you are done, `sess end` handles the handoff: commit, push, PR, branch cleanup. One command. You switch to the next thing with a clear conscience.

At the end of the day, you open the dashboard. You see your focus time, your context switches, what shipped. You notice you have been bouncing between three issues for two days. Tomorrow, you decide to focus on one.

Your teammate sees your session is active on the auth issue. They do not need to ask. They pick up the billing issue without stepping on your work.

This is what SESS is for. Not more tools. Not more dashboards. Just clarity about what you are doing, how it is going, and what comes next.
