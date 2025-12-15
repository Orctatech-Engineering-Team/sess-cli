# DESIGN.md

## Purpose

This document defines the **non-negotiable design principles** and architectural intent behind this project.

This is not documentation of features.
This is not a user guide.

This file exists to answer one question clearly:

> *What must never be violated, even as the system grows?*

When trade-offs arise, this document is the final authority.


## What This Tool Is

This project is a **session-oriented orchestration layer** that sits on top of:

* Git
* The GitHub CLI (`gh`)

It introduces the concept of **focused work sessions** to promote:

* deep, intentional work
* clean Git history
* healthy collaboration practices

It does **not** replace, abstract away, or reinterpret Git.


## What This Tool Is Not

* It is not a version control system
* It is not a Git wrapper that hides commands
* It is not a productivity tracker
* It is not an enforcement mechanism

If Git disappears, this tool must gracefully become irrelevant.


## Core Design Principles (Non-Negotiable)

### 1. Sessions Are a Layer, Never a Replacement

Sessions exist **on top of Git**, not instead of it.

* Git remains the source of truth
* Git commands must remain recognizable
* A Git-literate developer should never feel disoriented

**Forbidden:**

* Reimplementing Git concepts
* Creating hidden state that diverges from Git
* Inventing new semantics for existing Git behavior


### 2. Nothing Happens Silently

Every meaningful action must be observable.

* Commands are streamed, not summarized
* Errors are shown, not swallowed
* Workflow transitions are visible

If the system acts, the developer must see it act.

**Forbidden:**

* Silent rebases
* Implicit commits
* Auto-fixes without disclosure


### 3. Developer Intent Overrides Automation

Automation exists to **support intent**, not replace judgment.

The system may:

* suggest
* sequence
* guide

But it must never coerce.

**Forbidden:**

* Irreversible actions without consent
* No-escape workflows
* Mandatory automation paths


### 4. Failure Is a First-Class State

Failure is expected and respected.

* Tests will fail
* Rebases will conflict
* Linters will complain

The system must:

* stop calmly
* explain clearly
* allow correction
* resume precisely

**Forbidden:**

* Restart-everything flows
* Panic exits
* Vague or collapsed error states


### 5. Sessions Protect Focus

A session is a **boundary around attention**.

* It has an intentional start
* It has a natural rythm of pauses and resumtions
* It has a deliberate end
* It discourages shallow context switching

Sessions exist to make deep work sustainable.

**Forbidden:**

* Always-on background sessions
* Infinite or implicit sessions
* Treating sessions as timers only


### 6. The Tool Teaches by Showing

This system should make good engineering practices visible.

* Show Git output
* Show workflow stages
* Show consequences of actions

Developers should leave with better habits than they arrived with.

**Forbidden:**

* Over-abstracted interfaces
* "Trust me" UX
* Hiding complexity that matters

### 7. Graceful Degradation Is Mandatory

The system must fail safely.

* If the TUI crashes, Git still works
* If orchestration breaks, manual recovery is possible
* State must be recoverable

**Forbidden:**

* Tool-locked workflows
* Irrecoverable session states
* Dependency on the tool for correctness


### 8. Collaboration Is a Moral Concern

Collaboration quality matters.

The system should encourage:

* clean commit history
* readable PRs
* thoughtful handoffs

Not by force, but by structure.

**Forbidden:**

* Encouraging sloppy commits
* Auto-generating meaningless PRs
* Optimizing speed over clarity


### 9. Simplicity Beats Cleverness

If behavior cannot be explained clearly, it does not belong.

* Explicit over implicit
* Few states over clever states
* Calm UX over flashy UX

**Forbidden:**

* Hidden heuristics
* Over-engineered abstractions
* Surprising behavior


### 10. The User Must Always Be Able to Leave

At any point, the developer must be able to stop using the tool and continue normally.

Sessions must end cleanly.

**Forbidden:**

* Lock-in patterns
* Hostage states
* Penalizing exit


## Architectural Implications

From these principles, the following architectural constraints follow:

* Git commands are executed directly
* Output is streamed, not buffered
* Workflows are modeled as state machines
* Automation is interruptible and resumable
* State is explicit and inspectable

Any architectural decision that violates these constraints is invalid.


## Design Ethos

> This tool exists to make disciplined work feel natural, not forced.
>
> It respects the developer, the craft, and the tools it stands on.


## When to Revisit This Document

This document should be revisited when:

* adding new workflow automation
* introducing new session states
* expanding beyond Git/GitHub
* debating "smart" behavior

If a feature conflicts with this document, the feature loses.
