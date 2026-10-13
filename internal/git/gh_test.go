//go:build integration

package git

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGHRunCancellationWithInheritedPipes(t *testing.T) {
	requirePOSIXFixture(t)
	isolateGitEnvironment(t)
	repo := newGitTestRepo(t)
	bin := t.TempDir()
	writePipeHoldingHelper(t, filepath.Join(bin, "gh"))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	assertInheritedPipeCancellation(t, repo, func(ctx context.Context) error {
		_, _, err := NewGH().Run(ctx, repo, "fixture")
		return err
	})
}

func TestGHFindOpenPRByHeadPreservesFirstResult(t *testing.T) {
	output := `[{"number":42,"url":"https://github.com/test/repo/pull/42","title":"First","state":"OPEN"},
{"number":43,"url":"https://github.com/test/repo/pull/43","title":"Second","state":"OPEN"}]`
	dir, argsPath := fakeRecoveryGH(t, output, "", 0)
	got, err := NewGH().FindOpenPRByHead(context.Background(), dir, "sess/42-work")
	if err != nil || got == nil || got.Number != 42 {
		t.Fatalf("FindOpenPRByHead = (%+v, %v), want first PR and nil error", got, err)
	}
	assertRecoveryGHArgs(t, argsPath, []string{
		"pr", "list", "--head", "sess/42-work", "--state", "open", "--json", "number,url,title,state",
	})
}

func TestGHGetPR(t *testing.T) {
	for _, state := range []string{"OPEN", "MERGED", "CLOSED"} {
		t.Run(state, func(t *testing.T) {
			want := recoveryTestPR(state)
			dir, argsPath := fakeRecoveryGH(t, prJSON(t, want), "", 0)
			got, err := NewGH().GetPR(context.Background(), dir, want.Number)
			if err != nil || got == nil || *got != want {
				t.Fatalf("GetPR = (%+v, %v), want (%+v, nil)", got, err, want)
			}
			assertRecoveryGHArgs(t, argsPath, []string{
				"pr", "view", "42", "--json",
				"number,url,title,state,baseRefName,headRefName,headRefOid,headRepository,headRepositoryOwner",
			})
		})
	}
}

func TestGHGetPRErrors(t *testing.T) {
	for _, tt := range recoveryMetadataErrors(t) {
		t.Run(tt.name, func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, tt.output, "", 0)
			got, err := NewGH().GetPR(context.Background(), dir, 42)
			if got != nil || err == nil {
				t.Fatalf("GetPR = (%+v, %v), want nil and error", got, err)
			}
			assertRecoveryGHArgs(t, argsPath, []string{
				"pr", "view", "42", "--json",
				"number,url,title,state,baseRefName,headRefName,headRefOid,headRepository,headRepositoryOwner",
			})
		})
	}
	t.Run("explicit number mismatch", func(t *testing.T) {
		pr := recoveryTestPR("OPEN")
		pr.Number = 43
		dir, _ := fakeRecoveryGH(t, prJSON(t, pr), "", 0)
		got, err := NewGH().GetPR(context.Background(), dir, 42)
		if got != nil || err == nil || !strings.Contains(err.Error(), "returned PR #43") {
			t.Fatalf("GetPR = (%+v, %v), want number mismatch error", got, err)
		}
	})
	for _, number := range []int{0, -1} {
		t.Run("invalid number "+prJSON(t, number), func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, prJSON(t, recoveryTestPR("OPEN")), "", 0)
			got, err := NewGH().GetPR(context.Background(), dir, number)
			if got != nil || err == nil {
				t.Fatalf("GetPR = (%+v, %v), want nil and error", got, err)
			}
			if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
				t.Fatalf("invalid number invoked gh: %v", err)
			}
		})
	}
}

func TestGHFindPRByHead(t *testing.T) {
	for _, state := range []string{"OPEN", "MERGED", "CLOSED"} {
		t.Run(state, func(t *testing.T) {
			want := recoveryTestPR(state)
			otherHead := want
			otherHead.Number = 43
			otherHead.HeadRefName = "other"
			otherBase := want
			otherBase.Number = 44
			otherBase.BaseRefName = "other"
			dir, argsPath := fakeRecoveryGH(t, prJSON(t, []PR{otherHead, want, otherBase}), "", 0)
			got, err := NewGH().FindPRByHead(context.Background(), dir, want.HeadRefName, want.BaseRefName)
			if err != nil || got == nil || *got != want {
				t.Fatalf("FindPRByHead = (%+v, %v), want (%+v, nil)", got, err, want)
			}
			assertRecoveryGHArgs(t, argsPath, recoveryListArgs())
		})
	}
	t.Run("empty", func(t *testing.T) {
		dir, argsPath := fakeRecoveryGH(t, "[]", "", 0)
		got, err := NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
		if got != nil || err != nil {
			t.Fatalf("FindPRByHead = (%+v, %v), want (nil, nil)", got, err)
		}
		assertRecoveryGHArgs(t, argsPath, recoveryListArgs())
	})
	t.Run("only mismatched refs", func(t *testing.T) {
		pr := recoveryTestPR("OPEN")
		pr.HeadRefName = "other"
		dir, _ := fakeRecoveryGH(t, prJSON(t, []PR{pr}), "", 0)
		got, err := NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
		if got != nil || err != nil {
			t.Fatalf("FindPRByHead = (%+v, %v), want (nil, nil)", got, err)
		}
	})
}

func TestGHFindPRByHeadErrors(t *testing.T) {
	for _, tt := range recoveryMetadataErrors(t) {
		t.Run(tt.name, func(t *testing.T) {
			output := "[" + tt.output + "]"
			if tt.name == "null" || tt.name == "empty output" {
				output = tt.output
			}
			dir, _ := fakeRecoveryGH(t, output, "", 0)
			got, err := NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
			if got != nil || err == nil {
				t.Fatalf("FindPRByHead = (%+v, %v), want nil and error", got, err)
			}
		})
	}
	t.Run("ambiguous branch reuse", func(t *testing.T) {
		open := recoveryTestPR("OPEN")
		merged := recoveryTestPR("MERGED")
		merged.Number = 41
		dir, _ := fakeRecoveryGH(t, prJSON(t, []PR{merged, open}), "", 0)
		got, err := NewGH().FindPRByHead(context.Background(), dir, open.HeadRefName, open.BaseRefName)
		if got != nil || err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("FindPRByHead = (%+v, %v), want ambiguity error", got, err)
		}
	})
	t.Run("saturated with mismatched refs", func(t *testing.T) {
		prs := make([]PR, 100)
		for i := range prs {
			prs[i] = recoveryTestPR("CLOSED")
			prs[i].Number = i + 1
			prs[i].HeadRefName = "other"
		}
		dir, _ := fakeRecoveryGH(t, prJSON(t, prs), "", 0)
		got, err := NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
		if got != nil || err == nil || !strings.Contains(err.Error(), "completeness") {
			t.Fatalf("FindPRByHead = (%+v, %v), want completeness error", got, err)
		}
	})
	t.Run("malformed mismatched entry", func(t *testing.T) {
		pr := recoveryTestPR("UNKNOWN")
		pr.HeadRefName = "other"
		dir, _ := fakeRecoveryGH(t, prJSON(t, []PR{pr}), "", 0)
		got, err := NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
		if got != nil || err == nil {
			t.Fatalf("FindPRByHead = (%+v, %v), want nil and error", got, err)
		}
	})
}

func TestGHRecoveryCommandErrors(t *testing.T) {
	for _, message := range []string{"authentication required", "network unavailable", "pull request not found"} {
		t.Run(message, func(t *testing.T) {
			for _, lookup := range []string{"number", "head"} {
				t.Run(lookup, func(t *testing.T) {
					dir, _ := fakeRecoveryGH(t, "[]", message, 1)
					var got *PR
					var err error
					if lookup == "number" {
						got, err = NewGH().GetPR(context.Background(), dir, 42)
					} else {
						got, err = NewGH().FindPRByHead(context.Background(), dir, "sess/42-work", "main")
					}
					var exitErr *exec.ExitError
					if got != nil || !errors.As(err, &exitErr) || !strings.Contains(err.Error(), message) {
						t.Fatalf("lookup = (%+v, %v), want wrapped exit error containing %q", got, err, message)
					}
				})
			}
		})
	}
}

func TestGHResolveRepository(t *testing.T) {
	for _, tt := range []struct {
		name, remote, output, want string
	}{
		{
			name: "HTTPS renamed repository", remote: "https://github.com/old/old.git",
			output: `{"nameWithOwner":"Canonical/Repo","url":"https://github.com/Canonical/Repo"}`,
			want:   "github.com/Canonical/Repo",
		},
		{
			name: "SSH enterprise repository", remote: "git@ghe.example.com:test/repo.git",
			output: `{"nameWithOwner":"test/repo","url":"https://ghe.example.com/test/repo"}`,
			want:   "ghe.example.com/test/repo",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, tt.output, "", 0)
			got, err := NewGH().ResolveRepository(context.Background(), dir, tt.remote)
			if got != tt.want || err != nil {
				t.Fatalf("ResolveRepository = (%q, %v), want (%q, nil)", got, err, tt.want)
			}
			assertRecoveryGHArgs(t, argsPath, []string{
				"repo", "view", tt.remote, "--json", "nameWithOwner,url",
			})
		})
	}
}

func TestGHResolveRepositoryErrors(t *testing.T) {
	for _, tt := range []struct {
		name, output string
	}{
		{name: "invalid JSON", output: "{"},
		{name: "null", output: "null"},
		{name: "missing URL", output: `{"nameWithOwner":"test/repo"}`},
		{name: "missing name", output: `{"url":"https://github.com/test/repo"}`},
		{name: "relative URL", output: `{"nameWithOwner":"test/repo","url":"/test/repo"}`},
		{name: "bad URL", output: `{"nameWithOwner":"test/repo","url":"https://%/test/repo"}`},
		{name: "name mismatch", output: `{"nameWithOwner":"test/repo","url":"https://github.com/fork/repo"}`},
		{name: "bad name", output: `{"nameWithOwner":"repo","url":"https://github.com/repo"}`},
		{name: "query", output: `{"nameWithOwner":"test/repo","url":"https://github.com/test/repo?x=1"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, _ := fakeRecoveryGH(t, tt.output, "", 0)
			got, err := NewGH().ResolveRepository(context.Background(), dir, "https://github.com/test/repo.git")
			if got != "" || err == nil {
				t.Fatalf("ResolveRepository = (%q, %v), want empty and error", got, err)
			}
		})
	}
	t.Run("command failure", func(t *testing.T) {
		dir, _ := fakeRecoveryGH(t, "", "network unavailable", 1)
		got, err := NewGH().ResolveRepository(context.Background(), dir, "https://github.com/test/repo.git")
		var exitErr *exec.ExitError
		if got != "" || !errors.As(err, &exitErr) {
			t.Fatalf("ResolveRepository = (%q, %v), want wrapped exit error", got, err)
		}
	})
	for _, remote := range []string{"", " ", "--web"} {
		t.Run("invalid remote "+remote, func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, "{}", "", 0)
			got, err := NewGH().ResolveRepository(context.Background(), dir, remote)
			if got != "" || err == nil {
				t.Fatalf("ResolveRepository = (%q, %v), want empty and error", got, err)
			}
			if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
				t.Fatalf("invalid remote invoked gh: %v", err)
			}
		})
	}
}

func TestGHRepositoryBoundRecovery(t *testing.T) {
	for _, tt := range []struct {
		name, owner, repositoryName string
		match                       bool
	}{
		{name: "same repository", owner: "test", repositoryName: "repo", match: true},
		{name: "case insensitive", owner: "TEST", repositoryName: "REPO", match: true},
		{name: "foreign fork", owner: "foreign", repositoryName: "repo"},
		{name: "same owner other repository", owner: "test", repositoryName: "other"},
		{name: "missing owner", repositoryName: "repo"},
		{name: "missing repository", owner: "test"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pr := recoveryTestPR("MERGED")
			pr.HeadRepositoryOwner.Login = tt.owner
			pr.HeadRepository.Name = tt.repositoryName
			t.Run("get", func(t *testing.T) {
				dir, argsPath := fakeRecoveryGH(t, prJSON(t, pr), "", 0)
				got, err := NewGH().GetPR(context.Background(), dir, 42, "github.com/test/repo")
				if tt.match {
					if got == nil || err != nil || *got != pr {
						t.Fatalf("GetPR = (%+v, %v), want matching PR", got, err)
					}
				} else if got != nil || err == nil {
					t.Fatalf("GetPR = (%+v, %v), want rejected head repository", got, err)
				}
				assertRecoveryGHArgs(t, argsPath, []string{
					"pr", "view", "42", "--json", recoveryPRFields, "--repo", "github.com/test/repo",
				})
			})
			t.Run("find", func(t *testing.T) {
				dir, argsPath := fakeRecoveryGH(t, prJSON(t, []PR{pr}), "", 0)
				got, err := NewGH().FindPRByHead(
					context.Background(), dir, pr.HeadRefName, pr.BaseRefName, "github.com/test/repo",
				)
				if err != nil || (got != nil) != tt.match {
					t.Fatalf("FindPRByHead = (%+v, %v), want match=%v", got, err, tt.match)
				}
				assertRecoveryGHArgs(t, argsPath, append(recoveryListArgs(), "--repo", "github.com/test/repo"))
			})
		})
	}
	t.Run("foreign fork does not cause ambiguity", func(t *testing.T) {
		want := recoveryTestPR("OPEN")
		foreign := want
		foreign.Number = 43
		foreign.HeadRepositoryOwner.Login = "foreign"
		dir, _ := fakeRecoveryGH(t, prJSON(t, []PR{foreign, want}), "", 0)
		got, err := NewGH().FindPRByHead(
			context.Background(), dir, want.HeadRefName, want.BaseRefName, "github.com/test/repo",
		)
		if got == nil || err != nil || *got != want {
			t.Fatalf("FindPRByHead = (%+v, %v), want same-repository PR", got, err)
		}
	})
}

func TestGHRecoveryInvalidRepository(t *testing.T) {
	for _, repository := range [][]string{{""}, {"test/repo"}, {"github.com//repo"}, {"github.com/test/repo", "other"}} {
		t.Run(strings.Join(repository, ","), func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, "[]", "", 0)
			if got, err := NewGH().GetPR(context.Background(), dir, 42, repository...); got != nil || err == nil {
				t.Fatalf("GetPR = (%+v, %v), want invalid repository error", got, err)
			}
			got, err := NewGH().FindPRByHead(
				context.Background(), dir, "sess/42-work", "main", repository...,
			)
			if got != nil || err == nil {
				t.Fatalf("FindPRByHead = (%+v, %v), want invalid repository error", got, err)
			}
			if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
				t.Fatalf("invalid repository invoked gh: %v", err)
			}
		})
	}
}

func TestGHCreatePRForHead(t *testing.T) {
	for _, base := range []string{"main", ""} {
		t.Run("base "+base, func(t *testing.T) {
			want := "https://ghe.example.com/test/repo/pull/42"
			dir, argsPath := fakeRecoveryGH(t, want, "", 0)
			got, err := NewGH().CreatePRForHead(
				context.Background(), dir, "Work", "Fixes #42", base, "sess/42-work", "ghe.example.com/test/repo",
			)
			if got != want || err != nil {
				t.Fatalf("CreatePRForHead = (%q, %v), want (%q, nil)", got, err, want)
			}
			args := []string{"pr", "create", "--title", "Work", "--body", "Fixes #42"}
			if base != "" {
				args = append(args, "--base", base)
			}
			args = append(args, "--head", "sess/42-work", "--repo", "ghe.example.com/test/repo")
			assertRecoveryGHArgs(t, argsPath, args)
		})
	}
	t.Run("command failure", func(t *testing.T) {
		dir, _ := fakeRecoveryGH(t, "", "creation failed", 1)
		got, err := NewGH().CreatePRForHead(
			context.Background(), dir, "Work", "Body", "main", "branch", "github.com/test/repo",
		)
		var exitErr *exec.ExitError
		if got != "" || !errors.As(err, &exitErr) {
			t.Fatalf("CreatePRForHead = (%q, %v), want wrapped exit error", got, err)
		}
	})
	for _, tt := range []struct{ name, head, repository string }{
		{name: "empty head", repository: "github.com/test/repo"},
		{name: "empty repository", head: "branch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, argsPath := fakeRecoveryGH(t, "", "", 0)
			got, err := NewGH().CreatePRForHead(
				context.Background(), dir, "Work", "Body", "main", tt.head, tt.repository,
			)
			if got != "" || err == nil {
				t.Fatalf("CreatePRForHead = (%q, %v), want invalid input error", got, err)
			}
			if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
				t.Fatalf("invalid input invoked gh: %v", err)
			}
		})
	}
}

func TestGHCreatePRPreservesArguments(t *testing.T) {
	dir, argsPath := fakeRecoveryGH(t, "https://github.com/test/repo/pull/42", "", 0)
	_, err := NewGH().CreatePR(context.Background(), dir, "Work", "Body", "main")
	if err != nil {
		t.Fatal(err)
	}
	assertRecoveryGHArgs(t, argsPath, []string{
		"pr", "create", "--title", "Work", "--body", "Body", "--base", "main",
	})
}

func recoveryTestPR(state string) PR {
	pr := PR{
		Number: 42, URL: "https://github.com/test/repo/pull/42", Title: "Work", State: state,
		BaseRefName: "main", HeadRefName: "sess/42-work", HeadRefOid: strings.Repeat("a", 40),
	}
	pr.HeadRepository.Name = "repo"
	pr.HeadRepositoryOwner.Login = "test"
	return pr
}

func recoveryMetadataErrors(t *testing.T) []struct{ name, output string } {
	t.Helper()
	tests := []struct{ name, output string }{
		{name: "invalid JSON", output: "{"},
		{name: "empty output", output: ""},
		{name: "null", output: "null"},
		{name: "empty object", output: "{}"},
		{name: "wrong JSON type", output: `"not a PR"`},
	}
	for _, field := range []string{"number", "url", "baseRefName", "headRefName", "headRefOid", "state"} {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(prJSON(t, recoveryTestPR("OPEN"))), &metadata); err != nil {
			t.Fatal(err)
		}
		delete(metadata, field)
		tests = append(tests, struct{ name, output string }{name: "missing " + field, output: prJSON(t, metadata)})
	}
	pr := recoveryTestPR("UNKNOWN")
	tests = append(tests, struct{ name, output string }{name: "unknown state", output: prJSON(t, pr)})
	pr = recoveryTestPR("OPEN")
	pr.Number = -1
	tests = append(tests, struct{ name, output string }{name: "negative number", output: prJSON(t, pr)})
	return tests
}

func prJSON(t *testing.T, value any) string {
	t.Helper()
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func recoveryListArgs() []string {
	return []string{
		"pr", "list", "--head", "sess/42-work", "--base", "main", "--state", "all", "--limit", "100",
		"--json", "number,url,title,state,baseRefName,headRefName,headRefOid,headRepository,headRepositoryOwner",
	}
}

func fakeRecoveryGH(t *testing.T, stdout, stderr string, exitCode int) (string, string) {
	t.Helper()
	requirePOSIXFixture(t)
	dir := t.TempDir()
	bin := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	files := map[string]string{
		"stdout": stdout,
		"stderr": stderr,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
printf '%s\n' "$@" > args
cat stdout
cat stderr >&2
exit ` + prJSON(t, exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir, argsPath
}

func assertRecoveryGHArgs(t *testing.T, path string, want []string) {
	t.Helper()
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gh args = %q, want %q", got, want)
	}
}
