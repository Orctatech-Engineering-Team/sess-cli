package session

import (
	"fmt"
	"sort"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
)

// SessionTimeline is the recorded lifecycle detail; Session holds authoritative elapsed time.
type SessionTimeline struct {
	Session         *db.Session
	Events          []*db.SessionEvent
	Intervals       []ActiveInterval
	HistoryComplete bool
}

// ActiveInterval is anchored at a recorded start or resume. End is nil while ongoing.
type ActiveInterval struct {
	Start    time.Time
	End      *time.Time
	Duration time.Duration
}

// GetSessionTimeline reads session state and journal entries from one database snapshot.
func (m *Manager) GetSessionTimeline(sessionID int64) (*SessionTimeline, error) {
	session, events, err := m.db.ReadSessionTimeline(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get timeline for session %d: %w", sessionID, err)
	}
	if session == nil {
		return nil, fmt.Errorf("session %d not found", sessionID)
	}
	return buildSessionTimeline(session, events, time.Now()), nil
}

func buildSessionTimeline(session *db.Session, events []*db.SessionEvent, now time.Time) *SessionTimeline {
	ordered := append([]*db.SessionEvent{}, events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})
	timeline := &SessionTimeline{
		Session:   session,
		Events:    ordered,
		Intervals: []ActiveInterval{},
	}
	complete := len(ordered) > 0
	state := ""
	var open *time.Time
	var previousID int64
	var previousTime time.Time
	for i, event := range ordered {
		next, valid := timelineTransition(state, event.Kind)
		if !valid || event.State != next || event.SessionID != session.ID {
			complete = false
		}
		if i > 0 && (event.ID <= previousID || event.OccurredAt.Before(previousTime)) {
			complete = false
		}
		previousID = event.ID
		previousTime = event.OccurredAt
		state = next

		switch event.Kind {
		case db.EventSessionStarted, db.EventSessionResumed:
			start := event.OccurredAt
			open = &start
		case db.EventSessionPaused, db.EventSessionEnded:
			if open != nil {
				end := event.OccurredAt
				timeline.Intervals = append(timeline.Intervals, ActiveInterval{
					Start:    *open,
					End:      &end,
					Duration: intervalDuration(*open, end),
				})
				open = nil
			}
		default:
			open = nil
		}
	}
	if open != nil && session.State == db.StateActive {
		if now.Before(*open) {
			complete = false
		}
		timeline.Intervals = append(timeline.Intervals, ActiveInterval{
			Start:    *open,
			Duration: intervalDuration(*open, now),
		})
	}
	timeline.HistoryComplete = complete && state == session.State
	return timeline
}

func timelineTransition(state, kind string) (string, bool) {
	switch kind {
	case db.EventSessionStarted:
		return db.StateActive, state == ""
	case db.EventSessionPaused:
		return db.StatePaused, state == db.StateActive
	case db.EventSessionResumed:
		return db.StateActive, state == db.StatePaused
	case db.EventSessionEnded:
		return db.StateEnded, state == db.StateActive || state == db.StatePaused
	default:
		return "", false
	}
}

func intervalDuration(start, end time.Time) time.Duration {
	duration := end.Sub(start)
	if duration < 0 {
		return 0
	}
	return duration
}
