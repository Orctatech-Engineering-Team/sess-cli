package sess

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

const contextProtocolVersion = "sess.agent-work/v0"

type contextCommandDeps struct {
	getwd  func() (string, error)
	dbPath func() (string, error)
	now    func() time.Time
}

type contextEnvelope struct {
	ProtocolVersion string      `json:"protocol_version"`
	Kind            string      `json:"kind"`
	GeneratedAt     string      `json:"generated_at"`
	Data            contextData `json:"data"`
}

type contextData struct {
	Availability    string                  `json:"availability"`
	Project         *contextProject         `json:"project"`
	Session         *contextSession         `json:"session"`
	RepositoryState *contextRepositoryState `json:"repository_state"`
	ActiveAttempts  []*contextAttempt       `json:"active_attempts"`
	LatestHandoff   *contextHandoff         `json:"latest_handoff"`
	Checks          []*contextCheck         `json:"checks"`
	Warnings        []contextWarning        `json:"warnings"`
}

type contextProject struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	BaseBranch string `json:"base_branch"`
}

type contextSession struct {
	ID     string        `json:"id"`
	State  string        `json:"state"`
	Branch string        `json:"branch"`
	Issue  *contextIssue `json:"issue"`
}

type contextIssue struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type contextAttempt struct {
	ID                    string                  `json:"id"`
	SessionID             string                  `json:"session_id"`
	Participant           contextParticipant      `json:"participant"`
	Role                  string                  `json:"role"`
	State                 string                  `json:"state"`
	StartedAt             string                  `json:"started_at"`
	UpdatedAt             string                  `json:"updated_at"`
	FinishedAt            *string                 `json:"finished_at"`
	StartRepositoryState  contextRepositoryState  `json:"start_repository_state"`
	ResultRepositoryState *contextRepositoryState `json:"result_repository_state"`
	Claims                []contextClaim          `json:"claims"`
	BlockedReason         string                  `json:"blocked_reason"`
}

type contextParticipant struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Label          string `json:"label"`
	IdentitySource string `json:"identity_source"`
}

type contextClaim struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	CreatedAt string `json:"created_at"`
}

type contextCheck struct {
	ID              string                 `json:"id"`
	AttemptID       string                 `json:"attempt_id"`
	Name            string                 `json:"name"`
	ProcedureType   string                 `json:"procedure_type"`
	ProcedureValue  string                 `json:"procedure"`
	Outcome         string                 `json:"outcome"`
	StartedAt       string                 `json:"started_at"`
	FinishedAt      string                 `json:"finished_at"`
	ExitCode        *int                   `json:"exit_code"`
	RepositoryState contextRepositoryState `json:"repository_state"`
	Reproducibility string                 `json:"reproducibility"`
	Summary         string                 `json:"summary"`
}

type contextHandoff struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	AttemptID       string                 `json:"attempt_id"`
	CreatedAt       string                 `json:"created_at"`
	RepositoryState contextRepositoryState `json:"repository_state"`
	Completed       []string               `json:"completed"`
	Remaining       []string               `json:"remaining"`
	Risks           []string               `json:"risks"`
	NextAction      string                 `json:"next_action"`
	CheckIDs        []string               `json:"check_ids"`
	SupersedesID    string                 `json:"supersedes_id"`
}

type contextRepositoryState struct {
	Repository *string `json:"repository"`
	Branch     *string `json:"branch"`
	HeadOID    *string `json:"head_oid"`
	Worktree   string  `json:"worktree"`
	CapturedAt string  `json:"captured_at"`
}

type contextWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var contextCmd = newContextCommand(contextCommandDeps{
	getwd:  os.Getwd,
	dbPath: db.GetDefaultDBPath,
	now:    time.Now,
})

func newContextCommand(deps contextCommandDeps) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Show structured session context",
		Long:  "Emit read-only project, session, and repository context for humans, scripts, and coding agents.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !jsonOutput {
				return fmt.Errorf("context currently requires --json")
			}
			cwd, err := deps.getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			dbPath, err := deps.dbPath()
			if err != nil {
				return fmt.Errorf("get database path: %w", err)
			}
			return writeContext(cmd.Context(), cmd.OutOrStdout(), cwd, dbPath, deps.now().UTC())
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit the versioned Agent Work Protocol JSON document")
	return cmd
}

func writeContext(ctx stdcontext.Context, w io.Writer, cwd, dbPath string, now time.Time) error {
	generatedAt := now.Format(time.RFC3339Nano)
	data := contextData{
		Availability:   "no_repository",
		ActiveAttempts: make([]*contextAttempt, 0),
		Checks:         make([]*contextCheck, 0),
		Warnings:       make([]contextWarning, 0),
	}

	isRepo, err := git.IsRepo(ctx, cwd)
	if err != nil {
		return fmt.Errorf("check git repository: %w", err)
	}
	if isRepo {
		data.Availability = "untracked_project"
		data.RepositoryState, data.Warnings, err = readContextRepositoryState(ctx, cwd, generatedAt)
		if err != nil {
			return err
		}

		if _, err := os.Stat(dbPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("inspect database: %w", err)
			}
		} else if err := readContextSession(cwd, dbPath, generatedAt, &data); err != nil {
			return err
		}
	}

	envelope := contextEnvelope{
		ProtocolVersion: contextProtocolVersion,
		Kind:            "context",
		GeneratedAt:     generatedAt,
		Data:            data,
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(envelope); err != nil {
		return fmt.Errorf("write context JSON: %w", err)
	}
	return nil
}

func readContextSession(cwd, dbPath, generatedAt string, data *contextData) (err error) {
	database, err := db.OpenReadOnly(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	manager := session.NewManager(database)
	project, err := manager.GetProject(cwd)
	if err != nil {
		return fmt.Errorf("get tracked project: %w", err)
	}
	if project == nil {
		return nil
	}

	data.Availability = "idle_project"
	data.Project = &contextProject{
		ID:         contextRecordID("project", project.ID),
		Name:       project.Name,
		Path:       project.Path,
		BaseBranch: project.BaseBranch,
	}
	current, err := manager.GetActiveSession(project.ID)
	if err != nil {
		return fmt.Errorf("get current session: %w", err)
	}
	if current != nil {
		data.Availability = "session"
		data.Session = contextSessionFromDB(current)
		data.Warnings = append(data.Warnings, contextWarning{
			Code:    "protocol_history_unavailable",
			Message: "This session predates Agent Work Protocol records.",
		})
		if data.RepositoryState.Branch == nil || *data.RepositoryState.Branch != current.Branch {
			data.Warnings = append(data.Warnings, contextWarning{
				Code:    "session_branch_mismatch",
				Message: "The checked-out branch does not match the session branch.",
			})
		}
		attempts, err := database.ListActiveAttempts(current.ID)
		if err != nil {
			return fmt.Errorf("list active attempts: %w", err)
		}
		data.ActiveAttempts = make([]*contextAttempt, 0, len(attempts))
		now, _ := time.Parse(time.RFC3339Nano, generatedAt)
		for _, attempt := range attempts {
			data.ActiveAttempts = append(data.ActiveAttempts, contextAttemptFromDB(attempt))
			if attempt.State == db.AttemptActive && now.Sub(attempt.UpdatedAt) >= 24*time.Hour {
				data.Warnings = append(data.Warnings, contextWarning{
					Code:    "attempt_claim_stale",
					Message: fmt.Sprintf("Attempt %s has not been updated for at least 24 hours.", attempt.ID),
				})
			}
			for _, claim := range attempt.Claims {
				for _, other := range attempts {
					if other.ID <= attempt.ID {
						continue
					}
					for _, otherClaim := range other.Claims {
						if contextClaimsOverlap(claim.Type, claim.Value, otherClaim.Type, otherClaim.Value) {
							data.Warnings = append(data.Warnings, contextWarning{
								Code:    "attempt_claim_overlap",
								Message: fmt.Sprintf("Attempt %s overlaps %s on %s %q.", attempt.ID, other.ID, claim.Type, claim.Value),
							})
						}
					}
				}
			}
			checks, err := database.ListChecksForSession(current.ID)
			if err != nil {
				return fmt.Errorf("list checks: %w", err)
			}
			data.Checks = make([]*contextCheck, 0, len(checks))
			for _, check := range checks {
				data.Checks = append(data.Checks, contextCheckFromDB(check))
				if data.RepositoryState.HeadOID != nil &&
					check.RepositoryState.HeadOID != "" &&
					*data.RepositoryState.HeadOID != check.RepositoryState.HeadOID {
					data.Warnings = append(data.Warnings, contextWarning{
						Code:    "check_stale",
						Message: fmt.Sprintf("Check %s was recorded at a different repository revision.", check.ID),
					})
				}
			}
			handoffs, err := database.ListHandoffsForSession(current.ID)
			if err != nil {
				return fmt.Errorf("list handoffs: %w", err)
			}
			superseded := make(map[string]bool)
			for _, handoff := range handoffs {
				if handoff.SupersedesID != "" {
					superseded[handoff.SupersedesID] = true
				}
			}
			for i := len(handoffs) - 1; i >= 0; i-- {
				if superseded[handoffs[i].ID] {
					continue
				}
				data.LatestHandoff = contextHandoffFromDB(handoffs[i])
				if data.RepositoryState.HeadOID != nil &&
					handoffs[i].RepositoryState.HeadOID != "" &&
					*data.RepositoryState.HeadOID != handoffs[i].RepositoryState.HeadOID {
					data.Warnings = append(data.Warnings, contextWarning{
						Code:    "handoff_stale",
						Message: fmt.Sprintf("Handoff %s was recorded at a different repository revision.", handoffs[i].ID),
					})
				}
				break
			}
		}
	}
	return nil
}

func contextAttemptFromDB(attempt *db.Attempt) *contextAttempt {
	claims := make([]contextClaim, 0, len(attempt.Claims))
	for _, claim := range attempt.Claims {
		claims = append(claims, contextClaim{
			Type: claim.Type, Value: claim.Value,
			CreatedAt: claim.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return &contextAttempt{
		ID:                    contextRecordIDString("attempt", attempt.ID),
		SessionID:             contextRecordID("session", attempt.SessionID),
		Participant:           contextParticipant{ID: attempt.Participant.ID, Kind: attempt.Participant.Kind, Label: attempt.Participant.Label, IdentitySource: attempt.Participant.IdentitySource},
		Role:                  attempt.Role,
		State:                 attempt.State,
		StartedAt:             attempt.StartedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             attempt.UpdatedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt:            contextTimePointer(attempt.FinishedAt),
		StartRepositoryState:  *contextRepositoryStateFromDB(&attempt.StartRepositoryState),
		ResultRepositoryState: contextRepositoryStateFromDB(attempt.ResultRepositoryState),
		Claims:                claims,
		BlockedReason:         attempt.BlockedReason,
	}
}

func contextCheckFromDB(check *db.Check) *contextCheck {
	return &contextCheck{
		ID: check.ID, AttemptID: contextRecordIDString("attempt", check.AttemptID),
		Name: check.Name, ProcedureType: check.ProcedureType,
		ProcedureValue: check.ProcedureValue, Outcome: check.Outcome,
		StartedAt:       check.StartedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt:      check.FinishedAt.UTC().Format(time.RFC3339Nano),
		ExitCode:        check.ExitCode,
		RepositoryState: *contextRepositoryStateFromDB(&check.RepositoryState),
		Reproducibility: check.Reproducibility, Summary: check.Summary,
	}
}

func contextHandoffFromDB(handoff *db.Handoff) *contextHandoff {
	return &contextHandoff{
		ID:              contextRecordIDString("handoff", handoff.ID),
		SessionID:       contextRecordID("session", handoff.SessionID),
		AttemptID:       contextRecordIDString("attempt", handoff.AttemptID),
		CreatedAt:       handoff.CreatedAt.UTC().Format(time.RFC3339Nano),
		RepositoryState: *contextRepositoryStateFromDB(&handoff.RepositoryState),
		Completed:       handoff.Completed, Remaining: handoff.Remaining, Risks: handoff.Risks,
		NextAction: handoff.NextAction, CheckIDs: handoff.CheckIDs,
		SupersedesID: handoff.SupersedesID,
	}
}

func contextRepositoryStateFromDB(state *db.RepositoryState) *contextRepositoryState {
	if state == nil {
		return nil
	}
	return &contextRepositoryState{
		Repository: optionalString(state.Repository),
		Branch:     optionalString(state.Branch),
		HeadOID:    optionalString(state.HeadOID),
		Worktree:   state.Worktree,
		CapturedAt: state.CapturedAt.UTC().Format(time.RFC3339Nano),
	}
}

func contextTimePointer(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func contextRecordIDString(kind, id string) string {
	return kind + ":" + id
}

func contextClaimsOverlap(leftType, leftValue, rightType, rightValue string) bool {
	if leftType != rightType {
		return false
	}
	if leftValue == rightValue {
		return true
	}
	if leftType != "path" {
		return false
	}
	return contextPathClaimContains(leftValue, rightValue) ||
		contextPathClaimContains(rightValue, leftValue)
}

func contextPathClaimContains(pattern, value string) bool {
	return strings.HasSuffix(pattern, "/**") &&
		strings.HasPrefix(value, strings.TrimSuffix(pattern, "**"))
}

func readContextRepositoryState(
	ctx stdcontext.Context,
	cwd, capturedAt string,
) (*contextRepositoryState, []contextWarning, error) {
	state := &contextRepositoryState{
		Worktree:   "unknown",
		CapturedAt: capturedAt,
	}
	warnings := make([]contextWarning, 0)

	branch, err := git.CurrentBranch(ctx, cwd)
	if err != nil {
		branchRef, refErr := git.RunCombined(ctx, cwd, "symbolic-ref", "--quiet", "HEAD")
		if refErr != nil {
			return nil, nil, fmt.Errorf("read current branch: %w", err)
		}
		branch = strings.TrimPrefix(branchRef, "refs/heads/")
	}
	if branch != "" && branch != "HEAD" {
		state.Branch = &branch
	}

	head, err := git.RunCombined(ctx, cwd, "rev-parse", "--verify", "HEAD")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		warnings = append(warnings, contextWarning{
			Code:    "head_unavailable",
			Message: "The repository has no readable HEAD commit.",
		})
	} else {
		state.HeadOID = &head
	}

	dirty, err := git.IsDirty(ctx, cwd)
	if err != nil {
		return nil, nil, fmt.Errorf("read worktree state: %w", err)
	}
	if dirty {
		state.Worktree = "changed"
	} else {
		state.Worktree = "clean"
	}
	return state, warnings, nil
}

func contextSessionFromDB(current *db.Session) *contextSession {
	var issue *contextIssue
	if current.IssueID != "" || current.IssueTitle != "" {
		issue = &contextIssue{ID: current.IssueID, Title: current.IssueTitle}
	}
	return &contextSession{
		ID:     contextRecordID("session", current.ID),
		State:  current.State,
		Branch: current.Branch,
		Issue:  issue,
	}
}

func contextRecordID(kind string, id int64) string {
	return kind + ":" + strconv.FormatInt(id, 10)
}

func init() {
	if err := contextCmd.MarkFlagRequired("json"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(contextCmd)
}
