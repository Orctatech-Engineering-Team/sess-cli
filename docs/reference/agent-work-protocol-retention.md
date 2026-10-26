# Agent Work Protocol record lifecycle

**Status:** Approved for future implementation planning
**Issue:** [#15](https://github.com/Orctatech-Engineering-Team/sess-cli/issues/15)

Protocol v0 keeps attempts, checks, and handoffs indefinitely. This document
defines a later lifecycle for backing up, exporting, deleting, and compacting
those records. It does not authorize destructive commands.

## Goals

The lifecycle must:

- make data loss difficult and visible
- preserve the meaning and source of retained evidence
- support moving records between machines
- let a user inspect the exact scope before deletion
- recover safely from interrupted maintenance
- keep local-first and agent-agnostic behavior
- avoid changing normal session or agent workflows

## Non-goals

The lifecycle does not:

- synchronize protocol records between repositories
- authenticate participants
- infer missing history
- silently delete records based on age or size
- turn summaries into proof that the original evidence still exists

## Record classes

Protocol records have different preservation requirements:

- **Attempt**: who contributed, when, where, and how the attempt ended
- **Check**: what validation ran, against which revision, and its outcome
- **Handoff**: what one participant left for the next participant
- **Context**: a derived snapshot and never an archival record

Compaction must preserve attempts, check outcomes, original handoffs, correction
relationships, repository revisions, and participant attribution. It may not
collapse them into one undifferentiated summary.

## Backup

Backup is the first operation users should perform before deletion or compaction.

### Backup contract

A backup should:

- capture a coherent SQLite snapshot while SESS is not writing
- include the database schema version
- include protocol version information
- include the source database path and creation time
- write to a user-selected path outside the active database directory when
  possible
- verify the backup can be opened read-only before reporting success
- leave the active database unchanged on failure

The implementation should use SQLite's consistent backup mechanism rather than
copying a live database file byte-for-byte. A backup must include journal or WAL
state needed to restore a coherent snapshot.

### Backup safety

Never overwrite an existing backup by default. Require an explicit replacement
choice and write a temporary file before the final rename. A successful backup
should report its path, byte size, schema version, and content checksum.

Backups can contain issue titles, paths, commands, and human-written notes.
They must use the same filesystem permissions as the local SESS database and
must not be uploaded automatically.

## Export

Export is for inspection, migration, and long-term archival. It is distinct from
a SQLite backup.

### Export format

The first export format should be versioned JSON Lines:

```text
{"export_version":"sess.agent-work-export/v1","kind":"manifest",...}
{"export_version":"sess.agent-work-export/v1","kind":"participant",...}
{"export_version":"sess.agent-work-export/v1","kind":"attempt",...}
{"export_version":"sess.agent-work-export/v1","kind":"check",...}
{"export_version":"sess.agent-work-export/v1","kind":"handoff",...}
```

Each record keeps its original opaque ID. Relationships use those IDs rather
than row numbers. Records are ordered by kind and ID for deterministic output;
the manifest records the source database, export time, schema version, protocol
version, record counts, and checksum algorithm.

Export must:

- read without changing session state or Git
- preserve `null` and empty values distinctly where the source does
- include supersession relationships
- include check reproducibility and repository revisions
- omit raw command output because Protocol v0 does not store it by default
- fail rather than silently omit a record

An import design is separate. Exporting a file must not imply that importing it
is safe or supported.

## Explicit deletion

Deletion must always be user initiated and previewable.

### Preview

Before deleting, SESS should show:

- the selected project or session
- attempts and participant IDs affected
- checks and handoffs affected
- supersession relationships that will lose their source
- the backup path and checksum
- the number of records remaining afterward

The preview must not mutate the database. The final command must require an
explicit confirmation flag or interactive confirmation. A bare status command
must never delete protocol records.

### Scope

The initial deletion scope should be one complete session. Partial deletion of
an attempt, check, or handoff creates broken evidence relationships and should
wait for a separate design.

Deleting a session's protocol records must not silently delete the session's
ordinary lifecycle history or elapsed totals. The command must state which
records it removes.

### Execution

Deletion should:

1. create and verify a backup
2. begin one SQLite transaction
3. re-check the selected records and backup precondition
4. delete protocol records in dependency order
5. commit or roll back as one unit
6. report the backup needed for recovery

If the database changed after the preview, fail and require a new preview.

## Compaction

Compaction reduces storage without pretending that distinct work events were one
event. It is not deletion with a friendlier name.

### Safe compaction shape

A future design may archive immutable records into a versioned export and leave
an in-database archive manifest:

```json
{
  "archive_id": "archive:01",
  "source_record_ids": ["handoff:1", "handoff:2"],
  "archive_path": "/home/example/.sess-cli/archive-01.jsonl",
  "checksum": "sha256:...",
  "created_at": "2026-10-18T12:00:00Z"
}
```

The manifest must preserve:

- every source record ID
- record kind and session
- original ordering
- supersession relationships
- archive path and checksum
- protocol and export versions

Until archive verification succeeds, no source record may be removed. Context
must show when a record is archived and how to locate it. A compacted context
summary cannot be presented as the complete history.

### What must not be compacted away

Never discard:

- failed or cancelled attempts
- failed checks
- the original handoff superseded by a correction
- participant-to-attempt attribution
- repository revisions
- evidence that a check was run on a dirty workspace

These facts explain how the final state was reached.

## Retention policies

Protocol v0 default:

- no automatic deletion
- no automatic compaction
- no age-based cleanup
- no size-based cleanup
- no network backup

A later policy may offer opt-in thresholds by session age, project, record kind,
or database size. Policies must preview their effect and require a verified
backup before destructive work.

## Interrupted maintenance

Backup, export, deletion, and compaction can be interrupted by cancellation,
process termination, disk-full errors, or concurrent database writes.

The implementation must:

- use temporary files and atomic rename for file outputs
- remove incomplete temporary files on the next run
- keep the active database valid after any failure
- use SQLite transactions for database changes
- reject concurrent destructive maintenance
- make an in-progress marker recoverable after restart
- never treat an incomplete archive as verified
- provide a repair or resume path before retrying

The maintenance marker should identify the operation, target scope, temporary
path, and start time. It must not contain secrets or raw check output.

## Privacy and security

Lifecycle files can contain repository paths, issue titles, commands, and
human-written work summaries. Implementations must:

- use user-only filesystem permissions by default
- avoid shell interpolation of user-provided paths
- validate archive paths and refuse traversal
- refuse symlinked destinations unless explicitly supported
- avoid uploading backups or exports
- redact secrets supplied in summaries before export when possible, while still
  warning that caller-supplied text cannot be fully inspected
- make archive checksums and record counts visible to the user

## Approved decisions

The maintainer approved these decisions on 2026-10-18:

1. Backup before every destructive operation.
2. JSON Lines for the first export format.
3. One-session deletion as the first deletion scope.
4. No partial record deletion.
5. Explicit confirmation after a preview.
6. Verified archive manifest before compaction removes source rows.
7. No automatic cleanup in Protocol v0.

These decisions define the design boundary for future backup, export, deletion,
and compaction issues. They do not authorize implementation by themselves.
