//go:build integration

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunCancellationWithInheritedPipes(t *testing.T) {
	isolateGitEnvironment(t)
	for _, streaming := range []bool{false, true} {
		name := "buffered"
		if streaming {
			name = "streaming"
		}
		t.Run(name, func(t *testing.T) {
			requirePOSIXFixture(t)
			repo := newGitTestRepo(t)
			writePipeHoldingHelper(t, filepath.Join(repo, ".git", "hooks", "pre-commit"))
			assertInheritedPipeCancellation(t, repo, func(ctx context.Context) error {
				args := []string{"commit", "--allow-empty", "-m", "Cancelled commit"}
				if streaming {
					return RunStream(ctx, repo, args, nil, nil)
				}
				_, _, err := Run(ctx, repo, args...)
				return err
			})
		})
	}
}

func TestRunStreamDrainsSuccessfulOutput(t *testing.T) {
	requirePOSIXFixture(t)
	isolateGitEnvironment(t)
	repo := newGitTestRepo(t)
	script := `#!/bin/sh
i=0
while [ "$i" -lt 10000 ]; do
	printf 'stdout-%s\n' "$i"
	printf 'stderr-%s\n' "$i" >&2
	i=$((i + 1))
done
printf 'stdout-final'
printf 'stderr-final' >&2
`
	path := filepath.Join(t.TempDir(), "emit")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr []string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := RunStream(
		ctx,
		repo,
		[]string{"-c", "alias.emit=!" + path, "emit"},
		func(line string) { stdout = append(stdout, line) },
		func(line string) { stderr = append(stderr, line) },
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range []struct {
		name  string
		lines []string
	}{
		{name: "stdout", lines: stdout},
		{name: "stderr", lines: stderr},
	} {
		if len(stream.lines) != 10001 {
			t.Fatalf("%s: got %d lines, want 10001", stream.name, len(stream.lines))
		}
		for i, line := range stream.lines[:10000] {
			if want := fmt.Sprintf("%s-%d", stream.name, i); line != want {
				t.Fatalf("%s line %d = %q, want %q", stream.name, i, line, want)
			}
		}
		if got, want := stream.lines[10000], stream.name+"-final"; got != want {
			t.Fatalf("final %s = %q, want %q", stream.name, got, want)
		}
	}
}

func requirePOSIXFixture(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture requires POSIX shell and process semantics")
	}
	for _, tool := range []string{"sh", "sleep", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("fixture requires %s: %v", tool, err)
		}
	}
}

func writePipeHoldingHelper(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
echo $$ > .git/sess-helper.pid
sleep 60 &
child=$!
echo "$child" > .git/sess-child.pid
wait "$child"
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
}

func assertInheritedPipeCancellation(t *testing.T, repo string, run func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	returned := false
	t.Cleanup(func() {
		cancel()
		// CommandContext kills only the direct child; fixture descendants need cleanup.
		for _, name := range []string{"sess-child.pid", "sess-helper.pid"} {
			pid := readFixturePID(filepath.Join(repo, ".git", name))
			if pid == 0 {
				continue
			}
			process, err := os.FindProcess(pid)
			if err != nil {
				t.Errorf("find fixture process %d: %v", pid, err)
				continue
			}
			if err := process.Kill(); err != nil {
				t.Logf("fixture process %d may already have exited: %v", pid, err)
			}
		}
		if !returned {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("executor still running after fixture cleanup")
			}
		}
	})

	readyBy := time.Now().Add(5 * time.Second)
	for readFixturePID(filepath.Join(repo, ".git", "sess-child.pid")) == 0 {
		select {
		case err := <-done:
			returned = true
			t.Fatalf("command returned before fixture became ready: %v", err)
		default:
		}
		if time.Now().After(readyBy) {
			t.Fatal("fixture did not start its pipe-holding descendant")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		returned = true
		if err == nil {
			t.Fatal("cancelled command returned no error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled command did not return within 3s while descendant held pipes")
	}
}

func readFixturePID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}
