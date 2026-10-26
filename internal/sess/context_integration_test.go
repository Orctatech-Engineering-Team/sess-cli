//go:build integration

package sess

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

func TestContextCommandAvailability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		setup            func(*testing.T, string, string)
		wantAvailability string
		wantProject      bool
		wantSession      bool
		wantWarning      string
	}{
		{
			name:             "no repository",
			setup:            func(*testing.T, string, string) {},
			wantAvailability: "no_repository",
		},
		{
			name: "untracked project",
			setup: func(t *testing.T, cwd, _ string) {
				initContextTestRepo(t, cwd)
			},
			wantAvailability: "untracked_project",
		},
		{
			name: "idle project",
			setup: func(t *testing.T, cwd, path string) {
				initContextTestRepo(t, cwd)
				database := openContextTestDB(t, path)
				if _, err := session.NewManager(database).InitializeProject(cwd, "main"); err != nil {
					t.Fatal(err)
				}
				closeContextTestDB(t, database)
			},
			wantAvailability: "idle_project",
			wantProject:      true,
		},
		{
			name: "active session",
			setup: func(t *testing.T, cwd, path string) {
				initContextTestRepo(t, cwd)
				runContextTestGit(t, cwd, "checkout", "-b", "feature/context")
				database := openContextTestDB(t, path)
				manager := session.NewManager(database)
				project, err := manager.InitializeProject(cwd, "main")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := manager.StartSession(
					project.ID,
					"feature/context",
					"11",
					"Add read-only structured session context",
					"feature",
				); err != nil {
					t.Fatal(err)
				}
				closeContextTestDB(t, database)
			},
			wantAvailability: "session",
			wantProject:      true,
			wantSession:      true,
			wantWarning:      "protocol_history_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			dbPath := filepath.Join(t.TempDir(), "sess.db")
			tt.setup(t, cwd, dbPath)

			before, err := os.ReadFile(dbPath)
			dbExists := err == nil
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			output := executeContextTestCommand(t, cwd, dbPath)
			envelope := decodeContextTestOutput(t, output)

			if envelope.Data.Availability != tt.wantAvailability {
				t.Fatalf("availability = %q, want %q", envelope.Data.Availability, tt.wantAvailability)
			}
			if got := envelope.Data.Project != nil; got != tt.wantProject {
				t.Fatalf("project present = %v, want %v", got, tt.wantProject)
			}
			if got := envelope.Data.Session != nil; got != tt.wantSession {
				t.Fatalf("session present = %v, want %v", got, tt.wantSession)
			}
			assertContextTestWarning(t, envelope.Data.Warnings, tt.wantWarning)

			if !dbExists {
				if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
					t.Fatalf("context created database: %v", err)
				}
			} else {
				after, err := os.ReadFile(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(after, before) {
					t.Fatal("context changed database contents")
				}
			}
		})
	}
}

func TestContextCommandSessionContract(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	initContextTestRepo(t, cwd)
	runContextTestGit(t, cwd, "checkout", "-b", "feature/context")
	dbPath := filepath.Join(t.TempDir(), "sess.db")
	database := openContextTestDB(t, dbPath)
	manager := session.NewManager(database)
	project, err := manager.InitializeProject(cwd, "main")
	if err != nil {
		t.Fatal(err)
	}
	started, err := manager.StartSession(
		project.ID,
		"feature/context",
		"11",
		"Add read-only structured session context",
		"feature",
	)
	if err != nil {
		t.Fatal(err)
	}
	participant := db.Participant{
		ID: "participant:context", Kind: db.ParticipantAgent,
		Label: "context-agent", IdentitySource: "configured",
		CreatedAt: time.Date(2026, 10, 18, 10, 31, 0, 0, time.UTC),
	}
	if err := database.CreateParticipant(&participant); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateAttempt(&db.Attempt{
		ID: "attempt:context", SessionID: started.ID, Participant: participant,
		Role: "implementation", State: db.AttemptActive,
		StartedAt: participant.CreatedAt, UpdatedAt: participant.CreatedAt,
		StartRepositoryState: db.RepositoryState{
			Branch: "feature/context", Worktree: "clean", CapturedAt: participant.CreatedAt,
		},
		Claims: []db.AttemptClaim{{Type: "path", Value: "internal/sess/**", CreatedAt: participant.CreatedAt}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateCheck(&db.Check{
		ID: "check:context", AttemptID: "attempt:context", Name: "Go tests",
		ProcedureType: db.CheckCommand, ProcedureValue: "go test ./...",
		Outcome: db.CheckPassed, StartedAt: participant.CreatedAt,
		FinishedAt: participant.CreatedAt, RepositoryState: db.RepositoryState{
			Branch: "feature/context", Worktree: "clean", CapturedAt: participant.CreatedAt,
		},
		Reproducibility: "revision", Summary: "passed",
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateHandoff(&db.Handoff{
		ID: "handoff:context", SessionID: started.ID, AttemptID: "attempt:context",
		CreatedAt: participant.CreatedAt, RepositoryState: db.RepositoryState{
			Branch: "feature/context", Worktree: "clean", CapturedAt: participant.CreatedAt,
		},
		Completed: []string{"Implemented context integration."},
		Remaining: []string{"Review the next slice."}, Risks: []string{"None known."},
		NextAction: "Review the next slice.", CheckIDs: []string{"check:context"},
	}); err != nil {
		t.Fatal(err)
	}
	closeContextTestDB(t, database)
	indexPath := filepath.Join(cwd, ".git", "index")
	indexBefore, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	headBefore := runContextTestGit(t, cwd, "rev-parse", "HEAD")

	first := executeContextTestCommand(t, cwd, dbPath)
	second := executeContextTestCommand(t, cwd, dbPath)
	if first != second {
		t.Fatalf("context output is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	envelope := decodeContextTestOutput(t, first)
	if envelope.ProtocolVersion != contextProtocolVersion || envelope.Kind != "context" {
		t.Fatalf("protocol envelope = %q/%q", envelope.ProtocolVersion, envelope.Kind)
	}
	if envelope.GeneratedAt != "2026-10-18T10:30:00.123456789Z" {
		t.Fatalf("generated_at = %q", envelope.GeneratedAt)
	}
	if envelope.Data.Project == nil ||
		envelope.Data.Project.ID != contextRecordID("project", project.ID) ||
		envelope.Data.Project.Name != project.Name ||
		envelope.Data.Project.Path != cwd ||
		envelope.Data.Project.BaseBranch != "main" {
		t.Fatalf("project = %#v", envelope.Data.Project)
	}
	if envelope.Data.Session == nil ||
		envelope.Data.Session.ID != contextRecordID("session", started.ID) ||
		envelope.Data.Session.State != db.StateActive ||
		envelope.Data.Session.Branch != "feature/context" ||
		envelope.Data.Session.Issue == nil ||
		envelope.Data.Session.Issue.ID != "11" {
		t.Fatalf("session = %#v", envelope.Data.Session)
	}
	if envelope.Data.RepositoryState == nil ||
		envelope.Data.RepositoryState.Branch == nil ||
		*envelope.Data.RepositoryState.Branch != "feature/context" ||
		envelope.Data.RepositoryState.HeadOID == nil ||
		len(*envelope.Data.RepositoryState.HeadOID) != 40 ||
		envelope.Data.RepositoryState.Worktree != "clean" {
		t.Fatalf("repository state = %#v", envelope.Data.RepositoryState)
	}
	if len(envelope.Data.ActiveAttempts) != 1 ||
		envelope.Data.ActiveAttempts[0].ID != "attempt:attempt:context" ||
		envelope.Data.ActiveAttempts[0].Participant.ID != participant.ID ||
		len(envelope.Data.ActiveAttempts[0].Claims) != 1 ||
		len(envelope.Data.Checks) != 1 ||
		envelope.Data.Checks[0].ID != "check:context" ||
		envelope.Data.LatestHandoff == nil ||
		envelope.Data.LatestHandoff.ID != "handoff:handoff:context" {
		t.Fatalf("protocol history = attempts %#v, checks %#v, handoff %#v",
			envelope.Data.ActiveAttempts, envelope.Data.Checks, envelope.Data.LatestHandoff)
	}
	indexAfter, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(indexAfter, indexBefore) {
		t.Fatal("context changed the Git index")
	}
	if headAfter := runContextTestGit(t, cwd, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Fatalf("context changed HEAD from %q to %q", headBefore, headAfter)
	}
}

func TestContextCommandPausedSessionWithoutIssue(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	initContextTestRepo(t, cwd)
	runContextTestGit(t, cwd, "checkout", "-b", "feature/paused")
	dbPath := filepath.Join(t.TempDir(), "sess.db")
	database := openContextTestDB(t, dbPath)
	manager := session.NewManager(database)
	project, err := manager.InitializeProject(cwd, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.StartSession(project.ID, "feature/paused", "", "", "feature"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PauseSession(project.ID); err != nil {
		t.Fatal(err)
	}
	closeContextTestDB(t, database)

	envelope := decodeContextTestOutput(t, executeContextTestCommand(t, cwd, dbPath))
	if envelope.Data.Session == nil ||
		envelope.Data.Session.State != db.StatePaused ||
		envelope.Data.Session.Issue != nil {
		t.Fatalf("paused session = %#v", envelope.Data.Session)
	}
}

func TestContextCommandWarnsAboutUnbornHead(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	runContextTestGit(t, cwd, "init", "-b", "main")
	envelope := decodeContextTestOutput(
		t,
		executeContextTestCommand(t, cwd, filepath.Join(t.TempDir(), "missing.db")),
	)
	if envelope.Data.Availability != "untracked_project" ||
		envelope.Data.RepositoryState == nil ||
		envelope.Data.RepositoryState.HeadOID != nil {
		t.Fatalf("unborn repository context = %#v", envelope.Data)
	}
	assertContextTestWarning(t, envelope.Data.Warnings, "head_unavailable")
}

func TestContextCommandWarnsAboutSessionBranchMismatch(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	initContextTestRepo(t, cwd)
	runContextTestGit(t, cwd, "branch", "feature/context")
	dbPath := filepath.Join(t.TempDir(), "sess.db")
	database := openContextTestDB(t, dbPath)
	manager := session.NewManager(database)
	project, err := manager.InitializeProject(cwd, "main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.StartSession(project.ID, "feature/context", "", "", "feature"); err != nil {
		t.Fatal(err)
	}
	closeContextTestDB(t, database)

	envelope := decodeContextTestOutput(t, executeContextTestCommand(t, cwd, dbPath))
	assertContextTestWarning(t, envelope.Data.Warnings, "session_branch_mismatch")
}

func executeContextTestCommand(t *testing.T, cwd, dbPath string) string {
	t.Helper()

	var output bytes.Buffer
	cmd := newContextCommand(contextCommandDeps{
		getwd:  func() (string, error) { return cwd, nil },
		dbPath: func() (string, error) { return dbPath, nil },
		now: func() time.Time {
			return time.Date(2026, 10, 18, 10, 30, 0, 123456789, time.UTC)
		},
	})
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func decodeContextTestOutput(t *testing.T, output string) contextEnvelope {
	t.Helper()
	var envelope contextEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("decode context output: %v\n%s", err, output)
	}
	return envelope
}

func assertContextTestWarning(t *testing.T, warnings []contextWarning, want string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == want {
			return
		}
	}
	if want != "" {
		t.Fatalf("warnings = %#v, want %q", warnings, want)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func openContextTestDB(t *testing.T, path string) *db.DB {
	t.Helper()
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func closeContextTestDB(t *testing.T, database *db.DB) {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func initContextTestRepo(t *testing.T, dir string) {
	t.Helper()
	runContextTestGit(t, dir, "init", "-b", "main")
	runContextTestGit(t, dir, "config", "user.name", "SESS Test")
	runContextTestGit(t, dir, "config", "user.email", "sess@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runContextTestGit(t, dir, "add", "README.md")
	runContextTestGit(t, dir, "commit", "-m", "Initial commit")
}

func runContextTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
