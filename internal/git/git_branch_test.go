//go:build integration

package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalBranchHead(t *testing.T) {
	isolateGitEnvironment(t)
	repo := newGitTestRepo(t)
	runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")
	first := runGit(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "branch", "topic/literal")
	runGit(t, repo, "branch", "topic;literal")
	runGit(t, repo, "update-ref", "refs/remotes/origin/remote-only", first)
	runGit(t, repo, "tag", "tag-only")
	runGit(t, repo, "commit", "--allow-empty", "-m", "Second commit")
	second := runGit(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "update-ref", "refs/remotes/origin/topic/literal", second)
	runGit(t, repo, "pack-refs", "--all", "--prune")

	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{name: "literal branch", branch: "topic/literal", want: first},
		{name: "shell metacharacter is literal", branch: "topic;literal", want: first},
		{name: "current branch", branch: "main", want: second},
		{name: "missing branch", branch: "missing"},
		{name: "descendant is not exact", branch: "topic"},
		{name: "remote only", branch: "remote-only"},
		{name: "tag only", branch: "tag-only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertLocalBranchHead(t, repo, tt.branch, tt.want)
			if got := runGit(t, repo, "symbolic-ref", "HEAD"); got != "refs/heads/main" {
				t.Fatalf("HEAD changed to %q", got)
			}
		})
	}
}

func TestLocalBranchHeadInvalid(t *testing.T) {
	isolateGitEnvironment(t)
	repo := newGitTestRepo(t)
	runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")
	runGit(t, repo, "checkout", "-b", "topic")
	runGit(t, repo, "checkout", "main")
	for _, branch := range []string{
		"", "-topic", "--help", "--format=%(objectname)", "HEAD", "@{-1}", "@{-2}",
		"topic..other", "topic^", "topic~1", "topic:other", "topic\nmain",
		"topic\tmain", "topic; touch injected", "topic/*", "topic/[a]", "topic//other",
	} {
		t.Run(branch, func(t *testing.T) {
			t.Parallel()
			got, err := LocalBranchHead(context.Background(), repo, branch)
			if got != "" || err == nil {
				t.Fatalf("LocalBranchHead(%q) = (%q, %v), want empty and error", branch, got, err)
			}
			if strings.Contains(err.Error(), "get local branch head") {
				t.Fatalf("invalid branch reached ref lookup: %v", err)
			}
		})
	}
}

func TestLocalBranchHeadLocations(t *testing.T) {
	isolateGitEnvironment(t)
	repo := newGitTestRepo(t)
	runGit(t, repo, "commit", "--allow-empty", "-m", "Initial commit")
	want := runGit(t, repo, "rev-parse", "HEAD")
	linked := filepath.Join(t.TempDir(), "linked worktree")
	runGit(t, repo, "worktree", "add", "-b", "linked", linked)
	for _, dir := range []string{repo, linked} {
		nested := filepath.Join(dir, "nested", "subdirectory with spaces")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatal(err)
		}
		for _, location := range []string{dir, nested} {
			t.Run(location, func(t *testing.T) {
				t.Parallel()
				assertLocalBranchHead(t, location, "main", want)
				assertLocalBranchHead(t, location, "linked", want)
			})
		}
	}
}

func TestLocalBranchHeadErrors(t *testing.T) {
	isolateGitEnvironment(t)
	t.Run("not a repository", func(t *testing.T) {
		t.Parallel()
		got, err := LocalBranchHead(context.Background(), t.TempDir(), "topic")
		if got != "" || err == nil || !strings.Contains(err.Error(), "get local branch head") {
			t.Fatalf("LocalBranchHead = (%q, %v), want empty and ref lookup error", got, err)
		}
	})
	t.Run("missing directory", func(t *testing.T) {
		t.Parallel()
		got, err := LocalBranchHead(context.Background(), filepath.Join(t.TempDir(), "missing"), "topic")
		if got != "" || err == nil {
			t.Fatalf("LocalBranchHead = (%q, %v), want empty and error", got, err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		t.Parallel()
		repo := newGitTestRepo(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, err := LocalBranchHead(ctx, repo, "main")
		if got != "" || !errors.Is(err, context.Canceled) {
			t.Fatalf("LocalBranchHead = (%q, %v), want empty and context.Canceled", got, err)
		}
	})
}

func assertLocalBranchHead(t *testing.T, dir, branch, want string) {
	t.Helper()
	got, err := LocalBranchHead(context.Background(), dir, branch)
	if err != nil || got != want {
		t.Fatalf("LocalBranchHead(%q, %q) = (%q, %v), want (%q, nil)", dir, branch, got, err, want)
	}
}
