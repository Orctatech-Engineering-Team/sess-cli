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
	ActiveAttempts  []any                   `json:"active_attempts"`
	LatestHandoff   any                     `json:"latest_handoff"`
	Checks          []any                   `json:"checks"`
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
		ActiveAttempts: make([]any, 0),
		Checks:         make([]any, 0),
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
		} else if err := readContextSession(cwd, dbPath, &data); err != nil {
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

func readContextSession(cwd, dbPath string, data *contextData) (err error) {
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
	}
	return nil
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
