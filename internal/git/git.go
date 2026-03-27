/*
Copyright © 2025 Bernard Katamanso <bernard@orctatech.com>
*/
package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileStatus represents one entry from `git status --porcelain`
type FileStatus struct {
	IndexStatus    byte   // first status char
	WorktreeStatus byte   // second status char
	Path           string // path shown (target path for rename/copy)
	OrigPath       string // original path for rename/copy (empty if not rename)
	RawLine        string // original porcelain line
}

// Run runs `git <args...>` in dir with a default timeout and returns stdout, stderr and error.
func Run(ctx context.Context, dir string, args ...string) (stdout string, stderr string, err error) {
	// If caller didn't provide a context deadline, add a sensible timeout to avoid hanging.
	// Caller can provide ctx with its own deadline to override.
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = strings.TrimRight(outBuf.String(), "\n")
	stderr = strings.TrimRight(errBuf.String(), "\n")

	// Wrap the error with stderr content for better debugging
	if err != nil {
		if stderr != "" {
			err = fmt.Errorf("%w: %s", err, stderr)
		}
	}
	return
}

// RunCombined runs git and returns combined output and error.
func RunCombined(ctx context.Context, dir string, args ...string) (string, error) {
	out, _, err := Run(ctx, dir, args...)
	return out, err
}

// GitStatusPorcelain runs `git status --porcelain` in the provided dir and returns parsed entries.
func GitStatusPorcelain(ctx context.Context, dir string) ([]FileStatus, error) {
	out, err := RunCombined(ctx, dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParsePorcelain(out), nil
}

// ParsePorcelain parses `git status --porcelain` output (porcelain v1).
// It returns a slice of FileStatus preserving Index/Worktree status and path(s).
//
// Format (per-line):
// XY <path> (for regular)
// XY <from> -> <to>  (for rename/copy)
// We split the line into the 2-status chars then the rest after a single space.
func ParsePorcelain(out string) []FileStatus {
	var res []FileStatus
	if strings.TrimSpace(out) == "" {
		return res
	}

	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		fs := FileStatus{RawLine: ln}
		// porcelain v1: first two chars are status, then a space, then path (possibly "from -> to")
		if len(ln) < 3 {
			// malformed, still keep raw
			res = append(res, fs)
			continue
		}
		fs.IndexStatus = ln[0]
		fs.WorktreeStatus = ln[1]
		rest := strings.TrimSpace(ln[3:]) // skip "XY" (2 chars + single space)
		fs.Path = rest
		// check rename pattern "from -> to"
		if idx := strings.Index(rest, "->"); idx != -1 {
			// split on '->', trim spaces around
			parts := strings.SplitN(rest, "->", 2)
			orig := strings.TrimSpace(parts[0])
			newp := strings.TrimSpace(parts[1])
			fs.OrigPath = orig
			fs.Path = newp
		}
		res = append(res, fs)
	}
	return res
}

// RunStream runs `git <args...>` in dir and streams stdout/stderr to callbacks.
// It returns the exit error when the command finishes.
// Note: If your callbacks aren't thread-safe, they may be called concurrently.
// Consider using RunGitWithOutput() with separate channels if ordering matters.
func RunStream(ctx context.Context, dir string, args []string,
	onStdout func(string),
	onStderr func(string)) error {

	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start git: %w", err)
	}

	// Use sync.WaitGroup to wait for both streams to finish
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if onStdout != nil {
				onStdout(line)
			}
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if onStderr != nil {
				onStderr(line)
			}
		}
	}()

	// Wait for both streams to finish reading
	wg.Wait()

	// Wait for process to exit
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git failed: %w", err)
	}
	return nil
}

// OutputChannels holds separate channels for stdout, stderr, and errors
type OutputChannels struct {
	Stdout <-chan string
	Stderr <-chan string
	Err    <-chan error
}

// RunGitWithOutput runs `git <args...>` in dir and streams stdout/stderr lines
// back to the caller through separate channels. The caller must read all channels
// until they are closed. If the command exits with error, it is sent on the error channel.
func RunGitWithOutput(ctx context.Context, dir string, args ...string) OutputChannels {
	stdoutCh := make(chan string)
	stderrCh := make(chan string)
	errCh := make(chan error, 1) // buffered so goroutine can exit

	go func() {
		defer close(stdoutCh)
		defer close(stderrCh)
		defer close(errCh)

		// apply default timeout if none given
		var cancel context.CancelFunc
		if _, ok := ctx.Deadline(); !ok {
			ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
		}

		cmd := exec.CommandContext(ctx, "git", args...)
		if dir != "" {
			cmd.Dir = dir
		}

		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			errCh <- fmt.Errorf("stdout pipe: %w", err)
			return
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			errCh <- fmt.Errorf("stderr pipe: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errCh <- fmt.Errorf("start git: %w", err)
			return
		}

		// scan stdout
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				select {
				case stdoutCh <- scanner.Text():
				case <-ctx.Done():
					return
				}
			}
		}()

		// scan stderr
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				select {
				case stderrCh <- scanner.Text():
				case <-ctx.Done():
					return
				}
			}
		}()

		// wait for command to finish
		if err := cmd.Wait(); err != nil {
			errCh <- fmt.Errorf("git failed: %w", err)
			return
		}
		errCh <- nil
	}()

	return OutputChannels{
		Stdout: stdoutCh,
		Stderr: stderrCh,
		Err:    errCh,
	}
}

// Fetch runs `git fetch <remote> <branch>`
func Fetch(ctx context.Context, dir, remote, branch string) error {
	_, err := RunCombined(ctx, dir, "fetch", remote, branch)
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	return nil
}

// FetchAll runs `git fetch --all`
func FetchAll(ctx context.Context, dir string) error {
	_, err := RunCombined(ctx, dir, "fetch", "--all")
	if err != nil {
		return fmt.Errorf("git fetch --all failed: %w", err)
	}
	return nil
}

// Pull runs `git pull <remote> <branch>`
func Pull(ctx context.Context, dir, remote, branch string) error {
	_, err := RunCombined(ctx, dir, "pull", remote, branch)
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}

// Push runs `git push <remote> <branch>`
func Push(ctx context.Context, dir, remote, branch string) error {
	_, err := RunCombined(ctx, dir, "push", remote, branch)
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

// PushWithOptions runs `git push` with additional flags
func PushWithOptions(ctx context.Context, dir string, args ...string) error {
	pushArgs := append([]string{"push"}, args...)
	_, err := RunCombined(ctx, dir, pushArgs...)
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

// Rebase runs `git rebase <upstream>`
func Rebase(ctx context.Context, dir, upstream string) error {
	_, err := RunCombined(ctx, dir, "rebase", upstream)
	if err != nil {
		return fmt.Errorf("git rebase failed: %w", err)
	}
	return nil
}

// RebaseOnto runs `git rebase --onto <newbase> <upstream> <branch>`
func RebaseOnto(ctx context.Context, dir, newbase, upstream, branch string) error {
	_, err := RunCombined(ctx, dir, "rebase", "--onto", newbase, upstream, branch)
	if err != nil {
		return fmt.Errorf("git rebase --onto failed: %w", err)
	}
	return nil
}

// Checkout runs `git checkout <branch>`
func Checkout(ctx context.Context, dir, branch string) error {
	_, err := RunCombined(ctx, dir, "checkout", branch)
	if err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}
	return nil
}

// CheckoutNew runs `git checkout -b <branch>`
func CheckoutNew(ctx context.Context, dir, branch string) error {
	_, err := RunCombined(ctx, dir, "checkout", "-b", branch)
	if err != nil {
		return fmt.Errorf("git checkout -b failed: %w", err)
	}
	return nil
}

// CurrentBranch returns the current branch name
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	branch, err := RunCombined(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch failed: %w", err)
	}
	return branch, nil
}

// Add runs `git add <paths...>`
func Add(ctx context.Context, dir string, paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := RunCombined(ctx, dir, args...)
	if err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	return nil
}

// AddAll runs `git add -A`
func AddAll(ctx context.Context, dir string) error {
	_, err := RunCombined(ctx, dir, "add", "-A")
	if err != nil {
		return fmt.Errorf("git add -A failed: %w", err)
	}
	return nil
}

// Commit runs `git commit -m <message>`
func Commit(ctx context.Context, dir, message string) error {
	_, err := RunCombined(ctx, dir, "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	return nil
}

// LatestCommitSubject returns the subject line for HEAD.
func LatestCommitSubject(ctx context.Context, dir string) (string, error) {
	out, err := RunCombined(ctx, dir, "log", "-n1", "--format=%s")
	if err != nil {
		return "", fmt.Errorf("git log failed: %w", err)
	}
	return out, nil
}

// CommitAmend runs `git commit --amend --no-edit`
func CommitAmend(ctx context.Context, dir string) error {
	_, err := RunCombined(ctx, dir, "commit", "--amend", "--no-edit")
	if err != nil {
		return fmt.Errorf("git commit --amend failed: %w", err)
	}
	return nil
}

// IsDirty checks if there are any uncommitted changes in the repo.
// Returns true if there are staged or unstaged changes.
func IsDirty(ctx context.Context, dir string) (bool, error) {
	out, err := RunCombined(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// HasCommitsAheadOf reports whether HEAD is ahead of the supplied upstream.
func HasCommitsAheadOf(ctx context.Context, dir, upstream string) (bool, error) {
	out, err := RunCombined(ctx, dir, "rev-list", "--count", upstream+"..HEAD")
	if err != nil {
		return false, fmt.Errorf("git rev-list failed: %w", err)
	}

	count := strings.TrimSpace(out)
	return count != "" && count != "0", nil
}

// IsRepo checks if the directory is a git repository
func IsRepo(ctx context.Context, dir string) (bool, error) {
	_, _, err := Run(ctx, dir, "rev-parse", "--git-dir")
	return err == nil, nil
}

// GetRemoteURL returns the URL for the specified remote
func GetRemoteURL(ctx context.Context, dir, remote string) (string, error) {
	url, err := RunCombined(ctx, dir, "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("get remote url failed: %w", err)
	}
	return url, nil
}

// ListRemotes returns all configured remotes
func ListRemotes(ctx context.Context, dir string) ([]string, error) {
	out, err := RunCombined(ctx, dir, "remote")
	if err != nil {
		return nil, fmt.Errorf("list remotes failed: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}

// Clone runs `git clone <url> <dir>`
func Clone(ctx context.Context, url, targetDir string) error {
	_, err := RunCombined(ctx, "", "clone", url, targetDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

// DeleteBranch deletes a local branch after it has been merged.
func DeleteBranch(ctx context.Context, dir, branch string) error {
	_, err := RunCombined(ctx, dir, "branch", "-d", branch)
	if err != nil {
		return fmt.Errorf("git branch -d failed: %w", err)
	}
	return nil
}

// Init runs `git init` in the specified directory
func Init(ctx context.Context, dir string) error {
	_, err := RunCombined(ctx, dir, "init")
	if err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	return nil
}

// Log returns the git log with specified format and limit
func Log(ctx context.Context, dir string, limit int, format string) (string, error) {
	args := []string{"log"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	if format != "" {
		args = append(args, fmt.Sprintf("--format=%s", format))
	}
	out, err := RunCombined(ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("git log failed: %w", err)
	}
	return out, nil
}

// Diff returns the git diff output
func Diff(ctx context.Context, dir string, args ...string) (string, error) {
	diffArgs := append([]string{"diff"}, args...)
	out, err := RunCombined(ctx, dir, diffArgs...)
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return out, nil
}

// Show returns the output of `git show <ref>`
func Show(ctx context.Context, dir, ref string) (string, error) {
	out, err := RunCombined(ctx, dir, "show", ref)
	if err != nil {
		return "", fmt.Errorf("git show failed: %w", err)
	}
	return out, nil
}

// GetRootDir returns the root directory of the git repository
func GetRootDir(ctx context.Context, dir string) (string, error) {
	root, err := RunCombined(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("get root dir failed: %w", err)
	}
	return filepath.Clean(root), nil
}
