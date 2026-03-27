package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

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

	var count int
	if err := database.conn.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('sessions')
		WHERE name = 'current_slice_start'
	`).Scan(&count); err != nil {
		t.Fatalf("query current_slice_start column: %v", err)
	}
	if count != 1 {
		t.Fatalf("current_slice_start column count = %d, want 1", count)
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
