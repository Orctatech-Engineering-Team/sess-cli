//go:build integration

package tui

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

func TestEndCheckpointRestoresPRDetailsAfterRestart(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	failure := filepath.Join(ghDir, "fail-create")
	writeEndTestFile(t, ghDir, "fail-create", "fail\n")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "create pull request") {
		t.Fatalf("first attempt = %v, want PR failure", err)
	}
	if err := os.Remove(failure); err != nil {
		t.Fatal(err)
	}
	reopened := reopenEndCheckpointDB(t, database, dbPath)
	retry := endWorkflowConfig{cwd: cfg.cwd, project: cfg.project, session: cfg.session}
	prepared, cancelled, err := prepareEndConfig(t.Context(), reopened, retry)
	if err != nil || cancelled || prepared.prTitle != cfg.prTitle || prepared.prBody != cfg.prBody {
		t.Fatalf("restored config = %#v, cancelled = %t, error = %v", prepared, cancelled, err)
	}
	result, err := executeEndWorkflow(t.Context(), reopened, retry, discardEndTestMessage)
	if err != nil {
		t.Fatalf("retry after database reopen: %v", err)
	}
	args, err := os.ReadFile(filepath.Join(ghDir, "last-create-args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), cfg.prTitle) || !strings.Contains(string(args), cfg.prBody) {
		t.Fatalf("PR arguments = %q, want original title/body", args)
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
}

func TestEndCheckpointReconcilesPRStateAfterRestart(t *testing.T) {
	for _, state := range []string{"MERGED", "CLOSED"} {
		t.Run(state, func(t *testing.T) {
			cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
			endTestGit(t, cfg.cwd, "checkout", "main")
			writeEndTestFile(t, cfg.cwd, "base-only.txt", "base\n")
			endTestGit(t, cfg.cwd, "add", "base-only.txt")
			endTestGit(t, cfg.cwd, "commit", "-m", "Base file")
			endTestGit(t, cfg.cwd, "push", "origin", "main")
			endTestGit(t, cfg.cwd, "checkout", cfg.session.Branch)
			endTestGit(t, cfg.cwd, "merge", "--no-edit", "main")
			endTestGit(t, cfg.cwd, "rm", "base-only.txt")
			endTestGit(t, cfg.cwd, "commit", "-m", "Remove base file")
			writeEndTestFile(t, ghDir, "block-checkout", "block\n")
			_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
			if err == nil || !strings.Contains(err.Error(), "switch back") {
				t.Fatalf("first attempt = %v, want checkout failure", err)
			}
			if err := os.Remove(filepath.Join(cfg.cwd, "base-only.txt")); err != nil {
				t.Fatal(err)
			}
			writeEndTestFile(t, ghDir, "pr-state", state)
			reopened := reopenEndCheckpointDB(t, database, dbPath)
			retry := endWorkflowConfig{cwd: cfg.cwd, project: cfg.project, session: cfg.session}
			result, err := executeEndWorkflow(t.Context(), reopened, retry, discardEndTestMessage)
			if state == "CLOSED" {
				if err == nil || !strings.Contains(err.Error(), "closed without merging") {
					t.Fatalf("retry = %v, want closed PR recovery error", err)
				}
				assertEndTestSessionOpen(t, session.NewManager(reopened), cfg.session)
			} else {
				if err != nil {
					t.Fatalf("retry after merge: %v", err)
				}
				assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
			}
			creates, err := os.ReadFile(filepath.Join(ghDir, "creates"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(creates), "create\n") != 1 {
				t.Fatalf("duplicate PR creation: %q", creates)
			}
		})
	}
}

func TestEndCheckpointSaveFailurePreventsGitMutation(t *testing.T) {
	cfg, database, dbPath, _ := newEndCheckpointFixture(t)
	writeEndTestFile(t, cfg.cwd, "work.txt", "uncommitted\n")
	cfg.commitMessage = "Must not commit"
	head := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	checkpointTestSQL(t, dbPath, `
		CREATE TRIGGER fail_checkpoint BEFORE INSERT ON end_workflows
		BEGIN SELECT RAISE(ABORT, 'checkpoint unavailable'); END
	`)
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "save end checkpoint before Git operations") {
		t.Fatalf("error = %v, want checkpoint write failure", err)
	}
	if endTestGit(t, cfg.cwd, "rev-parse", "HEAD") != head {
		t.Fatal("work committed without a durable checkpoint")
	}
	assertEndTestSessionOpen(t, session.NewManager(database), cfg.session)
	if refs := endTestGit(t, cfg.cwd, "ls-remote", "--heads", "origin", cfg.session.Branch); refs != "" {
		t.Fatalf("branch pushed without checkpoint: %s", refs)
	}
}

func TestEndCheckpointSkipsCommittedWorkAndPreservesBaseBranch(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	writeEndTestFile(t, cfg.cwd, "work.txt", "commit once\n")
	cfg.commitMessage = "Saved commit message"
	writeEndTestFile(t, ghDir, "fail-create", "fail\n")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil {
		t.Fatal("expected PR failure")
	}
	head := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(ghDir, "fail-create")); err != nil {
		t.Fatal(err)
	}
	reopened := reopenEndCheckpointDB(t, database, dbPath)
	project := *cfg.project
	project.BaseBranch = "different-base"
	retry := endWorkflowConfig{cwd: cfg.cwd, project: &project, session: cfg.session}
	result, err := executeEndWorkflow(t.Context(), reopened, retry, discardEndTestMessage)
	if err != nil {
		t.Fatal(err)
	}
	if endTestGit(t, cfg.cwd, "rev-parse", "refs/heads/"+cfg.session.Branch) != head {
		t.Fatal("retry changed the recorded session head")
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
}

func TestEndCheckpointRejectsNewCommitsAfterPush(t *testing.T) {
	cfg, database, _, ghDir := newEndCheckpointFixture(t)
	writeEndTestFile(t, ghDir, "fail-create", "fail\n")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil {
		t.Fatal("expected PR failure")
	}
	writeEndTestFile(t, cfg.cwd, "work.txt", "new work\n")
	endTestGit(t, cfg.cwd, "commit", "-am", "New work")
	if err := os.Remove(filepath.Join(ghDir, "fail-create")); err != nil {
		t.Fatal(err)
	}
	_, err = executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "branch changed since end checkpoint") {
		t.Fatalf("error = %v, want changed branch error", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(database), cfg.session)
}

func TestEndCheckpointPreparedWithoutPRRejectsNewCommit(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	endTestGit(t, cfg.cwd, "reset", "--hard", "main")
	cfg.endWithoutPR = true
	cfg.commitMessage = "Matching but unexpected work"
	head := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	checkpointTestSQL(t, dbPath, `
		CREATE TRIGGER fail_checkout_checkpoint BEFORE UPDATE ON end_workflows
		WHEN NEW.phase = 'base_checked_out'
		BEGIN SELECT RAISE(ABORT, 'checkpoint unavailable'); END
	`)
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "save end checkpoint "+db.EndBaseCheckedOut) {
		t.Fatalf("first attempt = %v, want checkout checkpoint failure", err)
	}
	checkpoint := requireEndTestCheckpoint(t, database, cfg.session.ID, db.EndPrepared)
	if checkpoint.Head != head || !checkpoint.EndWithoutPR {
		t.Fatalf("checkpoint = %#v, want original no-PR head %s", checkpoint, head)
	}
	checkpointTestSQL(t, dbPath, "DROP TRIGGER fail_checkout_checkpoint")
	endTestGit(t, cfg.cwd, "checkout", cfg.session.Branch)
	writeEndTestFile(t, cfg.cwd, "work.txt", "new work\n")
	endTestGit(t, cfg.cwd, "commit", "-am", cfg.commitMessage)
	newHead := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")

	reopened := reopenEndCheckpointDB(t, database, dbPath)
	_, err = executeEndWorkflow(t.Context(), reopened, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "branch changed since no-PR end checkpoint") {
		t.Fatalf("retry = %v, want changed no-PR branch error", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(reopened), cfg.session)
	requireEndTestCheckpoint(t, reopened, cfg.session.ID, db.EndPrepared)
	if endTestGit(t, cfg.cwd, "rev-parse", "HEAD") != newHead {
		t.Fatal("rejected retry changed the new commit")
	}
	if _, err := os.Stat(filepath.Join(ghDir, "creates")); !os.IsNotExist(err) {
		t.Fatalf("no-PR retry attempted PR creation: %v", err)
	}
}

func TestEndCheckpointCommittedRejectsUnrelatedCommitAfterReset(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	checkpointTestSQL(t, dbPath, `
		CREATE TRIGGER fail_rebase_checkpoint BEFORE UPDATE ON end_workflows
		WHEN NEW.phase = 'rebased'
		BEGIN SELECT RAISE(ABORT, 'checkpoint unavailable'); END
	`)
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "save end checkpoint "+db.EndRebased) {
		t.Fatalf("first attempt = %v, want rebase checkpoint failure", err)
	}
	checkpoint := requireEndTestCheckpoint(t, database, cfg.session.ID, db.EndCommitted)
	if checkpoint.RebaseTarget == "" {
		t.Fatal("committed checkpoint has no saved rebase target")
	}
	checkpointTestSQL(t, dbPath, "DROP TRIGGER fail_rebase_checkpoint")
	endTestGit(t, cfg.cwd, "reset", "--hard", checkpoint.RebaseTarget)
	writeEndTestFile(t, cfg.cwd, "unrelated.txt", "unrelated work\n")
	endTestGit(t, cfg.cwd, "add", "unrelated.txt")
	endTestGit(t, cfg.cwd, "commit", "-m", "Unrelated commit")
	head := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	if ancestor := endTestGit(t, cfg.cwd, "merge-base", checkpoint.RebaseTarget, "HEAD"); ancestor != checkpoint.RebaseTarget {
		t.Fatalf("saved target %s is not an ancestor of unrelated head %s", checkpoint.RebaseTarget, head)
	}

	reopened := reopenEndCheckpointDB(t, database, dbPath)
	_, err = executeEndWorkflow(t.Context(), reopened, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "without a matching commit or rebase record") {
		t.Fatalf("retry = %v, want unrelated history rejection", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(reopened), cfg.session)
	requireEndTestCheckpoint(t, reopened, cfg.session.ID, db.EndCommitted)
	if endTestGit(t, cfg.cwd, "rev-parse", "HEAD") != head {
		t.Fatal("rejected retry changed unrelated history")
	}
	if refs := endTestGit(t, cfg.cwd, "ls-remote", "--heads", "origin", cfg.session.Branch); refs != "" {
		t.Fatalf("rejected retry pushed unrelated history: %s", refs)
	}
	if _, err := os.Stat(filepath.Join(ghDir, "creates")); !os.IsNotExist(err) {
		t.Fatalf("rejected retry attempted PR creation: %v", err)
	}
}

func TestEndCheckpointPreparedResumesMatchingInterruptedCommit(t *testing.T) {
	cfg, database, dbPath, _ := newEndCheckpointFixture(t)
	head := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	writeEndTestFile(t, cfg.cwd, "work.txt", "commit once\n")
	cfg.commitMessage = "Interrupted matching commit"
	checkpointTestSQL(t, dbPath, `
		CREATE TRIGGER fail_commit_checkpoint BEFORE UPDATE ON end_workflows
		WHEN NEW.phase = 'committed'
		BEGIN SELECT RAISE(ABORT, 'checkpoint unavailable'); END
	`)
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "save end checkpoint "+db.EndCommitted) {
		t.Fatalf("first attempt = %v, want commit checkpoint failure", err)
	}
	checkpoint := requireEndTestCheckpoint(t, database, cfg.session.ID, db.EndPrepared)
	committedHead := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	if checkpoint.Head != head || committedHead == head ||
		endTestGit(t, cfg.cwd, "rev-parse", "HEAD^") != head {
		t.Fatalf("checkpoint head = %s, committed head = %s, want one commit after %s", checkpoint.Head, committedHead, head)
	}
	checkpointTestSQL(t, dbPath, "DROP TRIGGER fail_commit_checkpoint")

	reopened := reopenEndCheckpointDB(t, database, dbPath)
	result, err := executeEndWorkflow(t.Context(), reopened, endWorkflowConfig{
		cwd: cfg.cwd, project: cfg.project, session: cfg.session,
	}, discardEndTestMessage)
	if err != nil {
		t.Fatalf("retry interrupted commit: %v", err)
	}
	if got := endTestGit(t, cfg.cwd, "rev-parse", cfg.session.Branch); got != committedHead {
		t.Fatalf("retry changed committed head from %s to %s", committedHead, got)
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
}

func TestEndCheckpointCommittedResumesCompletedRebaseWithSavedTarget(t *testing.T) {
	cfg, database, dbPath, _ := newEndCheckpointFixture(t)
	originalHead := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	endTestGit(t, cfg.cwd, "checkout", "main")
	writeEndTestFile(t, cfg.cwd, "base.txt", "first base update\n")
	endTestGit(t, cfg.cwd, "add", "base.txt")
	endTestGit(t, cfg.cwd, "commit", "-m", "Advance base")
	target := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	endTestGit(t, cfg.cwd, "push", "origin", "main")
	endTestGit(t, cfg.cwd, "checkout", cfg.session.Branch)
	checkpointTestSQL(t, dbPath, `
		CREATE TRIGGER fail_rebase_checkpoint BEFORE UPDATE ON end_workflows
		WHEN NEW.phase = 'rebased'
		BEGIN SELECT RAISE(ABORT, 'checkpoint unavailable'); END
	`)
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "save end checkpoint "+db.EndRebased) {
		t.Fatalf("first attempt = %v, want rebase checkpoint failure", err)
	}
	checkpoint := requireEndTestCheckpoint(t, database, cfg.session.ID, db.EndCommitted)
	rebasedHead := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	if checkpoint.Head != originalHead || checkpoint.RebaseTarget != target || rebasedHead == originalHead {
		t.Fatalf("checkpoint = %#v, rebased head = %s, want completed rebase from %s onto %s", checkpoint, rebasedHead, originalHead, target)
	}
	log := endTestGit(t, cfg.cwd, "reflog", "show", "-2", "--format=%H%n%gs", "refs/heads/"+cfg.session.Branch)
	wantLog := rebasedHead + "\nrebase (finish): refs/heads/" + cfg.session.Branch + " onto " + target + "\n" + originalHead + "\n"
	if !strings.HasPrefix(log, wantLog) {
		t.Fatalf("branch reflog = %q, want completed rebase evidence %q", log, wantLog)
	}
	checkpointTestSQL(t, dbPath, "DROP TRIGGER fail_rebase_checkpoint")
	endTestGit(t, cfg.cwd, "checkout", "main")
	writeEndTestFile(t, cfg.cwd, "base.txt", "second base update\n")
	endTestGit(t, cfg.cwd, "commit", "-am", "Advance base again")
	newTarget := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	endTestGit(t, cfg.cwd, "push", "origin", "main")
	endTestGit(t, cfg.cwd, "fetch", "origin", "main")
	endTestGit(t, cfg.cwd, "checkout", cfg.session.Branch)
	if newTarget == target || endTestGit(t, cfg.cwd, "rev-parse", "origin/main") != newTarget {
		t.Fatal("fixture did not advance the live rebase target")
	}

	reopened := reopenEndCheckpointDB(t, database, dbPath)
	result, err := executeEndWorkflow(t.Context(), reopened, endWorkflowConfig{
		cwd: cfg.cwd, project: cfg.project, session: cfg.session,
	}, discardEndTestMessage)
	if err != nil {
		t.Fatalf("retry completed rebase: %v", err)
	}
	if got := endTestGit(t, cfg.cwd, "rev-parse", cfg.session.Branch); got != rebasedHead {
		t.Fatalf("retry rebased again using moving base: head = %s, want %s", got, rebasedHead)
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
}

func TestEndCheckpointPushedRejectsChangedRemoteBeforePRCreation(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	writeEndTestFile(t, ghDir, "fail-create", "fail\n")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "create pull request") {
		t.Fatalf("first attempt = %v, want PR failure", err)
	}
	checkpoint := requireEndTestCheckpoint(t, database, cfg.session.ID, db.EndPushed)
	endTestGit(t, cfg.cwd, "checkout", "-b", "remote-update")
	writeEndTestFile(t, cfg.cwd, "work.txt", "remote-only work\n")
	endTestGit(t, cfg.cwd, "commit", "-am", "Remote update")
	remoteHead := endTestGit(t, cfg.cwd, "rev-parse", "HEAD")
	endTestGit(t, cfg.cwd, "push", "origin", "HEAD:refs/heads/"+cfg.session.Branch)
	endTestGit(t, cfg.cwd, "checkout", cfg.session.Branch)
	if endTestGit(t, cfg.cwd, "rev-parse", "HEAD") != checkpoint.Head || remoteHead == checkpoint.Head {
		t.Fatal("fixture did not change only the remote branch head")
	}
	if err := os.Remove(filepath.Join(ghDir, "fail-create")); err != nil {
		t.Fatal(err)
	}

	reopened := reopenEndCheckpointDB(t, database, dbPath)
	_, err = executeEndWorkflow(t.Context(), reopened, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "remote branch differs from end checkpoint") {
		t.Fatalf("retry = %v, want changed remote rejection", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(reopened), cfg.session)
	requireEndTestCheckpoint(t, reopened, cfg.session.ID, db.EndPushed)
	calls, err := os.ReadFile(filepath.Join(ghDir, "creates"))
	if err != nil || strings.Count(string(calls), "create\n") != 1 {
		t.Fatalf("PR creation calls = %q, error = %v, want only initial failed creation", calls, err)
	}
	if refs := endTestGit(t, cfg.cwd, "ls-remote", "--heads", "origin", cfg.session.Branch); !strings.HasPrefix(refs, remoteHead+"\t") {
		t.Fatalf("rejected retry changed remote head: %s", refs)
	}
}

func requireEndTestCheckpoint(t *testing.T, database *db.DB, sessionID int64, phase string) *db.EndWorkflow {
	t.Helper()
	checkpoint, err := database.GetEndWorkflow(sessionID)
	if err != nil || checkpoint == nil || checkpoint.Phase != phase {
		t.Fatalf("checkpoint = %#v, error = %v, want phase %s", checkpoint, err, phase)
	}
	return checkpoint
}

func TestEndCheckpointCompletionFailureResumesOnlyFinalization(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	send := func(msg tea.Msg) {
		if line, ok := msg.(gitLineMsg); ok && string(line) == "Recording session completion" {
			checkpointTestSQL(t, dbPath, `
				CREATE TRIGGER fail_complete BEFORE UPDATE ON projects
				BEGIN SELECT RAISE(ABORT, 'project unavailable'); END
			`)
		}
	}
	_, err := executeEndWorkflow(t.Context(), database, cfg, send)
	if err == nil || !strings.Contains(err.Error(), "checkpoint retained for retry") {
		t.Fatalf("error = %v, want atomic completion failure", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(database), cfg.session)
	checkpoint, err := database.GetEndWorkflow(cfg.session.ID)
	if err != nil || checkpoint == nil || checkpoint.Phase != db.EndBaseCheckedOut {
		t.Fatalf("checkpoint = %#v, error = %v, want base checkout recorded", checkpoint, err)
	}
	checkpointTestSQL(t, dbPath, "DROP TRIGGER fail_complete")
	reopened := reopenEndCheckpointDB(t, database, dbPath)
	endTestGit(t, cfg.cwd, "branch", "-D", cfg.session.Branch)
	send = func(msg tea.Msg) {
		if line, ok := msg.(gitLineMsg); ok {
			for _, prefix := range []string{"Fetching ", "Rebasing ", "Pushing ", "Creating "} {
				if strings.HasPrefix(string(line), prefix) {
					t.Errorf("repeated completed operation: %s", line)
				}
			}
		}
	}
	result, err := executeEndWorkflow(t.Context(), reopened, endWorkflowConfig{
		cwd: cfg.cwd, project: cfg.project, session: cfg.session,
	}, send)
	if err != nil {
		t.Fatal(err)
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
	checkpoint, err = reopened.GetEndWorkflow(cfg.session.ID)
	if err != nil || checkpoint != nil {
		t.Fatalf("completed checkpoint = %#v, error = %v, want deleted", checkpoint, err)
	}
	creates, err := os.ReadFile(filepath.Join(ghDir, "creates"))
	if err != nil || strings.Count(string(creates), "create\n") != 1 {
		t.Fatalf("PR creation calls = %q, error = %v", creates, err)
	}
}

func TestEndCheckpointRecoversMergedPRAfterLostCreateResponse(t *testing.T) {
	cfg, database, dbPath, ghDir := newEndCheckpointFixture(t)
	writeEndTestFile(t, ghDir, "fail-after-create", "fail\n")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil {
		t.Fatal("expected lost response")
	}
	writeEndTestFile(t, ghDir, "pr-state", "MERGED")
	endTestGit(t, cfg.cwd, "checkout", "main")
	endTestGit(t, cfg.cwd, "branch", "-D", cfg.session.Branch)
	reopened := reopenEndCheckpointDB(t, database, dbPath)
	result, err := executeEndWorkflow(t.Context(), reopened, endWorkflowConfig{
		cwd: cfg.cwd, project: cfg.project, session: cfg.session,
	}, discardEndTestMessage)
	if err != nil {
		t.Fatal(err)
	}
	assertEndTestCompleted(t, cfg.cwd, session.NewManager(reopened), result)
}

func checkpointTestSQL(t *testing.T, path, query string) {
	t.Helper()
	connection, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(query); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
}

func newEndCheckpointFixture(t *testing.T) (endWorkflowConfig, *db.DB, string, string) {
	t.Helper()
	repo, _, _, _ := newEndTestRepo(t)
	writeEndTestFile(t, repo, "work.txt", "feature\n")
	endTestGit(t, repo, "commit", "-am", "Feature")
	attachEndTestOrigin(t, repo)
	ghDir := installEndTestGH(t)
	dbPath := filepath.Join(t.TempDir(), "checkpoint.db")
	database, err := db.Open(dbPath)
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
	sess, err := mgr.StartSession(project.ID, "feature/test", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	cfg := endWorkflowConfig{
		cwd: repo, project: project, session: sess,
		prTitle: "Saved title 100%", prBody: "## Summary\nSaved work\n\n## Testing\nChecked",
	}
	return cfg, database, dbPath, ghDir
}

func reopenEndCheckpointDB(t *testing.T, database *db.DB, path string) *db.DB {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	return reopened
}
