package sess

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestContextCommandRequiresJSONAndNoArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing JSON flag",
			args: nil,
			want: "context currently requires --json",
		},
		{
			name: "extra argument",
			args: []string{"--json", "extra"},
			want: "unknown command",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := newContextCommand(contextCommandDeps{
				getwd:  func() (string, error) { t.Fatal("getwd called"); return "", nil },
				dbPath: func() (string, error) { t.Fatal("dbPath called"); return "", nil },
				now:    time.Now,
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestContextEnvelopeIgnoresUnknownFields(t *testing.T) {
	t.Parallel()

	input := `{
		"protocol_version":"sess.agent-work/v0",
		"kind":"context",
		"generated_at":"2026-10-18T10:30:00Z",
		"future_top_level":true,
		"data":{
			"availability":"idle_project",
			"project":null,
			"session":null,
			"repository_state":null,
			"active_attempts":[],
			"latest_handoff":null,
			"checks":[],
			"warnings":[],
			"future_data":{"value":1}
		}
	}`
	envelope := decodeContextTestJSON(t, input)
	if envelope.ProtocolVersion != contextProtocolVersion ||
		envelope.Data.Availability != "idle_project" {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestContextWriteError(t *testing.T) {
	t.Parallel()

	err := writeContext(
		t.Context(),
		errorContextWriter{},
		t.TempDir(),
		t.TempDir()+"/missing.db",
		time.Date(2026, 10, 18, 10, 30, 0, 0, time.UTC),
	)
	if err == nil || !strings.Contains(err.Error(), "write context JSON") ||
		!errors.Is(err, errContextWrite) {
		t.Fatalf("writeContext() error = %v", err)
	}
}

var errContextWrite = errors.New("context writer failed")

type errorContextWriter struct{}

func (errorContextWriter) Write([]byte) (int, error) {
	return 0, errContextWrite
}

func decodeContextTestJSON(t *testing.T, input string) contextEnvelope {
	t.Helper()
	var envelope contextEnvelope
	if err := json.Unmarshal([]byte(input), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}
