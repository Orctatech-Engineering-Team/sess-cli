package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

func TestResolveStartProjectUsesDetectedBaseBranchForNewProject(t *testing.T) {
	repoDir := newClonedGitRepo(t, "master")
	database := newStartTestDB(t)
	manager := session.NewManager(database)

	project, baseBranch, err := resolveStartProject(context.Background(), manager, database, repoDir)
	if err != nil {
		t.Fatalf("resolveStartProject returned error: %v", err)
	}

	if baseBranch != "master" {
		t.Fatalf("baseBranch = %q, want %q", baseBranch, "master")
	}
	if project.BaseBranch != "master" {
		t.Fatalf("project.BaseBranch = %q, want %q", project.BaseBranch, "master")
	}
}

func TestResolveStartProjectRepairsInvalidStoredBaseBranch(t *testing.T) {
	repoDir := newClonedGitRepo(t, "master")
	database := newStartTestDB(t)
	manager := session.NewManager(database)

	project, err := manager.InitializeProject(repoDir, "dev")
	if err != nil {
		t.Fatalf("InitializeProject returned error: %v", err)
	}
	if project.BaseBranch != "dev" {
		t.Fatalf("project.BaseBranch = %q, want %q before repair", project.BaseBranch, "dev")
	}

	resolvedProject, baseBranch, err := resolveStartProject(context.Background(), manager, database, repoDir)
	if err != nil {
		t.Fatalf("resolveStartProject returned error: %v", err)
	}

	if baseBranch != "master" {
		t.Fatalf("baseBranch = %q, want %q", baseBranch, "master")
	}
	if resolvedProject.BaseBranch != "master" {
		t.Fatalf("resolvedProject.BaseBranch = %q, want %q", resolvedProject.BaseBranch, "master")
	}

	persisted, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID returned error: %v", err)
	}
	if persisted == nil {
		t.Fatal("persisted project = nil, want project")
	}
	if persisted.BaseBranch != "master" {
		t.Fatalf("persisted.BaseBranch = %q, want %q", persisted.BaseBranch, "master")
	}
}

func newStartTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "sess.db"))
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

func newClonedGitRepo(t *testing.T, defaultBranch string) string {
	t.Helper()

	root := t.TempDir()
	seedDir := filepath.Join(root, "seed")
	originDir := filepath.Join(root, "origin.git")
	repoDir := filepath.Join(root, "repo")

	runGit(t, "", "init", "-b", defaultBranch, seedDir)
	runGit(t, seedDir, "config", "user.name", "SESS Test")
	runGit(t, seedDir, "config", "user.email", "sess-test@example.com")

	readmePath := filepath.Join(seedDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("seed\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	runGit(t, seedDir, "add", "README.md")
	runGit(t, seedDir, "commit", "-m", "Initial commit")
	runGit(t, "", "clone", "--bare", seedDir, originDir)
	runGit(t, "", "clone", originDir, repoDir)

	return repoDir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}

	return string(out)
}
