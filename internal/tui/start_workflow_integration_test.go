//go:build integration

package tui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

func TestStartWorkflowCreatesBranchFromUpdatedBase(t *testing.T) {
	f := newStartWorkflowFixture(t)
	original := endTestGit(t, f.cfg.cwd, "rev-parse", "feature/test")
	advanceStartWorkflowOrigin(t, f.cfg.cwd)
	remoteHead := endTestGit(t, f.cfg.cwd, "rev-parse", "origin/main")
	endTestGit(t, f.cfg.cwd, "branch", "-f", "main", original)
	trace := filepath.Join(t.TempDir(), "git-trace")
	t.Setenv("GIT_TRACE", trace)
	var messages []tea.Msg
	var messagesMu sync.Mutex

	result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, func(msg tea.Msg) {
		messagesMu.Lock()
		defer messagesMu.Unlock()
		messages = append(messages, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStartWorkflowSession(t, f, result)
	if got := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD"); got != remoteHead {
		t.Fatalf("new branch head = %s, want updated base %s", got, remoteHead)
	}
	if got := endTestGit(t, f.cfg.cwd, "rev-parse", "feature/test"); got != original {
		t.Fatalf("original branch changed from %s to %s", original, got)
	}
	if len(messages) == 0 {
		t.Fatal("Git workflow did not stream any messages")
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	last := -1
	for _, command := range []string{
		"git checkout --no-overwrite-ignore main",
		"git fetch origin main",
		"git merge --ff-only --no-overwrite-ignore --no-autostash FETCH_HEAD",
		"git checkout --no-overwrite-ignore -b " + f.cfg.branch,
	} {
		next := strings.Index(string(data), command)
		if next <= last {
			t.Fatalf("missing or out-of-order command %q in trace:\n%s", command, data)
		}
		last = next
	}
}

func TestStartWorkflowPreservesIgnoredFilesDuringCheckout(t *testing.T) {
	for _, adopt := range []bool{false, true} {
		t.Run(fmt.Sprintf("adopt=%t", adopt), func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			source, target := "feature/test", "main"
			if adopt {
				source, target = "main", "feature/test"
				f.cfg.adopt, f.cfg.branch = true, target
			}
			endTestGit(t, f.cfg.cwd, "checkout", target)
			writeEndTestFile(t, f.cfg.cwd, "local.secret", "tracked target contents\n")
			endTestGit(t, f.cfg.cwd, "add", "local.secret")
			endTestGit(t, f.cfg.cwd, "commit", "-m", "Track target file")
			endTestGit(t, f.cfg.cwd, "checkout", source)
			writeEndTestFile(t, f.cfg.cwd, ".gitignore", "local.secret\n")
			endTestGit(t, f.cfg.cwd, "add", ".gitignore")
			endTestGit(t, f.cfg.cwd, "commit", "-m", "Ignore local file")
			writeEndTestFile(t, f.cfg.cwd, "local.secret", "precious ignored contents\n")
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)

			if result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage); err == nil || result != nil {
				t.Fatalf("checkout accepted ignored overwrite: result=%#v, err=%v", result, err)
			}
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
			data, err := os.ReadFile(filepath.Join(f.cfg.cwd, "local.secret"))
			if err != nil || string(data) != "precious ignored contents\n" {
				t.Fatalf("ignored file changed: %q, %v", data, err)
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
		})
	}
}

func TestStartWorkflowPreservesIgnoredFilesDuringBaseUpdate(t *testing.T) {
	f := newStartWorkflowFixture(t)
	endTestGit(t, f.cfg.cwd, "checkout", "main")
	writeEndTestFile(t, f.cfg.cwd, ".gitignore", "local.secret\n")
	endTestGit(t, f.cfg.cwd, "add", ".gitignore")
	endTestGit(t, f.cfg.cwd, "commit", "-m", "Ignore local file")
	base := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD")
	writeEndTestFile(t, f.cfg.cwd, "local.secret", "new remote contents\n")
	endTestGit(t, f.cfg.cwd, "add", "-f", "local.secret")
	endTestGit(t, f.cfg.cwd, "commit", "-m", "Track remote file")
	endTestGit(t, f.cfg.cwd, "push", "origin", "main")
	endTestGit(t, f.cfg.cwd, "checkout", "feature/test")
	endTestGit(t, f.cfg.cwd, "branch", "-f", "main", base)
	endTestGit(t, f.cfg.cwd, "checkout", "main")
	writeEndTestFile(t, f.cfg.cwd, "local.secret", "precious ignored contents\n")

	if result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage); err == nil || result != nil {
		t.Fatalf("base update accepted ignored overwrite: result=%#v, err=%v", result, err)
	}
	if got := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD"); got != base {
		t.Fatalf("failed update changed base head: %q, want %q", got, base)
	}
	data, err := os.ReadFile(filepath.Join(f.cfg.cwd, "local.secret"))
	if err != nil || string(data) != "precious ignored contents\n" {
		t.Fatalf("ignored file changed: %q, %v", data, err)
	}
	assertStartWorkflowCounts(t, f.path, 0, 0)
}

func TestStartWorkflowAdoptsBranchWithCollidingTag(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprintf("dirty=%t", dirty), func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.adopt, f.cfg.branch = true, "feature/test"
			endTestGit(t, f.cfg.cwd, "tag", f.cfg.branch)
			if dirty {
				makeStartWorkflowDirty(t, f.cfg.cwd)
			} else {
				endTestGit(t, f.cfg.cwd, "checkout", "main")
			}
			head := endTestGit(t, f.cfg.cwd, "rev-parse", "refs/heads/"+f.cfg.branch)
			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err != nil {
				t.Fatal(err)
			}
			assertStartWorkflowSession(t, f, result)
			if got := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD"); got != head {
				t.Fatalf("adopted head = %q, want %q", got, head)
			}
		})
	}
}

func TestStartWorkflowPersistenceFailureCanBeAdoptedAfterReopen(t *testing.T) {
	for _, table := range []string{"sessions", "session_events"} {
		t.Run(table, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			original := endTestGit(t, f.cfg.cwd, "rev-parse", "feature/test")
			checkpointTestSQL(t, f.path, "CREATE TRIGGER fail_start BEFORE INSERT ON "+table+
				" BEGIN SELECT RAISE(ABORT, 'start unavailable'); END")

			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err == nil || result != nil {
				t.Fatalf("failed save = %#v, %v; want error and no result", result, err)
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
			if got := endTestGit(t, f.cfg.cwd, "branch", "--show-current"); got != f.cfg.branch {
				t.Fatalf("branch after failed save = %q, want %q", got, f.cfg.branch)
			}
			if got := endTestGit(t, f.cfg.cwd, "rev-parse", "feature/test"); got != original {
				t.Fatalf("failed save changed original branch to %s", got)
			}

			makeStartWorkflowDirty(t, f.cfg.cwd)
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
			checkpointTestSQL(t, f.path, "DROP TRIGGER fail_start")
			f.reopen(t)
			f.cfg.adopt = true
			result, err = executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err != nil {
				t.Fatalf("adopt after actual database reopen: %v", err)
			}
			assertStartWorkflowSession(t, f, result)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)

			if _, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage); err == nil {
				t.Fatal("duplicate adoption succeeded")
			}
			assertStartWorkflowCounts(t, f.path, 1, 1)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
		})
	}
}

func TestStartWorkflowAdoptionPreservesHistoryAndWork(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		name := "clean switch"
		if dirty {
			name = "dirty current branch"
		}
		t.Run(name, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.branch = "feature/test"
			f.cfg.adopt = true
			writeEndTestFile(t, f.cfg.cwd, "work.txt", "existing feature commit\n")
			endTestGit(t, f.cfg.cwd, "commit", "-am", "Existing feature")
			head := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD")
			advanceStartWorkflowOrigin(t, f.cfg.cwd)
			if dirty {
				makeStartWorkflowDirty(t, f.cfg.cwd)
			} else {
				endTestGit(t, f.cfg.cwd, "checkout", "main")
			}
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
			trace := filepath.Join(t.TempDir(), "adopt-trace")
			t.Setenv("GIT_TRACE", trace)

			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err != nil {
				t.Fatal(err)
			}
			assertStartWorkflowSession(t, f, result)
			if got := endTestGit(t, f.cfg.cwd, "rev-parse", "HEAD"); got != head {
				t.Fatalf("adoption changed existing head from %s to %s", head, got)
			}
			if dirty {
				assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
			} else if got := endTestGit(t, f.cfg.cwd, "show-ref", "--heads"); got != before.refs {
				t.Fatalf("clean adoption changed local refs:\n%s", got)
			}
			data, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"git fetch", "git pull", "git reset", "git checkout -b", "git branch -f"} {
				if strings.Contains(string(data), command) {
					t.Errorf("adoption ran forbidden command %q:\n%s", command, data)
				}
			}
		})
	}
}

func TestStartWorkflowAdoptionSaveFailurePreservesDirtyBranch(t *testing.T) {
	for _, table := range []string{"sessions", "session_events"} {
		t.Run(table, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.branch, f.cfg.adopt = "feature/test", true
			makeStartWorkflowDirty(t, f.cfg.cwd)
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
			checkpointTestSQL(t, f.path, "CREATE TRIGGER fail_start BEFORE INSERT ON "+table+
				" BEGIN SELECT RAISE(ABORT, 'start unavailable'); END")
			if _, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage); err == nil {
				t.Fatal("adoption accepted failed persistence")
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
			checkpointTestSQL(t, f.path, "DROP TRIGGER fail_start")
			f.reopen(t)
			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err != nil {
				t.Fatal(err)
			}
			assertStartWorkflowSession(t, f, result)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
		})
	}
}

func TestStartWorkflowRejectsUnsafeRepositoryState(t *testing.T) {
	cases := []struct {
		name  string
		adopt bool
		setup func(*testing.T, *startWorkflowFixture)
	}{
		{"normal existing branch", false, func(t *testing.T, f *startWorkflowFixture) {
			f.cfg.branch = "feature/test"
		}},
		{"normal dirty tree", false, func(t *testing.T, f *startWorkflowFixture) {
			makeStartWorkflowDirty(t, f.cfg.cwd)
		}},
		{"normal rebase", false, func(t *testing.T, f *startWorkflowFixture) {
			createEndTestConflict(t, f.cfg.cwd)
		}},
		{"adopt missing branch", true, func(t *testing.T, f *startWorkflowFixture) {}},
		{"adopt remote-only branch", true, func(t *testing.T, f *startWorkflowFixture) {
			endTestGit(t, f.cfg.cwd, "push", "origin", "HEAD:refs/heads/"+f.cfg.branch)
			endTestGit(t, f.cfg.cwd, "fetch", "origin")
		}},
		{"adopt base branch", true, func(t *testing.T, f *startWorkflowFixture) {
			f.cfg.branch = "main"
		}},
		{"adopt rebase", true, func(t *testing.T, f *startWorkflowFixture) {
			f.cfg.branch = "feature/test"
			createEndTestConflict(t, f.cfg.cwd)
		}},
		{"adopt dirty other branch", true, func(t *testing.T, f *startWorkflowFixture) {
			endTestGit(t, f.cfg.cwd, "branch", f.cfg.branch)
			makeStartWorkflowDirty(t, f.cfg.cwd)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.adopt = tc.adopt
			tc.setup(t, f)
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage)
			if err == nil || result != nil {
				t.Fatalf("unsafe start = %#v, %v; want error", result, err)
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
		})
	}
}

func TestStartWorkflowRejectsExistingActiveOrPausedSession(t *testing.T) {
	for _, adopt := range []bool{false, true} {
		for _, state := range []string{db.StateActive, db.StatePaused} {
			for _, sameBranch := range []bool{false, true} {
				name := state + "/normal"
				if adopt {
					name = state + "/adopt"
				}
				if sameBranch {
					name += "/same branch"
				} else {
					name += "/other branch"
				}
				t.Run(name, func(t *testing.T) {
					f := newStartWorkflowFixture(t)
					f.cfg.adopt = adopt
					if adopt {
						f.cfg.branch = "feature/test"
					}
					existingBranch := "feature/other"
					if sameBranch {
						existingBranch = f.cfg.branch
					}
					manager := session.NewManager(f.database)
					existing, err := manager.StartSession(f.cfg.project.ID, existingBranch, "", "", "feature")
					if err != nil {
						t.Fatal(err)
					}
					events := 1
					if state == db.StatePaused {
						if _, err := manager.PauseSession(f.cfg.project.ID); err != nil {
							t.Fatal(err)
						}
						events++
					}
					before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
					if _, err := executeStartWorkflow(t.Context(), f.database, f.cfg, discardStartWorkflowMessage); err == nil {
						t.Fatal("start accepted existing active/paused session")
					}
					assertStartWorkflowCounts(t, f.path, 1, events)
					current, err := manager.GetActiveSession(f.cfg.project.ID)
					if err != nil || current == nil || current.ID != existing.ID || current.State != state {
						t.Fatalf("existing session changed: %#v, %v", current, err)
					}
					assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
				})
			}
		}
	}
}

func TestStartWorkflowCancellationBeforeMutation(t *testing.T) {
	for _, adopt := range []bool{false, true} {
		name := "normal"
		if adopt {
			name = "adopt"
		}
		t.Run(name, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.adopt = adopt
			if adopt {
				f.cfg.branch = "feature/test"
				makeStartWorkflowDirty(t, f.cfg.cwd)
			}
			before := snapshotStartWorkflowRepo(t, f.cfg.cwd)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err := executeStartWorkflow(ctx, f.database, f.cfg, discardStartWorkflowMessage)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled start = %v, want context.Canceled", err)
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
			assertStartWorkflowRepoUnchanged(t, f.cfg.cwd, before)
		})
	}
}

func TestStartWorkflowCancellationAfterBranchCreationPreventsSave(t *testing.T) {
	f := newStartWorkflowFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancelled := false
	send := func(tea.Msg) {
		if !cancelled && endTestGit(t, f.cfg.cwd, "branch", "--show-current") == f.cfg.branch {
			cancelled = true
			cancel()
		}
	}
	_, err := executeStartWorkflow(ctx, f.database, f.cfg, send)
	if !cancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("post-checkout cancellation = %v, callback cancelled = %t", err, cancelled)
	}
	assertStartWorkflowCounts(t, f.path, 0, 0)
	if got := endTestGit(t, f.cfg.cwd, "branch", "--show-current"); got != f.cfg.branch {
		t.Fatalf("created branch was not retained: %q", got)
	}
}

func TestStartWorkflowCompetingSaveLeavesOnlyWinner(t *testing.T) {
	for _, adopt := range []bool{false, true} {
		name := "normal"
		if adopt {
			name = "adopt"
		}
		t.Run(name, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			f.cfg.adopt = adopt
			if adopt {
				f.cfg.branch = "feature/test"
			}
			other, err := db.Open(f.path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			var winner *db.Session
			send := func(msg tea.Msg) {
				if line, ok := msg.(gitLineMsg); ok && string(line) == "Recording session start" {
					winner, err = session.NewManager(other).StartSession(f.cfg.project.ID, "feature/winner", "99", "Winner", "feature")
					if err != nil {
						t.Fatalf("competing save: %v", err)
					}
				}
			}
			result, err := executeStartWorkflow(t.Context(), f.database, f.cfg, send)
			if winner == nil || err == nil || result != nil {
				t.Fatalf("losing start = %#v, %v; winner = %#v", result, err, winner)
			}
			assertStartWorkflowCounts(t, f.path, 1, 1)
			current, err := f.database.GetActiveSession(f.cfg.project.ID)
			if err != nil || current == nil || current.ID != winner.ID {
				t.Fatalf("saved session = %#v, %v; want winner", current, err)
			}
			if got := endTestGit(t, f.cfg.cwd, "branch", "--show-current"); got != f.cfg.branch {
				t.Fatalf("losing save did not leave its Git branch intact: %q", got)
			}
		})
	}
}

func TestStartWorkflowRechecksBranchAndHeadBeforeSave(t *testing.T) {
	for _, change := range []string{"branch", "head"} {
		t.Run(change, func(t *testing.T) {
			f := newStartWorkflowFixture(t)
			changed := false
			send := func(msg tea.Msg) {
				if line, ok := msg.(gitLineMsg); !ok || string(line) != "Recording session start" {
					return
				}
				changed = true
				if change == "branch" {
					endTestGit(t, f.cfg.cwd, "checkout", "feature/test")
				} else {
					endTestGit(t, f.cfg.cwd, "commit", "--allow-empty", "-m", "Concurrent head change")
				}
			}
			if _, err := executeStartWorkflow(t.Context(), f.database, f.cfg, send); err == nil || !changed {
				t.Fatalf("changed %s accepted, changed = %t, error = %v", change, changed, err)
			}
			assertStartWorkflowCounts(t, f.path, 0, 0)
		})
	}
}

type startWorkflowFixture struct {
	cfg      startWorkflowConfig
	database *db.DB
	path     string
}

func newStartWorkflowFixture(t *testing.T) *startWorkflowFixture {
	t.Helper()
	repo, _, _, _ := newEndTestRepo(t)
	attachEndTestOrigin(t, repo)
	f := &startWorkflowFixture{path: filepath.Join(t.TempDir(), "start.db")}
	var err error
	f.database, err = db.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.database.Close(); err != nil {
			t.Error(err)
		}
	})
	project, err := session.NewManager(f.database).InitializeProject(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	f.cfg = startWorkflowConfig{
		cwd: repo, project: project, branch: "feature/new-session", branchType: "feature",
		issueID: "42", issueTitle: "Preserve issue metadata",
	}
	return f
}

func (f *startWorkflowFixture) reopen(t *testing.T) {
	t.Helper()
	if err := f.database.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.database, err = db.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
}

func advanceStartWorkflowOrigin(t *testing.T, repo string) {
	t.Helper()
	original := endTestGit(t, repo, "branch", "--show-current")
	endTestGit(t, repo, "checkout", "main")
	writeEndTestFile(t, repo, "base-update.txt", "remote base update\n")
	endTestGit(t, repo, "add", "base-update.txt")
	endTestGit(t, repo, "commit", "-m", "Advance origin base")
	endTestGit(t, repo, "push", "origin", "main")
	endTestGit(t, repo, "checkout", original)
}

func makeStartWorkflowDirty(t *testing.T, repo string) {
	t.Helper()
	writeEndTestFile(t, repo, "work.txt", "staged contents\n")
	endTestGit(t, repo, "add", "work.txt")
	writeEndTestFile(t, repo, "work.txt", "unstaged contents\n")
	writeEndTestFile(t, repo, "untracked.txt", "untracked contents\n")
}

type startWorkflowRepoSnapshot struct {
	branch, head, refs, status, staged, unstaged string
	files                                        map[string]string
}

func snapshotStartWorkflowRepo(t *testing.T, repo string) startWorkflowRepoSnapshot {
	t.Helper()
	snapshot := startWorkflowRepoSnapshot{
		branch:   endTestGit(t, repo, "branch", "--show-current"),
		head:     endTestGit(t, repo, "rev-parse", "HEAD"),
		refs:     endTestGit(t, repo, "show-ref", "--heads"),
		status:   endTestGit(t, repo, "status", "--porcelain=v1"),
		staged:   endTestGit(t, repo, "diff", "--cached", "--binary"),
		unstaged: endTestGit(t, repo, "diff", "--binary"),
		files:    make(map[string]string),
	}
	for _, name := range []string{"work.txt", "untracked.txt"} {
		data, err := os.ReadFile(filepath.Join(repo, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		snapshot.files[name] = string(data)
	}
	return snapshot
}

func assertStartWorkflowRepoUnchanged(t *testing.T, repo string, before startWorkflowRepoSnapshot) {
	t.Helper()
	if after := snapshotStartWorkflowRepo(t, repo); !reflect.DeepEqual(before, after) {
		t.Fatalf("repository changed:\nbefore: %#v\nafter: %#v", before, after)
	}
}

func assertStartWorkflowSession(t *testing.T, f *startWorkflowFixture, result *db.Session) {
	t.Helper()
	if result == nil || result.ID == 0 || result.ProjectID != f.cfg.project.ID ||
		result.Branch != f.cfg.branch || result.BranchType != f.cfg.branchType ||
		result.IssueID != f.cfg.issueID || result.IssueTitle != f.cfg.issueTitle ||
		result.State != db.StateActive || result.CurrentSliceStart == nil {
		t.Fatalf("started session = %#v, want config %#v", result, f.cfg)
	}
	saved, err := f.database.GetActiveSession(f.cfg.project.ID)
	if err != nil || saved == nil || saved.ID != result.ID || saved.IssueID != f.cfg.issueID ||
		saved.IssueTitle != f.cfg.issueTitle || saved.Branch != f.cfg.branch {
		t.Fatalf("persisted session = %#v, %v", saved, err)
	}
	assertStartWorkflowCounts(t, f.path, 1, 1)
	if got := endTestGit(t, f.cfg.cwd, "branch", "--show-current"); got != f.cfg.branch {
		t.Fatalf("current branch = %q, want %q", got, f.cfg.branch)
	}
}

func assertStartWorkflowCounts(t *testing.T, path string, sessions, events int) {
	t.Helper()
	connection, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	var gotSessions, gotEvents, starts int
	if err := connection.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&gotSessions); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow("SELECT COUNT(*) FROM session_events").Scan(&gotEvents); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow("SELECT COUNT(*) FROM session_events WHERE kind = 'session_started'").Scan(&starts); err != nil {
		t.Fatal(err)
	}
	if gotSessions != sessions || gotEvents != events || starts != sessions {
		t.Fatalf("sessions/events/starts = %d/%d/%d, want %d/%d/%d",
			gotSessions, gotEvents, starts, sessions, events, sessions)
	}
}

func discardStartWorkflowMessage(tea.Msg) {}
