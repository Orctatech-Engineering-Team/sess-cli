package sess

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

func TestHistoryDetailArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "not a number", args: []string{"abc"}, want: "session ID must be a positive integer"},
		{name: "zero", args: []string{"0"}, want: "session ID must be a positive integer"},
		{name: "negative", args: []string{"--", "-1"}, want: "session ID must be a positive integer"},
		{name: "overflow", args: []string{"9223372036854775808"}, want: "session ID must be a positive integer"},
		{name: "too many IDs", args: []string{"1", "2"}, want: "accepts at most 1 arg(s)"},
		{name: "all", args: []string{"1", "--all"}, want: "--all and --limit cannot be used with a session ID"},
		{name: "explicit false all", args: []string{"1", "--all=false"}, want: "--all and --limit cannot be used with a session ID"},
		{name: "explicit default limit", args: []string{"1", "--limit=10"}, want: "--all and --limit cannot be used with a session ID"},
		{name: "short limit", args: []string{"1", "-n", "5"}, want: "--all and --limit cannot be used with a session ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{
				Use:           historyCmd.Use,
				Args:          historyCmd.Args,
				RunE:          historyCmd.RunE,
				SilenceUsage:  true,
				SilenceErrors: true,
			}
			cmd.Flags().Bool("all", false, "")
			cmd.Flags().IntP("limit", "n", 10, "")
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

func TestRenderSessionTimeline(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 2, 10, 0, 0, 123, time.FixedZone("offset", 2*60*60))
	pause := start.Add(10 * time.Minute)
	resume := pause.Add(time.Hour)
	end := resume.Add(5 * time.Minute)
	tests := []struct {
		name     string
		timeline *session.SessionTimeline
		elapsed  time.Duration
		want     string
	}{
		{
			name: "complete lifecycle",
			timeline: &session.SessionTimeline{
				Session: &db.Session{ID: 42, Branch: "feature-login", BranchType: "feature", State: db.StateEnded},
				Events: []*db.SessionEvent{
					{Kind: "session_started", OccurredAt: start},
					{Kind: "session_paused", OccurredAt: pause, TotalElapsed: int64(10 * time.Minute)},
					{Kind: "session_resumed", OccurredAt: resume, TotalElapsed: int64(10 * time.Minute)},
					{Kind: "session_ended", OccurredAt: end, TotalElapsed: int64(15 * time.Minute)},
				},
				Intervals: []session.ActiveInterval{
					{Start: start, End: &pause, Duration: 10 * time.Minute},
					{Start: resume, End: &end, Duration: 5 * time.Minute},
				},
				HistoryComplete: true,
			},
			elapsed: 15 * time.Minute,
			want: "Session 42 · feature-login (feature)\n" +
				"ended · tracked active total 15m0s\n" +
				"Lifecycle history complete\n\n" +
				"Recorded events\n" +
				"  2026-01-02T08:00:00.000000123Z · started · elapsed 0s\n" +
				"  2026-01-02T08:10:00.000000123Z · paused · elapsed 10m0s\n" +
				"  2026-01-02T09:10:00.000000123Z · resumed · elapsed 10m0s\n" +
				"  2026-01-02T09:15:00.000000123Z · ended · elapsed 15m0s\n\n" +
				"Active intervals\n" +
				"  2026-01-02T08:00:00.000000123Z → 2026-01-02T08:10:00.000000123Z · 10m0s\n" +
				"  2026-01-02T09:10:00.000000123Z → 2026-01-02T09:15:00.000000123Z · 5m0s\n",
		},
		{
			name: "legacy without events",
			timeline: &session.SessionTimeline{
				Session: &db.Session{ID: 7, Branch: "legacy", State: db.StateEnded, TotalElapsed: int64(2 * time.Hour)},
			},
			elapsed: 2 * time.Hour,
			want: "Session 7 · legacy\n" +
				"ended · tracked active total 2h0m\n" +
				"No recorded lifecycle history · tracked total preserved\n",
		},
		{
			name: "partial legacy history",
			timeline: &session.SessionTimeline{
				Session: &db.Session{ID: 8, Branch: "legacy", State: db.StatePaused, TotalElapsed: int64(2 * time.Hour)},
				Events: []*db.SessionEvent{
					{Kind: "session_paused", OccurredAt: pause, TotalElapsed: int64(2 * time.Hour)},
				},
			},
			elapsed: 2 * time.Hour,
			want: "Session 8 · legacy\n" +
				"paused · tracked active total 2h0m\n" +
				"Lifecycle history incomplete · tracked total preserved\n\n" +
				"Recorded events\n" +
				"  2026-01-02T08:10:00.000000123Z · paused · elapsed 2h0m\n",
		},
		{
			name: "ongoing interval",
			timeline: &session.SessionTimeline{
				Session: &db.Session{ID: 9, Branch: "work", State: db.StateActive},
				Events: []*db.SessionEvent{
					{Kind: "session_started", OccurredAt: start},
				},
				Intervals:       []session.ActiveInterval{{Start: start, Duration: 30 * time.Second}},
				HistoryComplete: true,
			},
			elapsed: 30 * time.Second,
			want: "Session 9 · work\n" +
				"active · tracked active total 30s\n" +
				"Lifecycle history complete\n\n" +
				"Recorded events\n" +
				"  2026-01-02T08:00:00.000000123Z · started · elapsed 0s\n\n" +
				"Active intervals\n" +
				"  2026-01-02T08:00:00.000000123Z → ongoing · 30s\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := renderSessionTimeline(&output, tt.timeline, tt.elapsed); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tt.want {
				t.Fatalf("renderSessionTimeline() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestRenderSessionTimelineWriteError(t *testing.T) {
	t.Parallel()
	want := errors.New("output unavailable")
	timeline := &session.SessionTimeline{
		Session: &db.Session{ID: 1, Branch: "work", State: db.StatePaused},
	}
	err := renderSessionTimeline(historyErrorWriter{err: want}, timeline, 0)
	if !errors.Is(err, want) {
		t.Fatalf("renderSessionTimeline() error = %v, want %v", err, want)
	}
}

type historyErrorWriter struct {
	err error
}

func (w historyErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
