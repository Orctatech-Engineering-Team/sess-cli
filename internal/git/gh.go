package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GH provides GitHub CLI operations
type GH struct{}

// Issue represents a GitHub issue with fields returned from JSON
type Issue struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// NewGH creates a new GH instance
func NewGH() *GH {
	return &GH{}
}

// Run runs `gh <args...>` with a default timeout and returns stdout, stderr and error.
func (g *GH) Run(ctx context.Context, dir string, args ...string) (stdout string, stderr string, err error) {
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = strings.TrimRight(outBuf.String(), "\n")
	stderr = strings.TrimRight(errBuf.String(), "\n")

	if err != nil {
		if stderr != "" {
			err = fmt.Errorf("%w: %s", err, stderr)
		}
	}
	return
}

// RunCombined runs gh and returns combined output and error.
func (g *GH) RunCombined(ctx context.Context, dir string, args ...string) (string, error) {
	out, _, err := g.Run(ctx, dir, args...)
	return out, err
}

// CreatePR creates a pull request
func (g *GH) CreatePR(ctx context.Context, dir, title, body, base string) (string, error) {
	args := []string{"pr", "create", "--title", title, "--body", body}
	if base != "" {
		args = append(args, "--base", base)
	}
	out, err := g.RunCombined(ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("create PR failed: %w", err)
	}
	return out, nil
}

// ListPRs lists pull requests
func (g *GH) ListPRs(ctx context.Context, dir string, state string) (string, error) {
	args := []string{"pr", "list"}
	if state != "" {
		args = append(args, "--state", state)
	}
	out, err := g.RunCombined(ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("list PRs failed: %w", err)
	}
	return out, nil
}

// ViewPR views a pull request
func (g *GH) ViewPR(ctx context.Context, dir string, prNumber string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "pr", "view", prNumber)
	if err != nil {
		return "", fmt.Errorf("view PR failed: %w", err)
	}
	return out, nil
}

// MergePR merges a pull request
func (g *GH) MergePR(ctx context.Context, dir string, prNumber string, mergeMethod string) error {
	args := []string{"pr", "merge", prNumber}
	if mergeMethod != "" {
		args = append(args, "--"+mergeMethod)
	}
	_, err := g.RunCombined(ctx, dir, args...)
	if err != nil {
		return fmt.Errorf("merge PR failed: %w", err)
	}
	return nil
}

// ClosePR closes a pull request
func (g *GH) ClosePR(ctx context.Context, dir string, prNumber string) error {
	_, err := g.RunCombined(ctx, dir, "pr", "close", prNumber)
	if err != nil {
		return fmt.Errorf("close PR failed: %w", err)
	}
	return nil
}

// CreateIssue creates an issue
func (g *GH) CreateIssue(ctx context.Context, dir, title, body string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "issue", "create", "--title", title, "--body", body)
	if err != nil {
		return "", fmt.Errorf("create issue failed: %w", err)
	}
	return out, nil
}

// ListIssues lists issues
func (g *GH) ListIssues(ctx context.Context, dir string, state string) (string, error) {
	args := []string{"issue", "list"}
	if state != "" {
		args = append(args, "--state", state)
	}
	out, err := g.RunCombined(ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("list issues failed: %w", err)
	}
	return out, nil
}

// ListIssuesJSON lists issues and returns them as structured Issue objects
func (g *GH) ListIssuesJSON(ctx context.Context, dir string, state string) ([]Issue, error) {
	args := []string{"issue", "list", "--json", "id,title,url"}
	if state != "" {
		args = append(args, "--state", state)
	}

	out, err := g.RunCombined(ctx, dir, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues failed: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse issues JSON: %w", err)
	}

	return issues, nil
}

// ViewIssue views an issue
func (g *GH) ViewIssue(ctx context.Context, dir string, issueNumber string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "issue", "view", issueNumber)
	if err != nil {
		return "", fmt.Errorf("view issue failed: %w", err)
	}
	return out, nil
}

// CloseIssue closes an issue
func (g *GH) CloseIssue(ctx context.Context, dir string, issueNumber string) error {
	_, err := g.RunCombined(ctx, dir, "issue", "close", issueNumber)
	if err != nil {
		return fmt.Errorf("close issue failed: %w", err)
	}
	return nil
}

// RepoView shows repository information
func (g *GH) RepoView(ctx context.Context, dir string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "repo", "view")
	if err != nil {
		return "", fmt.Errorf("repo view failed: %w", err)
	}
	return out, nil
}

// RepoClone clones a repository
func (g *GH) RepoClone(ctx context.Context, repo, targetDir string) error {
	args := []string{"repo", "clone", repo}
	if targetDir != "" {
		args = append(args, targetDir)
	}
	_, err := g.RunCombined(ctx, "", args...)
	if err != nil {
		return fmt.Errorf("repo clone failed: %w", err)
	}
	return nil
}

// RepoCreate creates a new repository
func (g *GH) RepoCreate(ctx context.Context, name string, public bool) (string, error) {
	args := []string{"repo", "create", name}
	if public {
		args = append(args, "--public")
	} else {
		args = append(args, "--private")
	}
	out, err := g.RunCombined(ctx, "", args...)
	if err != nil {
		return "", fmt.Errorf("repo create failed: %w", err)
	}
	return out, nil
}

// RunWorkflow runs a GitHub Actions workflow
func (g *GH) RunWorkflow(ctx context.Context, dir, workflow string) error {
	_, err := g.RunCombined(ctx, dir, "workflow", "run", workflow)
	if err != nil {
		return fmt.Errorf("run workflow failed: %w", err)
	}
	return nil
}

// ListWorkflows lists GitHub Actions workflows
func (g *GH) ListWorkflows(ctx context.Context, dir string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "workflow", "list")
	if err != nil {
		return "", fmt.Errorf("list workflows failed: %w", err)
	}
	return out, nil
}

// ViewWorkflowRun views a workflow run
func (g *GH) ViewWorkflowRun(ctx context.Context, dir, runID string) (string, error) {
	out, err := g.RunCombined(ctx, dir, "run", "view", runID)
	if err != nil {
		return "", fmt.Errorf("view workflow run failed: %w", err)
	}
	return out, nil
}

// Auth checks authentication status
func (g *GH) Auth(ctx context.Context) (string, error) {
	out, err := g.RunCombined(ctx, "", "auth", "status")
	if err != nil {
		return "", fmt.Errorf("auth status failed: %w", err)
	}
	return out, nil
}
