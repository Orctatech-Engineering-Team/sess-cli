package db

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestOpenReadOnlyPreservesDatabase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createdPath := filepath.Join(dir, "created.db")
	path := filepath.Join(dir, "sess ? # context.db")
	database, err := Open(createdPath)
	if err != nil {
		t.Fatal(err)
	}
	project, err := database.CreateProject("project", t.TempDir(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(createdPath, path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	readOnly, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readOnly.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != project.Name {
		t.Fatalf("project = %#v, want %#v", got, project)
	}
	if _, err := readOnly.CreateProject("other", t.TempDir(), "main"); err == nil {
		t.Fatal("CreateProject() succeeded on read-only database")
	}
	if err := readOnly.Close(); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("read-only access changed database contents")
	}
}

func TestOpenReadOnlyDoesNotCreateMissingDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.db")
	if database, err := OpenReadOnly(path); err == nil {
		database.Close()
		t.Fatal("OpenReadOnly() succeeded for missing database")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing database was created: %v", err)
	}
}

func newTestDB(t *testing.T) *DB {
	t.Helper()

	database, err := Open(filepath.Join(t.TempDir(), "sess.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
	})

	return database
}

func newTestProject(t *testing.T, database *DB) *Project {
	t.Helper()

	project, err := database.CreateProject("repo", filepath.Join(t.TempDir(), "repo"), "main")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	return project
}

func TestOpenMigratesCurrentSliceStartColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess.db")

	rawDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw sqlite db: %v", err)
	}

	schema := `
	CREATE TABLE projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		created_at DATETIME NOT NULL,
		last_used_at DATETIME NOT NULL,
		base_branch TEXT NOT NULL DEFAULT 'dev',
		is_active BOOLEAN NOT NULL DEFAULT 1
	);

	CREATE TABLE sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		branch TEXT NOT NULL,
		issue_id TEXT,
		issue_title TEXT,
		state TEXT NOT NULL CHECK(state IN ('active', 'paused', 'ended')),
		start_time DATETIME NOT NULL,
		pause_time DATETIME,
		end_time DATETIME,
		total_elapsed INTEGER NOT NULL DEFAULT 0,
		branch_type TEXT,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);
	`
	if _, err := rawDB.Exec(schema); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	start := time.Now().Add(-time.Hour).UTC()
	if _, err := rawDB.Exec(`
		INSERT INTO projects (id, name, path, created_at, last_used_at, base_branch)
		VALUES (7, 'legacy', '/legacy', ?, ?, 'main');
	`, start, start); err != nil {
		t.Fatalf("seed legacy project: %v", err)
	}
	if _, err := rawDB.Exec(`
		INSERT INTO sessions (id, project_id, branch, issue_id, issue_title, state, start_time, total_elapsed)
		VALUES (13, 7, 'feature/legacy', '123', 'Legacy issue', 'paused', ?, 456);
	`, start); err != nil {
		t.Fatalf("seed legacy session: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw sqlite db: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close migrated db: %v", err)
		}
	}()

	assertSessionsColumnCount(t, database, "current_slice_start", 1)
	assertSessionsColumnCount(t, database, "pr_number", 1)
	assertSessionsColumnCount(t, database, "pr_url", 1)
	s, err := database.GetActiveSession(7)
	if err != nil || s == nil {
		t.Fatalf("get migrated session = %#v, err = %v", s, err)
	}
	if s.ID != 13 || s.Branch != "feature/legacy" || s.IssueID != "123" ||
		s.IssueTitle != "Legacy issue" || s.TotalElapsed != 456 || !s.StartTime.Equal(start) {
		t.Fatalf("migration changed legacy session: %#v", s)
	}
	events, err := database.ListSessionEvents(s.ID)
	if err != nil || len(events) != 0 {
		t.Fatalf("migration fabricated events: %#v, err = %v", events, err)
	}
	byID, err := database.GetSessionByID(s.ID)
	if err != nil || !reflect.DeepEqual(byID, s) {
		t.Fatalf("get migrated session by ID = %#v, err = %v", byID, err)
	}
	p, err := database.GetProjectByID(7)
	if err != nil || p == nil || p.Path != "/legacy" || p.BaseBranch != "main" || !p.LastUsedAt.Equal(start) {
		t.Fatalf("migration changed legacy project: %#v, err = %v", p, err)
	}
	if err := database.CreateEndWorkflow(testEndWorkflow(s)); err != nil {
		t.Fatalf("create checkpoint after old schema upgrade: %v", err)
	}
}

func TestOpenMigratesEndWorkflowRebaseTargetPreservingRows(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/legacy", StatePaused)
	w := testEndWorkflow(s)
	w.Phase = EndCommitted
	number := int64(42)
	w.PRNumber = &number
	w.PRURL = "https://github.com/example/repo/pull/42"
	if _, err := database.conn.Exec(`
		DROP TABLE end_workflows;
		CREATE TABLE end_workflows (
			session_id INTEGER PRIMARY KEY,
			branch TEXT NOT NULL,
			base_branch TEXT NOT NULL,
			commit_message TEXT NOT NULL,
			end_without_pr BOOLEAN NOT NULL,
			pr_title TEXT NOT NULL,
			pr_body TEXT NOT NULL,
			phase TEXT NOT NULL CHECK(phase IN (
				'prepared', 'committed', 'rebased', 'pushed', 'pr_created', 'base_checked_out'
			)),
			head TEXT NOT NULL,
			pr_number INTEGER,
			pr_url TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
	`); err != nil {
		t.Fatalf("create legacy checkpoint table: %v", err)
	}
	if _, err := database.conn.Exec(`
		INSERT INTO end_workflows (
			session_id, branch, base_branch, commit_message, end_without_pr,
			pr_title, pr_body, phase, head, pr_number, pr_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		w.SessionID,
		w.Branch,
		w.BaseBranch,
		w.CommitMessage,
		w.EndWithoutPR,
		w.PRTitle,
		w.PRBody,
		w.Phase,
		w.Head,
		w.PRNumber,
		w.PRURL,
	); err != nil {
		t.Fatalf("seed legacy checkpoint: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}
	for i := 0; i < 2; i++ {
		reopened, err := Open(database.path)
		if err != nil {
			t.Fatalf("open migrated database on attempt %d: %v", i, err)
		}
		got, err := reopened.GetEndWorkflow(s.ID)
		if err != nil || !reflect.DeepEqual(got, w) {
			t.Errorf("migrated checkpoint = %#v, err = %v, want %#v", got, err, w)
		}
		for _, column := range []string{"rebase_target", "repository", "remote_id"} {
			var notNull int
			var defaultValue string
			err = reopened.conn.QueryRow(`
				SELECT "notnull", dflt_value FROM pragma_table_info('end_workflows')
				WHERE name = ?
			`, column).Scan(&notNull, &defaultValue)
			if err != nil || notNull != 1 || defaultValue != "''" {
				t.Errorf("%s column: notnull = %d, default = %q, err = %v", column, notNull, defaultValue, err)
			}
		}
		if err := reopened.Close(); err != nil {
			t.Fatalf("close migrated database: %v", err)
		}
	}
}

func TestOpenMigratesEndWorkflowRepositoryIdentityPreservingRebaseTarget(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/legacy", StatePaused)
	w := testEndWorkflow(s)
	if err := database.CreateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	w.Phase = EndCommitted
	w.RebaseTarget = "0123456789abcdef0123456789abcdef01234567"
	if err := database.UpdateEndWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec(`
		ALTER TABLE end_workflows DROP COLUMN repository;
		ALTER TABLE end_workflows DROP COLUMN remote_id;
	`); err != nil {
		t.Fatalf("restore checkpoint schema without repository identity: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}
	for i := 0; i < 2; i++ {
		reopened, err := Open(database.path)
		if err != nil {
			t.Fatalf("open migrated database on attempt %d: %v", i, err)
		}
		got, err := reopened.GetEndWorkflow(s.ID)
		if err != nil || !reflect.DeepEqual(got, w) {
			t.Errorf("migrated checkpoint = %#v, err = %v, want %#v", got, err, w)
		}
		if err := reopened.UpdateEndWorkflow(w); err != nil {
			t.Errorf("update migrated checkpoint with empty identity: %v", err)
		}
		if err := reopened.Close(); err != nil {
			t.Fatalf("close migrated database: %v", err)
		}
	}
}

func TestSessionHistoryRoundTripsPRMetadata(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)

	prNumber := int64(45)
	endTime := time.Now()

	if _, err := database.CreateSession(&Session{
		ProjectID:         project.ID,
		Branch:            "feature/with-pr",
		State:             StateEnded,
		StartTime:         time.Now().Add(-time.Hour),
		EndTime:           &endTime,
		CurrentSliceStart: nil,
		TotalElapsed:      int64(30 * time.Minute),
		BranchType:        "feature",
		PRNumber:          &prNumber,
		PRURL:             "https://github.com/example/repo/pull/45",
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	history, err := database.GetSessionHistory(project.ID, 10)
	if err != nil {
		t.Fatalf("get session history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].PRNumber == nil || *history[0].PRNumber != prNumber {
		t.Fatalf("history PR number = %v, want %d", history[0].PRNumber, prNumber)
	}
	if history[0].PRURL != "https://github.com/example/repo/pull/45" {
		t.Fatalf("history PR URL = %q, want PR URL", history[0].PRURL)
	}
}

func assertSessionsColumnCount(t *testing.T, database *DB, column string, want int) {
	t.Helper()

	var count int
	if err := database.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('sessions')
		WHERE name = ?
	`, column).Scan(&count); err != nil {
		t.Fatalf("query %s column: %v", column, err)
	}
	if count != want {
		t.Fatalf("%s column count = %d, want %d", column, count, want)
	}
}

func TestGetActiveSessionReturnsLatestPausedOrActiveSession(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)

	olderStart := time.Now().Add(-2 * time.Hour)
	newerStart := time.Now().Add(-1 * time.Hour)
	pauseTime := time.Now().Add(-30 * time.Minute)

	if _, err := database.CreateSession(&Session{
		ProjectID:         project.ID,
		Branch:            "feature/old",
		State:             StateActive,
		StartTime:         olderStart,
		CurrentSliceStart: &olderStart,
		TotalElapsed:      int64(time.Minute),
		BranchType:        "feature",
	}); err != nil {
		t.Fatalf("create older session: %v", err)
	}

	latest, err := database.CreateSession(&Session{
		ProjectID:         project.ID,
		Branch:            "feature/new",
		State:             StateActive,
		StartTime:         newerStart,
		CurrentSliceStart: &newerStart,
		TotalElapsed:      int64(2 * time.Minute),
		BranchType:        "feature",
	})
	if err != nil {
		t.Fatalf("create newer session: %v", err)
	}
	latest.State = StatePaused
	latest.PauseTime = &pauseTime
	latest.CurrentSliceStart = nil
	if err := database.UpdateSession(latest); err != nil {
		t.Fatalf("pause newer session: %v", err)
	}

	active, err := database.GetActiveSession(project.ID)
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if active == nil {
		t.Fatal("active session = nil, want latest session")
	}
	if active.Branch != "feature/new" {
		t.Fatalf("active session branch = %q, want %q", active.Branch, "feature/new")
	}
	if active.State != StatePaused {
		t.Fatalf("active session state = %q, want %q", active.State, StatePaused)
	}
	if active.PauseTime == nil {
		t.Fatal("paused session missing pause time")
	}
}

func TestGetSessionHistoryOrdersNewestFirstAndAppliesLimit(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)

	startTimes := []time.Time{
		time.Now().Add(-3 * time.Hour),
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-1 * time.Hour),
	}
	branches := []string{"feature/one", "feature/two", "feature/three"}

	for i := range startTimes {
		if _, err := database.CreateSession(&Session{
			ProjectID:         project.ID,
			Branch:            branches[i],
			State:             StateEnded,
			StartTime:         startTimes[i],
			CurrentSliceStart: nil,
			TotalElapsed:      int64(time.Minute),
			BranchType:        "feature",
		}); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	history, err := database.GetSessionHistory(project.ID, 2)
	if err != nil {
		t.Fatalf("get session history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if history[0].Branch != "feature/three" {
		t.Fatalf("history[0].Branch = %q, want %q", history[0].Branch, "feature/three")
	}
	if history[1].Branch != "feature/two" {
		t.Fatalf("history[1].Branch = %q, want %q", history[1].Branch, "feature/two")
	}
}

func TestListSessionsReturnsAllSessionsNewestFirst(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)

	startTimes := []time.Time{
		time.Now().Add(-3 * time.Hour),
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-1 * time.Hour),
	}
	branches := []string{"feature/one", "feature/two", "feature/three"}

	for i := range startTimes {
		if _, err := database.CreateSession(&Session{
			ProjectID:         project.ID,
			Branch:            branches[i],
			State:             StateEnded,
			StartTime:         startTimes[i],
			CurrentSliceStart: nil,
			TotalElapsed:      int64(time.Minute),
			BranchType:        "feature",
		}); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	sessions, err := database.ListSessions(project.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("sessions length = %d, want 3", len(sessions))
	}
	if sessions[0].Branch != "feature/three" {
		t.Fatalf("sessions[0].Branch = %q, want %q", sessions[0].Branch, "feature/three")
	}
	if sessions[1].Branch != "feature/two" {
		t.Fatalf("sessions[1].Branch = %q, want %q", sessions[1].Branch, "feature/two")
	}
	if sessions[2].Branch != "feature/one" {
		t.Fatalf("sessions[2].Branch = %q, want %q", sessions[2].Branch, "feature/one")
	}
}

func endedSession(s *Session) *Session {
	ended := *s
	now := time.Now()
	number := int64(42)
	ended.State = StateEnded
	ended.EndTime = &now
	ended.CurrentSliceStart = nil
	ended.TotalElapsed = int64(time.Hour)
	ended.PRNumber = &number
	ended.PRURL = "https://github.com/example/repo/pull/42"
	return &ended
}

func TestCompleteSessionDeletesOnlyMatchingCheckpoint(t *testing.T) {
	for _, state := range []string{StateActive, StatePaused} {
		t.Run(state, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			first := newTestSession(t, database, project, "feature/first", state)
			other := newTestSession(t, database, project, "feature/other", StateActive)
			for _, s := range []*Session{first, other} {
				if err := database.CreateEndWorkflow(testEndWorkflow(s)); err != nil {
					t.Fatal(err)
				}
			}
			ended := endedSession(first)
			if err := database.CompleteSession(first, ended); err != nil {
				t.Fatalf("complete session: %v", err)
			}
			checkpoint, err := database.GetEndWorkflow(first.ID)
			if err != nil || checkpoint != nil {
				t.Fatalf("completed checkpoint = %#v, err = %v, want nil", checkpoint, err)
			}
			checkpoint, err = database.GetEndWorkflow(other.ID)
			if err != nil || !reflect.DeepEqual(checkpoint, testEndWorkflow(other)) {
				t.Fatalf("other checkpoint = %#v, err = %v", checkpoint, err)
			}
			sessions, err := database.ListSessions(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range sessions {
				if s.ID == first.ID {
					if s.State != StateEnded || s.EndTime == nil || !s.EndTime.Equal(*ended.EndTime) ||
						s.CurrentSliceStart != nil || s.TotalElapsed != ended.TotalElapsed ||
						s.PRNumber == nil || *s.PRNumber != *ended.PRNumber || s.PRURL != ended.PRURL {
						t.Fatalf("completed session = %#v, want %#v", s, ended)
					}
				} else if s.State != StateActive {
					t.Fatalf("other session state = %q, want active", s.State)
				}
			}
			p, err := database.GetProjectByID(project.ID)
			if err != nil || !p.LastUsedAt.After(project.LastUsedAt) {
				t.Fatalf("project was not touched: %#v, err = %v", p, err)
			}
			if err := database.CompleteSession(first, ended); err == nil {
				t.Fatal("duplicate completion succeeded, want error")
			}
		})
	}
}

func TestCompleteSessionRollsBackEveryOperation(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
	}{
		{
			name: "project update",
			trigger: `CREATE TRIGGER fail_touch BEFORE UPDATE ON projects
				BEGIN SELECT RAISE(ABORT, 'project update blocked'); END`,
		},
		{
			name: "checkpoint delete",
			trigger: `CREATE TRIGGER fail_delete BEFORE DELETE ON end_workflows
				BEGIN SELECT RAISE(ABORT, 'checkpoint delete blocked'); END`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			w := testEndWorkflow(s)
			if err := database.CreateEndWorkflow(w); err != nil {
				t.Fatal(err)
			}
			beforeSessions, err := database.ListSessions(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeProject, err := database.GetProjectByID(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.conn.Exec(tt.trigger); err != nil {
				t.Fatalf("create failure trigger: %v", err)
			}
			if err := database.CompleteSession(s, endedSession(s)); err == nil {
				t.Fatal("completion succeeded despite failure trigger")
			}
			afterSessions, err := database.ListSessions(project.ID)
			if err != nil || !reflect.DeepEqual(afterSessions, beforeSessions) {
				t.Fatalf("sessions changed after rollback: %#v, err = %v", afterSessions, err)
			}
			afterProject, err := database.GetProjectByID(project.ID)
			if err != nil || !reflect.DeepEqual(afterProject, beforeProject) {
				t.Fatalf("project changed after rollback: %#v, err = %v", afterProject, err)
			}
			got, err := database.GetEndWorkflow(s.ID)
			if err != nil || !reflect.DeepEqual(got, w) {
				t.Fatalf("checkpoint changed after rollback: %#v, err = %v", got, err)
			}
		})
	}
}

func TestCompleteSessionRejectsInvalidOrStaleIdentity(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Session)
	}{
		{name: "missing session", edit: func(s *Session) { s.ID++ }},
		{name: "wrong project", edit: func(s *Session) { s.ProjectID++ }},
		{name: "wrong branch", edit: func(s *Session) { s.Branch = "other" }},
		{name: "not ended", edit: func(s *Session) { s.State = StateActive }},
		{name: "missing end time", edit: func(s *Session) { s.EndTime = nil }},
		{name: "active slice", edit: func(s *Session) { now := time.Now(); s.CurrentSliceStart = &now }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := newTestSession(t, database, project, "feature/test", StateActive)
			ended := endedSession(s)
			tt.edit(ended)
			if err := database.CompleteSession(s, ended); err == nil {
				t.Fatal("invalid completion succeeded, want error")
			}
			active, err := database.GetActiveSession(project.ID)
			if err != nil || active == nil || active.ID != s.ID {
				t.Fatalf("original active session lost: %#v, err = %v", active, err)
			}
		})
	}
	database := newTestDB(t)
	if err := database.CompleteSession(nil, nil); err == nil {
		t.Fatal("nil completion succeeded, want error")
	}
}

func TestCompleteSessionRejectsSnapshotBeforePause(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	started := trackedTestSession(t, database, project)
	checkpoint := testEndWorkflow(started)
	if err := database.CreateEndWorkflow(checkpoint); err != nil {
		t.Fatal(err)
	}
	staleEnd := endedSession(started)
	paused := pausedJournalSession(started, started.StartTime.Add(time.Minute))
	if err := database.TransitionSession(started, paused); err != nil {
		t.Fatal(err)
	}
	if err := database.CompleteSession(started, staleEnd); err == nil {
		t.Fatal("completion from the pre-pause snapshot succeeded")
	}
	current, err := database.GetSessionByID(started.ID)
	if err != nil || current == nil || !sameSession(current, paused) {
		t.Fatalf("paused record changed: %#v, error = %v", current, err)
	}
	events, err := database.ListSessionEvents(started.ID)
	if err != nil || len(events) != 2 || events[1].Kind != EventSessionPaused {
		t.Fatalf("stale completion journaled: %#v, error = %v", events, err)
	}
	saved, err := database.GetEndWorkflow(started.ID)
	if err != nil || !reflect.DeepEqual(saved, checkpoint) {
		t.Fatalf("checkpoint changed: %#v, error = %v", saved, err)
	}
}
