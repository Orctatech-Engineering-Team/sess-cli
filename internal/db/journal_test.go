package db

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func trackedTestSession(t *testing.T, database *DB, project *Project) *Session {
	t.Helper()
	s, err := database.CreateTrackedSession(&Session{
		ProjectID:  project.ID,
		Branch:     "feature/journal",
		IssueID:    "42",
		IssueTitle: "Journal",
		BranchType: "feature",
		StartTime:  time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.FixedZone("offset", 7200)),
	})
	if err != nil {
		t.Fatalf("create tracked session: %v", err)
	}
	return s
}

func pausedJournalSession(s *Session, at time.Time) *Session {
	after := *s
	after.State = StatePaused
	after.PauseTime = &at
	after.CurrentSliceStart = nil
	after.TotalElapsed += int64(time.Minute)
	return &after
}

func resumedJournalSession(s *Session, at time.Time) *Session {
	after := *s
	after.State = StateActive
	after.CurrentSliceStart = &at
	return &after
}

func endedJournalSession(s *Session, at time.Time) *Session {
	after := *s
	after.State = StateEnded
	after.CurrentSliceStart = nil
	after.EndTime = &at
	after.TotalElapsed += int64(time.Minute)
	number := int64(42)
	after.PRNumber = &number
	after.PRURL = "https://github.com/example/repo/pull/42"
	return &after
}

func TestJournalRoundTripAcrossReopen(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := trackedTestSession(t, database, project)
	if s.State != StateActive || s.TotalElapsed != 0 ||
		s.CurrentSliceStart == nil || !s.CurrentSliceStart.Equal(s.StartTime) {
		t.Fatalf("invalid tracked start: %#v", s)
	}
	at := s.StartTime.Add(time.Hour).In(time.FixedZone("other", -18000))
	paused := pausedJournalSession(s, at)
	resumed := resumedJournalSession(paused, at)
	ended := endedJournalSession(resumed, at.Add(-time.Minute))
	steps := []*Session{s, paused, resumed, ended}
	for i := 1; i < len(steps); i++ {
		if err := database.TransitionSession(steps[i-1], steps[i]); err != nil {
			t.Fatalf("transition %d: %v", i, err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(database.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetSessionByID(s.ID)
	if err != nil || got == nil || !sameSession(got, ended) {
		t.Fatalf("reopened session = %#v, err = %v, want %#v", got, err, ended)
	}
	events, err := reopened.ListSessionEvents(s.ID)
	if err != nil || len(events) != len(steps) {
		t.Fatalf("events = %#v, err = %v", events, err)
	}
	kinds := []string{EventSessionStarted, EventSessionPaused, EventSessionResumed, EventSessionEnded}
	times := []time.Time{s.StartTime, *paused.PauseTime, *resumed.CurrentSliceStart, *ended.EndTime}
	for i, event := range events {
		if event.SessionID != s.ID || event.Kind != kinds[i] || event.State != steps[i].State ||
			event.TotalElapsed != steps[i].TotalElapsed || !event.OccurredAt.Equal(times[i]) {
			t.Fatalf("event %d = %#v, want kind %q at %v", i, event, kinds[i], times[i])
		}
		if event.OccurredAt.Location() != time.UTC {
			t.Fatalf("event %d not stored in UTC: %v", i, event.OccurredAt)
		}
		if i > 0 && event.ID <= events[i-1].ID {
			t.Fatalf("events not ordered by ID: %#v", events)
		}
	}
	for _, stamp := range []*time.Time{&got.StartTime, got.PauseTime, got.EndTime} {
		if stamp.Location() != time.UTC {
			t.Fatalf("session timestamp not stored in UTC: %v", stamp)
		}
	}
	snapshot, journal, err := reopened.ReadSessionTimeline(s.ID)
	if err != nil || !sameSession(snapshot, got) || !reflect.DeepEqual(journal, events) {
		t.Fatalf("timeline = %#v, %#v, err = %v", snapshot, journal, err)
	}
	missing, err := reopened.GetSessionByID(s.ID + 100)
	if err != nil || missing != nil {
		t.Fatalf("missing session = %#v, err = %v", missing, err)
	}
	missing, journal, err = reopened.ReadSessionTimeline(s.ID + 100)
	if err != nil || missing != nil || journal != nil {
		t.Fatalf("missing timeline = %#v, %#v, err = %v", missing, journal, err)
	}
}

func TestLegacySessionWritesDoNotFabricateEvents(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := newTestSession(t, database, project, "feature/legacy", StateActive)
	paused := pausedJournalSession(s, time.Now())
	if err := database.UpdateSession(paused); err != nil {
		t.Fatal(err)
	}
	events, err := database.ListSessionEvents(s.ID)
	if err != nil || len(events) != 0 {
		t.Fatalf("legacy events = %#v, err = %v, want none", events, err)
	}
	if err := database.CompleteSession(paused, endedSession(paused)); err != nil {
		t.Fatal(err)
	}
	events, err = database.ListSessionEvents(s.ID)
	if err != nil || len(events) != 1 || events[0].Kind != EventSessionEnded {
		t.Fatalf("completion events = %#v, err = %v, want only ended", events, err)
	}
}

func TestJournalRollsBackOnEventOrProjectFailure(t *testing.T) {
	failures := []struct {
		name    string
		trigger string
	}{
		{
			name: "event",
			trigger: `CREATE TRIGGER fail_event BEFORE INSERT ON session_events
				BEGIN SELECT RAISE(ABORT, 'event blocked'); END`,
		},
		{
			name: "project",
			trigger: `CREATE TRIGGER fail_project BEFORE UPDATE ON projects
				BEGIN SELECT RAISE(ABORT, 'project blocked'); END`,
		},
	}
	for _, action := range []string{"start", "pause", "end", "paused end"} {
		for _, failure := range failures {
			t.Run(action+"/"+failure.name, func(t *testing.T) {
				database := newTestDB(t)
				project := newTestProject(t, database)
				var s *Session
				if action != "start" {
					s = trackedTestSession(t, database, project)
					if action == "paused end" {
						paused := pausedJournalSession(s, time.Now())
						if err := database.TransitionSession(s, paused); err != nil {
							t.Fatal(err)
						}
						s = paused
					}
					if err := database.CreateEndWorkflow(testEndWorkflow(s)); err != nil {
						t.Fatal(err)
					}
				}
				beforeSessions, err := database.ListSessions(project.ID)
				if err != nil {
					t.Fatal(err)
				}
				beforeProject, err := database.GetProjectByID(project.ID)
				if err != nil {
					t.Fatal(err)
				}
				var beforeEvents []*SessionEvent
				if s != nil {
					beforeEvents, err = database.ListSessionEvents(s.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
				if _, err := database.conn.Exec(failure.trigger); err != nil {
					t.Fatal(err)
				}
				if action == "start" {
					input := &Session{ProjectID: project.ID, Branch: "feature/new", StartTime: time.Now()}
					original := *input
					_, err = database.CreateTrackedSession(input)
					if !reflect.DeepEqual(*input, original) {
						t.Fatalf("failed start mutated caller: %#v", input)
					}
				} else {
					after := pausedJournalSession(s, time.Now())
					if action != "pause" {
						after = endedJournalSession(s, time.Now())
					}
					err = database.TransitionSession(s, after)
				}
				if err == nil {
					t.Fatal("operation succeeded despite failure trigger")
				}
				afterSessions, err := database.ListSessions(project.ID)
				if err != nil || !reflect.DeepEqual(afterSessions, beforeSessions) {
					t.Fatalf("sessions changed after rollback: %#v, err = %v", afterSessions, err)
				}
				afterProject, err := database.GetProjectByID(project.ID)
				if err != nil || !reflect.DeepEqual(afterProject, beforeProject) {
					t.Fatalf("project changed after rollback: %#v, err = %v", afterProject, err)
				}
				if s == nil {
					var count int
					if err := database.conn.QueryRow(`SELECT COUNT(*) FROM session_events`).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Fatalf("failed start left %d events", count)
					}
					return
				}
				afterEvents, err := database.ListSessionEvents(s.ID)
				if err != nil || !reflect.DeepEqual(afterEvents, beforeEvents) {
					t.Fatalf("events changed after rollback: %#v, err = %v", afterEvents, err)
				}
				checkpoint, err := database.GetEndWorkflow(s.ID)
				if err != nil || !reflect.DeepEqual(checkpoint, testEndWorkflow(s)) {
					t.Fatalf("checkpoint changed after rollback: %#v, err = %v", checkpoint, err)
				}
			})
		}
	}
}

func TestTransitionSessionRejectsStaleAndChangedIdentity(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Session)
	}{
		{name: "ID", edit: func(s *Session) { s.ID++ }},
		{name: "project", edit: func(s *Session) { s.ProjectID++ }},
		{name: "branch", edit: func(s *Session) { s.Branch = "other" }},
		{name: "issue ID", edit: func(s *Session) { s.IssueID = "other" }},
		{name: "issue title", edit: func(s *Session) { s.IssueTitle = "other" }},
		{name: "branch type", edit: func(s *Session) { s.BranchType = "other" }},
		{name: "start", edit: func(s *Session) { s.StartTime = s.StartTime.Add(time.Second) }},
		{name: "elapsed", edit: func(s *Session) { s.TotalElapsed++ }},
		{name: "slice", edit: func(s *Session) { at := s.StartTime.Add(time.Second); s.CurrentSliceStart = &at }},
		{name: "pause", edit: func(s *Session) { at := s.StartTime; s.PauseTime = &at }},
		{name: "end", edit: func(s *Session) { at := s.StartTime; s.EndTime = &at }},
		{name: "PR number", edit: func(s *Session) { n := int64(99); s.PRNumber = &n }},
		{name: "PR URL", edit: func(s *Session) { s.PRURL = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := trackedTestSession(t, database, project)
			stale := *s
			tt.edit(&stale)
			after := pausedJournalSession(&stale, time.Now())
			if err := database.TransitionSession(&stale, after); err == nil {
				t.Fatal("stale transition succeeded")
			}
			after = pausedJournalSession(s, time.Now())
			tt.edit(after)
			if !sameSessionIdentity(s, after) {
				if err := database.TransitionSession(s, after); err == nil {
					t.Fatal("changed identity succeeded")
				}
			}
			events, err := database.ListSessionEvents(s.ID)
			if err != nil || len(events) != 1 {
				t.Fatalf("events = %#v, err = %v", events, err)
			}
		})
	}
}

func TestTransitionSessionRepeatedAndOldSnapshotsEmitNoDuplicates(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	start := trackedTestSession(t, database, project)
	at := start.StartTime.Add(time.Hour)
	pause := pausedJournalSession(start, at)
	resume := resumedJournalSession(pause, at)
	end := endedJournalSession(resume, at)
	steps := []*Session{start, pause, resume, end}
	for i := 1; i < len(steps); i++ {
		if err := database.TransitionSession(steps[i-1], steps[i]); err != nil {
			t.Fatal(err)
		}
		if err := database.TransitionSession(steps[i-1], steps[i]); err == nil {
			t.Fatal("duplicate transition succeeded")
		}
	}
	if err := database.TransitionSession(start, pause); err == nil {
		t.Fatal("old active snapshot overwrote ended session")
	}
	events, err := database.ListSessionEvents(start.ID)
	if err != nil || len(events) != 4 {
		t.Fatalf("events = %#v, err = %v, want four", events, err)
	}
}

func TestTransitionSessionAllowsOnlyLifecycleEdges(t *testing.T) {
	states := []string{StateActive, StatePaused, StateEnded}
	for _, beforeState := range states {
		for _, afterState := range states {
			t.Run(beforeState+"/"+afterState, func(t *testing.T) {
				database := newTestDB(t)
				project := newTestProject(t, database)
				s := newTestSession(t, database, project, "feature/legacy", beforeState)
				at := time.Now()
				after := *s
				after.State = afterState
				switch afterState {
				case StateActive:
					after.CurrentSliceStart = &at
				case StatePaused:
					after.PauseTime = &at
				case StateEnded:
					after.EndTime = &at
				}
				allowed := beforeState == StateActive && afterState == StatePaused ||
					beforeState == StatePaused && afterState == StateActive ||
					beforeState != StateEnded && afterState == StateEnded
				err := database.TransitionSession(s, &after)
				if (err == nil) != allowed {
					t.Fatalf("allowed = %v, transition error = %v", allowed, err)
				}
				events, err := database.ListSessionEvents(s.ID)
				wantCount := 0
				if allowed {
					wantCount = 1
				}
				if err != nil || len(events) != wantCount {
					t.Fatalf("events = %#v, err = %v, want %d", events, err, wantCount)
				}
			})
		}
	}
}

func TestTransitionSessionRejectsInvalidAfterState(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Session)
	}{
		{name: "missing pause time", edit: func(s *Session) { s.PauseTime = nil }},
		{name: "zero pause time", edit: func(s *Session) { s.PauseTime = &time.Time{} }},
		{name: "paused active slice", edit: func(s *Session) { s.CurrentSliceStart = &s.StartTime }},
		{name: "paused end time", edit: func(s *Session) { s.EndTime = &s.StartTime }},
		{name: "negative elapsed", edit: func(s *Session) { s.TotalElapsed = -1 }},
		{name: "decreased elapsed", edit: func(s *Session) { s.TotalElapsed = 0 }},
		{name: "unknown state", edit: func(s *Session) { s.State = "unknown" }},
		{name: "missing end time", edit: func(s *Session) { s.State = StateEnded; s.EndTime = nil }},
		{name: "ended active slice", edit: func(s *Session) {
			s.State = StateEnded
			s.EndTime = &s.StartTime
			s.CurrentSliceStart = &s.StartTime
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			s := trackedTestSession(t, database, project)
			s.TotalElapsed = 1
			if err := database.UpdateSession(s); err != nil {
				t.Fatal(err)
			}
			after := pausedJournalSession(s, time.Now())
			tt.edit(after)
			if err := database.TransitionSession(s, after); err == nil {
				t.Fatal("invalid transition succeeded")
			}
			got, err := database.GetSessionByID(s.ID)
			if err != nil || !sameSession(got, s) {
				t.Fatalf("invalid transition changed session: %#v, err = %v", got, err)
			}
			events, err := database.ListSessionEvents(s.ID)
			if err != nil || len(events) != 1 {
				t.Fatalf("invalid transition added events: %#v, err = %v", events, err)
			}
		})
	}
	database := newTestDB(t)
	for _, pair := range [][2]*Session{{nil, nil}, {&Session{}, nil}, {nil, &Session{}}} {
		if err := database.TransitionSession(pair[0], pair[1]); err == nil {
			t.Fatal("nil transition succeeded")
		}
	}
}

func TestCreateTrackedSessionGuardsExistingAndCompetingStarts(t *testing.T) {
	for _, state := range []string{StateActive, StatePaused, StateEnded} {
		t.Run(state, func(t *testing.T) {
			database := newTestDB(t)
			project := newTestProject(t, database)
			newTestSession(t, database, project, "feature/legacy", state)
			_, err := database.CreateTrackedSession(&Session{
				ProjectID: project.ID, Branch: "feature/new", StartTime: time.Now(),
			})
			if (err == nil) != (state == StateEnded) {
				t.Fatalf("existing %s: create error = %v", state, err)
			}
		})
	}
	t.Run("competing connections", func(t *testing.T) {
		database := newTestDB(t)
		project := newTestProject(t, database)
		other, err := Open(database.path)
		if err != nil {
			t.Fatal(err)
		}
		defer other.Close()
		start := make(chan struct{})
		results := make(chan error, 2)
		for i, connection := range []*DB{database, other} {
			go func() {
				<-start
				_, err := connection.CreateTrackedSession(&Session{
					ProjectID: project.ID, Branch: fmt.Sprintf("feature/%d", i), StartTime: time.Now(),
				})
				results <- err
			}()
		}
		close(start)
		successes := 0
		for range 2 {
			if <-results == nil {
				successes++
			}
		}
		sessions, err := database.ListSessions(project.ID)
		if err != nil || successes != 1 || len(sessions) != 1 {
			t.Fatalf("successes = %d, sessions = %#v, err = %v", successes, sessions, err)
		}
		events, err := database.ListSessionEvents(sessions[0].ID)
		if err != nil || len(events) != 1 {
			t.Fatalf("events = %#v, err = %v", events, err)
		}
	})
}

func TestSyncSessionRejectsStaleAccountingWithoutEvents(t *testing.T) {
	database := newTestDB(t)
	project := newTestProject(t, database)
	s := trackedTestSession(t, database, project)
	projectBefore, err := database.GetProjectByID(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	synced := *s
	at := s.StartTime.Add(time.Minute)
	synced.CurrentSliceStart = &at
	synced.TotalElapsed = int64(time.Minute)
	if err := database.SyncSession(s, &synced); err != nil {
		t.Fatal(err)
	}
	if err := database.SyncSession(s, &synced); err == nil {
		t.Fatal("stale sync succeeded")
	}
	if err := database.TransitionSession(s, pausedJournalSession(s, at)); err == nil {
		t.Fatal("pre-sync transition succeeded")
	}
	got, err := database.GetSessionByID(s.ID)
	if err != nil || !sameSession(got, &synced) {
		t.Fatalf("synced session = %#v, err = %v", got, err)
	}
	events, err := database.ListSessionEvents(s.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("sync fabricated events: %#v, err = %v", events, err)
	}
	projectAfter, err := database.GetProjectByID(project.ID)
	if err != nil || !reflect.DeepEqual(projectAfter, projectBefore) {
		t.Fatalf("sync touched project: %#v, err = %v", projectAfter, err)
	}
	paused := pausedJournalSession(&synced, at)
	if err := database.TransitionSession(&synced, paused); err != nil {
		t.Fatal(err)
	}
	if err := database.SyncSession(&synced, &synced); err == nil {
		t.Fatal("sync overwrote paused session")
	}
}

func TestReadSessionTimelineRemainsCoherentDuringTransitions(t *testing.T) {
	database := newTestDB(t)
	if _, err := database.conn.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		t.Fatal(err)
	}
	project := newTestProject(t, database)
	s := trackedTestSession(t, database, project)
	writer, err := Open(database.path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	var wg sync.WaitGroup
	errors := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		current := s
		for i := range 100 {
			at := s.StartTime.Add(time.Duration(i) * time.Minute)
			after := pausedJournalSession(current, at)
			if current.State == StatePaused {
				after = resumedJournalSession(current, at)
			}
			if err := writer.TransitionSession(current, after); err != nil {
				errors <- err
				return
			}
			current = after
		}
	}()
	for range 100 {
		snapshot, events, err := database.ReadSessionTimeline(s.ID)
		if err != nil || snapshot == nil || len(events) == 0 {
			t.Errorf("read timeline = %#v, %#v, err = %v", snapshot, events, err)
			break
		}
		last := events[len(events)-1]
		if last.State != snapshot.State || last.TotalElapsed != snapshot.TotalElapsed {
			t.Errorf("incoherent snapshot: %#v, last event %#v", snapshot, last)
			break
		}
	}
	wg.Wait()
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
}
