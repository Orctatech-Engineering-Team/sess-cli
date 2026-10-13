package session

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

func TestBuildSessionTimeline(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	event := func(id int64, kind, state string, offset time.Duration) *db.SessionEvent {
		return &db.SessionEvent{
			ID:         id,
			SessionID:  42,
			Kind:       kind,
			State:      state,
			OccurredAt: base.Add(offset),
		}
	}
	start := event(1, db.EventSessionStarted, db.StateActive, 0)
	pause := event(2, db.EventSessionPaused, db.StatePaused, time.Minute)
	resume := event(3, db.EventSessionResumed, db.StateActive, 2*time.Minute)
	end := event(4, db.EventSessionEnded, db.StateEnded, 3*time.Minute)
	tests := []struct {
		name      string
		state     string
		events    []*db.SessionEvent
		complete  bool
		durations []time.Duration
		open      bool
	}{
		{
			name: "active", state: db.StateActive, events: []*db.SessionEvent{start},
			complete: true, durations: []time.Duration{5 * time.Minute}, open: true,
		},
		{
			name: "paused", state: db.StatePaused, events: []*db.SessionEvent{start, pause},
			complete: true, durations: []time.Duration{time.Minute},
		},
		{
			name: "active end", state: db.StateEnded, events: []*db.SessionEvent{start, pause, resume, end},
			complete: true, durations: []time.Duration{time.Minute, time.Minute},
		},
		{
			name: "paused end", state: db.StateEnded,
			events:   []*db.SessionEvent{start, pause, event(3, db.EventSessionEnded, db.StateEnded, 3*time.Minute)},
			complete: true, durations: []time.Duration{time.Minute},
		},
		{
			name: "repeated slices", state: db.StateActive,
			events: []*db.SessionEvent{
				start, pause, resume, event(4, db.EventSessionPaused, db.StatePaused, 3*time.Minute),
				event(5, db.EventSessionResumed, db.StateActive, 4*time.Minute),
			},
			complete: true, durations: []time.Duration{time.Minute, time.Minute, time.Minute}, open: true,
		},
		{
			name: "equal timestamps ordered by ID", state: db.StateEnded,
			events: []*db.SessionEvent{
				event(3, db.EventSessionEnded, db.StateEnded, 0),
				event(1, db.EventSessionStarted, db.StateActive, 0),
				event(2, db.EventSessionPaused, db.StatePaused, 0),
			},
			complete: true, durations: []time.Duration{0},
		},
		{name: "legacy active no events", state: db.StateActive},
		{name: "legacy pause", state: db.StatePaused, events: []*db.SessionEvent{pause}},
		{name: "legacy end", state: db.StateEnded, events: []*db.SessionEvent{end}},
		{
			name: "legacy resumed interval only", state: db.StateEnded,
			events: []*db.SessionEvent{pause, resume, end}, durations: []time.Duration{time.Minute},
		},
		{
			name: "legacy resume first", state: db.StateActive, events: []*db.SessionEvent{resume},
			durations: []time.Duration{3 * time.Minute}, open: true,
		},
		{
			name: "state mismatch", state: db.StatePaused, events: []*db.SessionEvent{start},
		},
		{
			name: "invalid repeated pause", state: db.StatePaused,
			events:    []*db.SessionEvent{start, pause, event(3, db.EventSessionPaused, db.StatePaused, 2*time.Minute)},
			durations: []time.Duration{time.Minute},
		},
		{
			name: "invalid event state", state: db.StatePaused,
			events:    []*db.SessionEvent{start, event(2, db.EventSessionPaused, db.StateActive, time.Minute)},
			durations: []time.Duration{time.Minute},
		},
		{
			name: "backwards close", state: db.StateEnded,
			events:    []*db.SessionEvent{start, event(2, db.EventSessionEnded, db.StateEnded, -time.Minute)},
			durations: []time.Duration{0},
		},
		{
			name: "backwards open", state: db.StateActive,
			events:    []*db.SessionEvent{event(1, db.EventSessionStarted, db.StateActive, 6*time.Minute)},
			durations: []time.Duration{0}, open: true,
		},
		{
			name: "backwards resume", state: db.StateEnded,
			events: []*db.SessionEvent{
				start, pause, event(3, db.EventSessionResumed, db.StateActive, 0), end,
			},
			durations: []time.Duration{time.Minute, 3 * time.Minute},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			record := &db.Session{ID: 42, State: tt.state, TotalElapsed: int64(17 * time.Minute)}
			got := buildSessionTimeline(record, tt.events, base.Add(5*time.Minute))
			if got.HistoryComplete != tt.complete {
				t.Errorf("history complete = %v, want %v", got.HistoryComplete, tt.complete)
			}
			if got.Session != record || got.Session.TotalElapsed != int64(17*time.Minute) {
				t.Fatal("timeline replaced authoritative aggregate")
			}
			if len(got.Intervals) != len(tt.durations) {
				t.Fatalf("interval count = %d, want %d", len(got.Intervals), len(tt.durations))
			}
			for i, interval := range got.Intervals {
				if interval.Duration != tt.durations[i] {
					t.Errorf("interval %d duration = %v, want %v", i, interval.Duration, tt.durations[i])
				}
				wantOpen := tt.open && i == len(got.Intervals)-1
				if (interval.End == nil) != wantOpen {
					t.Errorf("interval %d open = %v, want %v", i, interval.End == nil, wantOpen)
				}
				anchored := false
				for _, e := range tt.events {
					if e.Kind == db.EventSessionStarted || e.Kind == db.EventSessionResumed {
						anchored = anchored || interval.Start.Equal(e.OccurredAt)
					}
				}
				if !anchored {
					t.Errorf("interval %d has invented start %v", i, interval.Start)
				}
			}
			for i := 1; i < len(got.Events); i++ {
				if got.Events[i].ID < got.Events[i-1].ID {
					t.Fatal("events are not in ID order")
				}
			}
		})
	}
}

func TestManagerGetSessionTimelineMissing(t *testing.T) {
	mgr, _ := newTestManager(t)
	if timeline, err := mgr.GetSessionTimeline(999); err == nil || timeline != nil {
		t.Fatalf("missing timeline = %#v, err = %v, want nil and error", timeline, err)
	}
}

func TestManagerGetSessionTimelineConcurrentTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(database)
	project := newTestProject(t, mgr)
	s, err := mgr.StartSession(project.ID, "feature/concurrent", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for range 25 {
			if _, err := mgr.PauseSession(project.ID); err != nil {
				done <- err
				return
			}
			if _, err := mgr.ResumeSession(project.ID); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for range 50 {
		timeline, err := mgr.GetSessionTimeline(s.ID)
		if err != nil {
			t.Errorf("read concurrent timeline: %v", err)
			break
		}
		if !timeline.HistoryComplete {
			t.Errorf("incoherent snapshot: state %q, %d events", timeline.Session.State, len(timeline.Events))
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestManagerGetSessionTimelinePersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sess.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(database)
	project := newTestProject(t, mgr)
	s, err := mgr.StartSession(project.ID, "feature/timeline", "", "", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.PauseSession(project.ID); err != nil {
		t.Fatal(err)
	}
	before, err := mgr.GetSessionTimeline(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})
	after, err := NewManager(database).GetSessionTimeline(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !before.HistoryComplete || !after.HistoryComplete || len(after.Events) != 2 || len(after.Intervals) != 1 {
		t.Fatalf("persisted timeline = %#v, want complete start/pause history", after)
	}
	if after.Session.TotalElapsed != before.Session.TotalElapsed ||
		after.Intervals[0].Duration != before.Intervals[0].Duration {
		t.Fatal("persisted elapsed or interval changed")
	}
	for i := range before.Events {
		if *before.Events[i] != *after.Events[i] {
			t.Errorf("persisted event %d changed", i)
		}
	}
}
