package git

import (
	"context"
	"fmt"
	"strings"
)

// LocalBranchHead returns the OID of a literal local branch, or "" if absent.
func LocalBranchHead(ctx context.Context, dir, branch string) (string, error) {
	if branch == "" || strings.HasPrefix(branch, "-") {
		return "", fmt.Errorf("invalid branch name %q", branch)
	}

	validated, err := RunCombined(ctx, dir, "check-ref-format", "--branch", branch)
	if err != nil {
		return "", fmt.Errorf("validate branch %q: %w", branch, err)
	}
	if validated != branch {
		return "", fmt.Errorf("branch name %q is not literal", branch)
	}

	ref := "refs/heads/" + branch
	out, err := RunCombined(ctx, dir, "for-each-ref", "--format=%(refname)%09%(objectname)", "--", ref)
	if err != nil {
		return "", fmt.Errorf("get local branch head %q: %w", branch, err)
	}

	// for-each-ref also matches descendants of the requested ref.
	for _, line := range strings.Split(out, "\n") {
		name, oid, ok := strings.Cut(line, "\t")
		if ok && name == ref {
			return oid, nil
		}
	}
	return "", nil
}
