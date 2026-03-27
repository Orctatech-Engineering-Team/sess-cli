package tui

import (
	"strings"
	"testing"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHasShippableWork(t *testing.T) {
	tests := []struct {
		name  string
		dirty bool
		ahead bool
		want  bool
	}{
		{name: "clean and not ahead", dirty: false, ahead: false, want: false},
		{name: "dirty only", dirty: true, ahead: false, want: true},
		{name: "ahead only", dirty: false, ahead: true, want: true},
		{name: "dirty and ahead", dirty: true, ahead: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasShippableWork(tt.dirty, tt.ahead); got != tt.want {
				t.Fatalf("hasShippableWork(%t, %t) = %t, want %t", tt.dirty, tt.ahead, got, tt.want)
			}
		})
	}
}

func TestRenderPRBodyWithIssue(t *testing.T) {
	body := renderPRBody(&db.Session{
		IssueID:    "123",
		IssueTitle: "Add end command",
	}, prPromptValues{
		Summary: "Implemented sess end workflow",
		Testing: "",
		Notes:   "",
	})

	if !strings.Contains(body, "Closes #123") {
		t.Fatalf("body %q missing issue close line", body)
	}
	if !strings.Contains(body, "## Summary\nImplemented sess end workflow") {
		t.Fatalf("body %q missing summary section", body)
	}
	if !strings.Contains(body, "## Testing\nNot provided") {
		t.Fatalf("body %q missing testing fallback", body)
	}
	if !strings.Contains(body, "## Notes\nNot provided") {
		t.Fatalf("body %q missing notes fallback", body)
	}
}

func TestPRFromCreateOutputParsesURLAndNumber(t *testing.T) {
	pr := prFromCreateOutput("https://github.com/example/repo/pull/45\n")
	if pr == nil {
		t.Fatal("pr = nil, want parsed PR")
	}
	if pr.Number != 45 {
		t.Fatalf("PR number = %d, want 45", pr.Number)
	}
	if pr.URL != "https://github.com/example/repo/pull/45" {
		t.Fatalf("PR URL = %q, want URL", pr.URL)
	}
}

func TestBuildPRTitleUsesIssueMetadataWithoutGitLookup(t *testing.T) {
	title, err := buildPRTitle(nil, "", &db.Session{
		IssueID:    "321",
		IssueTitle: "Ship sess end",
	})
	if err != nil {
		t.Fatalf("buildPRTitle returned error: %v", err)
	}
	if title != "#321 Ship sess end" {
		t.Fatalf("title = %q, want %q", title, "#321 Ship sess end")
	}
}

func TestTextPromptModelSubmitsOnCtrlJ(t *testing.T) {
	model := newTextPromptModel("PR summary", "Describe what changed", true)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Implemented sess end workflow")})
	model = updated.(textPromptModel)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	model = updated.(textPromptModel)

	if !model.done {
		t.Fatal("model.done = false, want true")
	}
	if model.cancelled {
		t.Fatal("model.cancelled = true, want false")
	}
	if model.value != "Implemented sess end workflow" {
		t.Fatalf("model.value = %q, want full input preserved", model.value)
	}
}

func TestPromptModelSelectsOnCtrlJ(t *testing.T) {
	model := promptModel{list: newStyledList([]list.Item{choiceKeepBranch, choiceDeleteLocal}, "Cleanup")}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	model = updated.(promptModel)

	if !model.done {
		t.Fatal("model.done = false, want true")
	}
	if model.choice != choiceKeepBranch {
		t.Fatalf("model.choice = %#v, want %#v", model.choice, choiceKeepBranch)
	}
}
