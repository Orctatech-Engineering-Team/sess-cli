//go:build integration

package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRunEndTUIRejectsUnfinishedRebase(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	original, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	createEndTestConflict(t, repo)
	head := endTestGit(t, repo, "rev-parse", "HEAD")
	status := endTestGit(t, repo, "status", "--porcelain")

	err = RunEndTUI(repo, database)
	if err == nil || !strings.Contains(err.Error(), "git rebase --continue") {
		t.Fatalf("RunEndTUI error = %v, want rebase recovery instructions", err)
	}
	if got := endTestGit(t, repo, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD changed from %s to %s", head, got)
	}
	if got := endTestGit(t, repo, "status", "--porcelain"); got != status {
		t.Fatalf("worktree changed from %q to %q", status, got)
	}
	current, err := mgr.GetActiveSession(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != original.ID || current.State != original.State {
		t.Fatalf("session = %#v, want original open session", current)
	}
}

func TestExecuteEndWorkflowRejectsUnfinishedRebaseBeforeCommit(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	createEndTestConflict(t, repo)
	head := endTestGit(t, repo, "rev-parse", "HEAD")
	status := endTestGit(t, repo, "status", "--porcelain")
	_, err = executeEndWorkflow(t.Context(), database, endWorkflowConfig{
		cwd:           repo,
		project:       project,
		session:       sess,
		commitMessage: "Must not commit conflicts",
	}, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "git rebase --continue") {
		t.Fatalf("error = %v, want recovery instructions", err)
	}
	if endTestGit(t, repo, "rev-parse", "HEAD") != head {
		t.Fatal("HEAD changed while rebase was unfinished")
	}
	if endTestGit(t, repo, "status", "--porcelain") != status {
		t.Fatal("index or worktree changed while rebase was unfinished")
	}
	assertEndTestSessionOpen(t, mgr, sess)
}

func TestExecuteEndWorkflowRetryAfterRebaseResolution(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	createEndTestConflict(t, repo)
	endTestGit(t, repo, "rebase", "--abort")
	attachEndTestOrigin(t, repo)
	installEndTestGH(t)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	cfg := endWorkflowConfig{
		cwd: repo, project: project, session: sess,
		prTitle: "Feature", prBody: "Summary",
	}

	_, err = executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "git rebase --continue") {
		t.Fatalf("first attempt error = %v, want recovery instructions", err)
	}
	assertEndTestSessionOpen(t, mgr, sess)
	writeEndTestFile(t, repo, "work.txt", "resolved\n")
	endTestGit(t, repo, "add", "work.txt")
	endTestGit(t, repo, "rebase", "--continue")

	result, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	assertEndTestCompleted(t, repo, mgr, result)
}

func TestExecuteEndWorkflowRetryAfterHandoffFailure(t *testing.T) {
	tests := []struct {
		name        string
		failure     string
		wantError   string
		wantCreates int
		paused      bool
	}{
		{name: "push rejection", failure: "push", wantError: "push branch", wantCreates: 1},
		{name: "PR creation failure", failure: "fail-create", wantError: "create pull request", wantCreates: 2},
		{
			name: "PR created but response lost", failure: "fail-after-create",
			wantError: "create pull request", wantCreates: 1,
		},
		{
			name: "checkout failure after PR", failure: "block-checkout",
			wantError: "switch back", wantCreates: 1, paused: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, database, mgr, project := newEndTestRepo(t)
			endTestGit(t, repo, "checkout", "main")
			writeEndTestFile(t, repo, "base-only.txt", "base\n")
			endTestGit(t, repo, "add", "base-only.txt")
			endTestGit(t, repo, "commit", "-m", "Base file")
			endTestGit(t, repo, "checkout", "feature/test")
			endTestGit(t, repo, "merge", "--ff-only", "main")
			endTestGit(t, repo, "rm", "base-only.txt")
			writeEndTestFile(t, repo, "work.txt", "feature\n")
			endTestGit(t, repo, "commit", "-am", "Feature")
			attachEndTestOrigin(t, repo)
			ghDir := installEndTestGH(t)
			sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
			if err != nil {
				t.Fatal(err)
			}
			if tt.paused {
				sess, err = mgr.PauseSession(project.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			failurePath := filepath.Join(ghDir, tt.failure)
			if tt.failure == "push" {
				failurePath = filepath.Join(repo, ".git", "hooks", "pre-push")
			}
			if err := os.WriteFile(failurePath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			cfg := endWorkflowConfig{
				cwd: repo, project: project, session: sess,
				prTitle: "Feature", prBody: "Summary",
			}
			_, err = executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("first attempt error = %v, want %q", err, tt.wantError)
			}
			assertEndTestSessionOpen(t, mgr, sess)
			if err := os.Remove(failurePath); err != nil {
				t.Fatal(err)
			}
			if tt.failure == "block-checkout" {
				if err := os.Remove(filepath.Join(repo, "base-only.txt")); err != nil {
					t.Fatal(err)
				}
			}
			if tt.wantCreates == 1 && tt.failure != "push" {
				cfg.prTitle = ""
				cfg.prBody = ""
			}

			result, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
			if err != nil {
				t.Fatalf("retry: %v", err)
			}
			assertEndTestCompleted(t, repo, mgr, result)
			calls, err := os.ReadFile(filepath.Join(ghDir, "creates"))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(calls), "create\n"); got != tt.wantCreates {
				t.Fatalf("PR creation attempts = %d, want %d", got, tt.wantCreates)
			}
		})
	}
}

func TestExecuteEndWorkflowMissingPRDetailsFailsClosed(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	writeEndTestFile(t, repo, "work.txt", "feature\n")
	endTestGit(t, repo, "commit", "-am", "Feature")
	attachEndTestOrigin(t, repo)
	ghDir := installEndTestGH(t)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeEndWorkflow(t.Context(), database, endWorkflowConfig{
		cwd: repo, project: project, session: sess,
	}, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "enter PR details") {
		t.Fatalf("error = %v, want missing PR details error", err)
	}
	assertEndTestSessionOpen(t, mgr, sess)
	if _, err := os.Stat(filepath.Join(ghDir, "creates")); !os.IsNotExist(err) {
		t.Fatalf("PR creation was attempted: %v", err)
	}
}

func TestExecuteEndWorkflowCancelledBeforeMutation(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	writeEndTestFile(t, repo, "work.txt", "uncommitted\n")
	head := endTestGit(t, repo, "rev-parse", "HEAD")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = executeEndWorkflow(ctx, database, endWorkflowConfig{
		cwd: repo, project: project, session: sess, commitMessage: "Must not commit",
	}, discardEndTestMessage)
	if err == nil {
		t.Fatal("cancelled workflow succeeded")
	}
	if endTestGit(t, repo, "rev-parse", "HEAD") != head {
		t.Fatal("cancelled workflow committed changes")
	}
	assertEndTestSessionOpen(t, mgr, sess)
}

func TestExecuteEndWorkflowCancellationStopsBeforePush(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	writeEndTestFile(t, repo, "work.txt", "feature\n")
	endTestGit(t, repo, "commit", "-am", "Feature")
	attachEndTestOrigin(t, repo)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	send := func(msg tea.Msg) {
		if line, ok := msg.(gitLineMsg); ok && strings.HasPrefix(string(line), "Pushing ") {
			cancel()
		}
	}
	_, err = executeEndWorkflow(ctx, database, endWorkflowConfig{
		cwd: repo, project: project, session: sess, prTitle: "Feature", prBody: "Summary",
	}, send)
	if err == nil {
		t.Fatal("cancelled workflow succeeded")
	}
	assertEndTestSessionOpen(t, mgr, sess)
	if refs := endTestGit(t, repo, "ls-remote", "--heads", "origin", "feature/test"); refs != "" {
		t.Fatalf("cancelled workflow pushed branch: %s", refs)
	}
}

func TestExecuteEndWorkflowWithoutPR(t *testing.T) {
	repo, database, mgr, project := newEndTestRepo(t)
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	result, err := executeEndWorkflow(t.Context(), database, endWorkflowConfig{
		cwd: repo, project: project, session: sess, endWithoutPR: true,
	}, discardEndTestMessage)
	if err != nil {
		t.Fatal(err)
	}
	if !result.endWithoutPR || result.pr != nil || result.session.State != db.StateEnded {
		t.Fatalf("result = %#v, want ended session without PR", result)
	}
	if branch := endTestGit(t, repo, "branch", "--show-current"); branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}
	current, err := mgr.GetActiveSession(project.ID)
	if err != nil || current != nil {
		t.Fatalf("active session = %#v, error = %v, want nil", current, err)
	}
}

func TestExecuteEndWorkflowCancellationBeforeCompletion(t *testing.T) {
	for _, withPR := range []bool{false, true} {
		name := "without PR"
		if withPR {
			name = "with PR"
		}
		t.Run(name, func(t *testing.T) {
			repo, database, mgr, project := newEndTestRepo(t)
			if withPR {
				writeEndTestFile(t, repo, "work.txt", "feature\n")
				endTestGit(t, repo, "commit", "-am", "Feature")
				attachEndTestOrigin(t, repo)
				installEndTestGH(t)
			}
			sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			send := func(msg tea.Msg) {
				if line, ok := msg.(gitLineMsg); ok && string(line) == "Recording session completion" {
					cancel()
				}
			}
			_, err = executeEndWorkflow(ctx, database, endWorkflowConfig{
				cwd: repo, project: project, session: sess,
				endWithoutPR: !withPR, prTitle: "Feature", prBody: "Summary",
			}, send)
			if err == nil || !strings.Contains(err.Error(), "interrupted before session completion") {
				t.Fatalf("error = %v, want cancelled completion", err)
			}
			assertEndTestSessionOpen(t, mgr, sess)
			if branch := endTestGit(t, repo, "branch", "--show-current"); branch != "main" {
				t.Fatalf("branch = %q, want checkout completed before cancellation", branch)
			}
		})
	}
}

func assertEndTestSessionOpen(t *testing.T, mgr *session.Manager, original *db.Session) {
	t.Helper()
	current, err := mgr.GetActiveSession(original.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != original.ID || current.State != original.State {
		t.Fatalf("session = %#v, want original %s session", current, original.State)
	}
}

func assertEndTestCompleted(t *testing.T, repo string, mgr *session.Manager, result endWorkflowResult) {
	t.Helper()
	if result.session == nil || result.session.State != db.StateEnded {
		t.Fatalf("result session = %#v, want ended session", result.session)
	}
	current, err := mgr.GetActiveSession(result.session.ProjectID)
	if err != nil || current != nil {
		t.Fatalf("active session = %#v, error = %v, want nil", current, err)
	}
	history, err := mgr.GetSessionHistory(result.session.ProjectID, 1)
	if err != nil || len(history) != 1 {
		t.Fatalf("session history = %#v, error = %v", history, err)
	}
	if history[0].PRNumber == nil || *history[0].PRNumber != 42 || history[0].PRURL == "" {
		t.Fatalf("persisted PR metadata = %#v", history[0])
	}
	if branch := endTestGit(t, repo, "branch", "--show-current"); branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}
}

func attachEndTestOrigin(t *testing.T, repo string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	endTestGit(t, repo, "init", "--bare", remote)
	endTestGit(t, repo, "remote", "add", "origin", remote)
	endTestGit(t, repo, "push", "origin", "main")
}

func installEndTestGH(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gh fixture uses a POSIX shell")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
  "repo view")
    printf '{"nameWithOwner":"example/repo","url":"https://github.com/example/repo"}\n'
    ;;
  "pr list")
    if [ -f "$FAKE_GH_DIR/pr" ]; then
      state=OPEN
      if [ -f "$FAKE_GH_DIR/pr-state" ]; then
        state=$(cat "$FAKE_GH_DIR/pr-state")
      fi
      case "$*" in
        *"--state open"*)
          if [ "$state" != OPEN ]; then printf '[]\n'; exit 0; fi
          ;;
      esac
      if [ -f "$FAKE_GH_DIR/pr-head" ]; then
        head=$(cat "$FAKE_GH_DIR/pr-head")
      else
        head=$(git rev-parse refs/heads/feature/test)
      fi
      printf '[{"number":42,"url":"https://github.com/example/repo/pull/42","state":"%s","baseRefName":"main","headRefName":"feature/test","headRefOid":"%s","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"example"}}]\n' "$state" "$head"
    else
      printf '[]\n'
    fi
    ;;
  "pr view")
    if [ ! -f "$FAKE_GH_DIR/pr" ]; then exit 1; fi
    state=OPEN
    if [ -f "$FAKE_GH_DIR/pr-state" ]; then
      state=$(cat "$FAKE_GH_DIR/pr-state")
    fi
    if [ -f "$FAKE_GH_DIR/pr-head" ]; then
      head=$(cat "$FAKE_GH_DIR/pr-head")
    else
      head=$(git rev-parse refs/heads/feature/test)
    fi
    printf '{"number":42,"url":"https://github.com/example/repo/pull/42","state":"%s","baseRefName":"main","headRefName":"feature/test","headRefOid":"%s","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"example"}}\n' "$state" "$head"
    ;;
  "pr create")
    printf 'create\n' >> "$FAKE_GH_DIR/creates"
    printf '%s\n' "$@" > "$FAKE_GH_DIR/last-create-args"
    if [ -f "$FAKE_GH_DIR/fail-create" ]; then
      printf 'PR creation rejected\n' >&2
      exit 1
    fi
    touch "$FAKE_GH_DIR/pr"
    git rev-parse refs/heads/feature/test > "$FAKE_GH_DIR/pr-head"
    if [ -f "$FAKE_GH_DIR/fail-after-create" ]; then
      printf 'response lost\n' >&2
      exit 1
    fi
    if [ -f "$FAKE_GH_DIR/block-checkout" ]; then
      printf 'keep this untracked work\n' > base-only.txt
    fi
    printf 'https://github.com/example/repo/pull/42\n'
    ;;
  *)
    printf 'unexpected gh command\n' >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_GH_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func discardEndTestMessage(tea.Msg) {}

func newEndTestRepo(t *testing.T) (string, *db.DB, *session.Manager, *db.Project) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_EDITOR", "true")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	endTestGit(t, repo, "init", "-b", "main")
	endTestGit(t, repo, "config", "user.name", "SESS test")
	endTestGit(t, repo, "config", "user.email", "sess-test@example.invalid")
	writeEndTestFile(t, repo, "work.txt", "original\n")
	endTestGit(t, repo, "add", "work.txt")
	endTestGit(t, repo, "commit", "-m", "Initial")
	endTestGit(t, repo, "checkout", "-b", "feature/test")
	database, err := db.Open(filepath.Join(t.TempDir(), "sess.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	mgr := session.NewManager(database)
	project, err := mgr.InitializeProject(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	return repo, database, mgr, project
}

func createEndTestConflict(t *testing.T, repo string) {
	t.Helper()
	writeEndTestFile(t, repo, "work.txt", "feature\n")
	endTestGit(t, repo, "commit", "-am", "Feature")
	endTestGit(t, repo, "checkout", "main")
	writeEndTestFile(t, repo, "work.txt", "base\n")
	endTestGit(t, repo, "commit", "-am", "Base")
	endTestGit(t, repo, "checkout", "feature/test")
	cmd := exec.CommandContext(t.Context(), "git", "rebase", "main")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("rebase unexpectedly succeeded: %s", out)
	}
}

func endTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeEndTestFile(t *testing.T, repo, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}
