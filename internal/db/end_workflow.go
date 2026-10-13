package db

import (
	"database/sql"
	"fmt"
)

const (
	EndPrepared       = "prepared"
	EndCommitted      = "committed"
	EndRebased        = "rebased"
	EndPushed         = "pushed"
	EndPRCreated      = "pr_created"
	EndBaseCheckedOut = "base_checked_out"
)

// EndWorkflow persists end-workflow inputs and the latest completed phase.
type EndWorkflow struct {
	SessionID     int64
	Branch        string
	BaseBranch    string
	Repository    string // Canonical GitHub host/owner/repo.
	RemoteID      string // SHA256 origin URL fingerprint, never the raw URL.
	CommitMessage string
	EndWithoutPR  bool
	PRTitle       string
	PRBody        string
	Phase         string
	Head          string
	RebaseTarget  string
	PRNumber      *int64
	PRURL         string
}

func endPhaseRank(phase string) int {
	switch phase {
	case EndPrepared:
		return 0
	case EndCommitted:
		return 1
	case EndRebased:
		return 2
	case EndPushed:
		return 3
	case EndPRCreated:
		return 4
	case EndBaseCheckedOut:
		return 5
	default:
		return -1
	}
}

// GetEndWorkflow returns nil when the session has no checkpoint.
func (db *DB) GetEndWorkflow(sessionID int64) (*EndWorkflow, error) {
	var w EndWorkflow
	err := db.conn.QueryRow(`
		SELECT session_id, branch, base_branch, commit_message, end_without_pr,
		       pr_title, pr_body, phase, head, rebase_target, pr_number, pr_url,
		       repository, remote_id
		FROM end_workflows WHERE session_id = ?
	`, sessionID).Scan(
		&w.SessionID,
		&w.Branch,
		&w.BaseBranch,
		&w.CommitMessage,
		&w.EndWithoutPR,
		&w.PRTitle,
		&w.PRBody,
		&w.Phase,
		&w.Head,
		&w.RebaseTarget,
		&w.PRNumber,
		&w.PRURL,
		&w.Repository,
		&w.RemoteID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get end workflow for session %d: %w", sessionID, err)
	}
	return &w, nil
}

// CreateEndWorkflow inserts prepared inputs without replacing an existing checkpoint.
func (db *DB) CreateEndWorkflow(w *EndWorkflow) error {
	if w == nil {
		return fmt.Errorf("create end workflow: workflow is nil")
	}
	if w.Phase != EndPrepared {
		return fmt.Errorf("create end workflow for session %d: phase %q must be %q", w.SessionID, w.Phase, EndPrepared)
	}
	if w.RebaseTarget != "" {
		return fmt.Errorf("create end workflow for session %d: rebase target must be empty", w.SessionID)
	}
	result, err := db.conn.Exec(`
		INSERT INTO end_workflows (
			session_id, branch, base_branch, commit_message, end_without_pr,
			pr_title, pr_body, phase, head, rebase_target, pr_number, pr_url,
			repository, remote_id
		)
		SELECT id, branch, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		FROM sessions WHERE id = ? AND branch = ? AND state IN ('active', 'paused')
	`,
		w.BaseBranch,
		w.CommitMessage,
		w.EndWithoutPR,
		w.PRTitle,
		w.PRBody,
		w.Phase,
		w.Head,
		w.RebaseTarget,
		w.PRNumber,
		w.PRURL,
		w.Repository,
		w.RemoteID,
		w.SessionID,
		w.Branch,
	)
	if err != nil {
		return fmt.Errorf("create end workflow for session %d on branch %q: %w", w.SessionID, w.Branch, err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("create end workflow for session %d on branch %q: active/paused session not found: %w",
			w.SessionID, w.Branch, err)
	}
	return nil
}

// UpdateEndWorkflow advances a checkpoint while preserving its identity and saved inputs.
// Equal phases are allowed so retries can persist recovered head or PR metadata.
func (db *DB) UpdateEndWorkflow(w *EndWorkflow) error {
	if w == nil {
		return fmt.Errorf("update end workflow: workflow is nil")
	}
	rank := endPhaseRank(w.Phase)
	if rank < 0 {
		return fmt.Errorf("update end workflow for session %d: invalid phase %q", w.SessionID, w.Phase)
	}
	if w.RebaseTarget != "" && rank < endPhaseRank(EndCommitted) {
		return fmt.Errorf("update end workflow for session %d: rebase target requires committed phase or later", w.SessionID)
	}
	result, err := db.conn.Exec(`
		UPDATE end_workflows SET phase = ?, head = ?, rebase_target = ?, pr_number = ?, pr_url = ?
		WHERE session_id = ? AND branch = ? AND base_branch = ?
		  AND commit_message = ? AND end_without_pr = ? AND pr_title = ? AND pr_body = ?
		  AND repository = ? AND remote_id = ?
		  AND (phase IN ('prepared', 'committed', 'rebased') OR head = ?)
		  AND (rebase_target = '' OR rebase_target = ?)
		  AND (pr_number IS NULL OR pr_number = ?)
		  AND CASE phase
		      WHEN 'prepared' THEN 0 WHEN 'committed' THEN 1 WHEN 'rebased' THEN 2
		      WHEN 'pushed' THEN 3 WHEN 'pr_created' THEN 4 WHEN 'base_checked_out' THEN 5
		      ELSE 6 END <= ?
		  AND EXISTS (
		      SELECT 1 FROM sessions
		      WHERE id = end_workflows.session_id AND branch = end_workflows.branch
		        AND state IN ('active', 'paused')
		  )
	`,
		w.Phase,
		w.Head,
		w.RebaseTarget,
		w.PRNumber,
		w.PRURL,
		w.SessionID,
		w.Branch,
		w.BaseBranch,
		w.CommitMessage,
		w.EndWithoutPR,
		w.PRTitle,
		w.PRBody,
		w.Repository,
		w.RemoteID,
		w.Head,
		w.RebaseTarget,
		w.PRNumber,
		rank,
	)
	if err != nil {
		return fmt.Errorf("update end workflow for session %d to phase %q: %w", w.SessionID, w.Phase, err)
	}
	if err := requireOneRow(result); err != nil {
		return fmt.Errorf("update end workflow for session %d to phase %q: checkpoint missing, stale phase, or immutable inputs/session identity/head/rebase target/PR number changed: %w",
			w.SessionID, w.Phase, err)
	}
	return nil
}
