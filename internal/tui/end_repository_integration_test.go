//go:build integration

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
)

func TestEndCheckpointRejectsChangedOrigin(t *testing.T) {
	cfg, database, _, ghDir := newEndCheckpointFixture(t)
	writeEndTestFile(t, ghDir, "fail-create", "fail\n")
	if _, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage); err == nil {
		t.Fatal("expected initial PR failure")
	}
	endTestGit(t, cfg.cwd, "remote", "set-url", "origin", "https://github.com/other/repo.git")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "origin changed") {
		t.Fatalf("retry = %v, want changed origin rejection", err)
	}
	assertEndTestSessionOpen(t, session.NewManager(database), cfg.session)
	creates, err := os.ReadFile(filepath.Join(ghDir, "creates"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(creates), "create\n") != 1 {
		t.Fatalf("retry created another PR: %q", creates)
	}
}

func TestEndCheckpointRejectsSeparatePushDestination(t *testing.T) {
	cfg, database, _, ghDir := newEndCheckpointFixture(t)
	endTestGit(t, cfg.cwd, "remote", "set-url", "--push", "origin", "https://github.com/other/repo.git")
	_, err := executeEndWorkflow(t.Context(), database, cfg, discardEndTestMessage)
	if err == nil || !strings.Contains(err.Error(), "matching origin fetch and push URL") {
		t.Fatalf("end = %v, want separate destination rejection", err)
	}
	checkpoint, err := database.GetEndWorkflow(cfg.session.ID)
	if err != nil || checkpoint != nil {
		t.Fatalf("checkpoint = %#v, error = %v; want no checkpoint", checkpoint, err)
	}
	if _, err := os.Stat(filepath.Join(ghDir, "creates")); !os.IsNotExist(err) {
		t.Fatalf("PR create should not run: %v", err)
	}
}
