//go:build integration

package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseInProgress(t *testing.T) {
	isolateGitEnvironment(t)

	for _, linked := range []bool{false, true} {
		name := "regular repository"
		if linked {
			name = "linked worktree"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, marker := range []string{"", "rebase-merge", "rebase-apply"} {
				name := marker
				if name == "" {
					name = "no rebase"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					repo := newGitTestRepo(t)
					dir := repo
					if linked {
						runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")
						dir = filepath.Join(t.TempDir(), "linked worktree")
						runGit(t, repo, "worktree", "add", "-b", "linked", dir)
						if marker != "" {
							makeRebaseMarker(t, repo, marker)
						}
						assertRebaseInProgress(t, dir, false)
					}

					if marker != "" {
						makeRebaseMarker(t, dir, marker)
					}
					nested := filepath.Join(dir, "nested", "directory with spaces")
					if err := os.MkdirAll(nested, 0755); err != nil {
						t.Fatal(err)
					}
					cwd, err := os.Getwd()
					if err != nil {
						t.Fatal(err)
					}
					relative, err := filepath.Rel(cwd, nested)
					if err != nil {
						t.Fatal(err)
					}
					for _, location := range []string{dir, nested, relative} {
						assertRebaseInProgress(t, location, marker != "")
					}
				})
			}
		})
	}
}

func TestRebaseInProgressLinkedMarkerDoesNotAffectMainWorktree(t *testing.T) {
	isolateGitEnvironment(t)
	for _, marker := range []string{"rebase-merge", "rebase-apply"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			repo := newGitTestRepo(t)
			runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")
			linked := filepath.Join(t.TempDir(), "linked")
			runGit(t, repo, "worktree", "add", "-b", "linked", linked)
			makeRebaseMarker(t, linked, marker)

			assertRebaseInProgress(t, linked, true)
			assertRebaseInProgress(t, repo, false)
		})
	}
}

func TestRebaseInProgressErrors(t *testing.T) {
	isolateGitEnvironment(t)
	t.Run("invalid repository", func(t *testing.T) {
		t.Parallel()
		got, err := RebaseInProgress(context.Background(), t.TempDir())
		if err == nil || got {
			t.Fatalf("RebaseInProgress = (%t, %v), want false and an error", got, err)
		}
	})
	t.Run("missing directory", func(t *testing.T) {
		t.Parallel()
		got, err := RebaseInProgress(context.Background(), filepath.Join(t.TempDir(), "missing"))
		if err == nil || got {
			t.Fatalf("RebaseInProgress = (%t, %v), want false and an error", got, err)
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		t.Parallel()
		repo := newGitTestRepo(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, err := RebaseInProgress(ctx, repo)
		if got || !errors.Is(err, context.Canceled) {
			t.Fatalf("RebaseInProgress = (%t, %v), want false and context.Canceled", got, err)
		}
	})
	for _, marker := range []string{"rebase-merge", "rebase-apply"} {
		t.Run("filesystem error "+marker, func(t *testing.T) {
			t.Parallel()
			repo := newGitTestRepo(t)
			path := runGit(t, repo, "rev-parse", "--path-format=absolute", "--git-path", marker)
			if err := os.Symlink(marker, path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			got, err := RebaseInProgress(context.Background(), repo)
			if got || err == nil {
				t.Fatalf("RebaseInProgress = (%t, %v), want false and an error", got, err)
			}
		})
	}
}

func assertRebaseInProgress(t *testing.T, dir string, want bool) {
	t.Helper()
	got, err := RebaseInProgress(context.Background(), dir)
	if err != nil || got != want {
		t.Fatalf("RebaseInProgress(%q) = (%t, %v), want (%t, nil)", dir, got, err, want)
	}
}

func makeRebaseMarker(t *testing.T, dir, marker string) {
	t.Helper()
	path := runGit(t, dir, "rev-parse", "--path-format=absolute", "--git-path", marker)
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("create %s: %v", marker, err)
	}
}

func newGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo with spaces")
	runGit(t, "", "init", "--template=", "-b", "main", dir)
	runGit(t, dir, "config", "user.name", "SESS Test")
	runGit(t, dir, "config", "user.email", "sess-test@example.com")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func isolateGitEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
		"GIT_CEILING_DIRECTORIES",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
}
