package sess

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var historyLimit int
var historyAll bool

var historyCmd = &cobra.Command{
	Use:   "history [session-id]",
	Short: "Show recent session history",
	Long: "Display recent sessions for the tracked project in the current directory or across all tracked projects.\n" +
		"Pass a session ID to show its recorded lifecycle and active intervals from any directory.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionID int64
		if len(args) == 1 {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("session ID must be a positive integer")
			}
			if cmd.Flags().Changed("all") || cmd.Flags().Changed("limit") {
				return fmt.Errorf("--all and --limit cannot be used with a session ID")
			}
			sessionID = id
		}
		if historyLimit < 1 {
			return fmt.Errorf("history limit must be at least 1")
		}

		dbPath, err := db.GetDefaultDBPath()
		if err != nil {
			return fmt.Errorf("failed to get database path: %w", err)
		}

		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		mgr := session.NewManager(database)

		if sessionID != 0 {
			timeline, err := mgr.GetSessionTimeline(sessionID)
			if err != nil {
				return fmt.Errorf("failed to get session timeline: %w", err)
			}
			elapsed := time.Duration(timeline.Session.TotalElapsed)
			if timeline.Session.State == db.StateActive {
				elapsed = mgr.GetCurrentElapsed(timeline.Session)
			}
			return renderSessionTimeline(cmd.OutOrStdout(), timeline, elapsed)
		}

		if historyAll {
			entries, err := mgr.GetGlobalSessionHistory(historyLimit)
			if err != nil {
				return fmt.Errorf("failed to get global session history: %w", err)
			}
			if len(entries) == 0 {
				fmt.Println("No session history")
				fmt.Println()
				fmt.Println("Run 'sess start' to begin tracking.")
				return nil
			}

			fmt.Printf("Session history (%d)\n\n", len(entries))
			for i, entry := range entries {
				printHistoryEntry(mgr, entry.Project.Name, entry.Session)
				if i < len(entries)-1 {
					fmt.Println()
				}
			}
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		project, err := mgr.GetProject(cwd)
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}
		if project == nil {
			return fmt.Errorf("not a tracked project. Run 'sess start' first")
		}

		sessions, err := mgr.GetSessionHistory(project.ID, historyLimit)
		if err != nil {
			return fmt.Errorf("failed to get session history: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No session history")
			fmt.Println()
			fmt.Println("Run 'sess start' to begin tracking.")
			return nil
		}

		fmt.Printf("Session history (%d)\n\n", len(sessions))

		for i, sess := range sessions {
			printHistoryEntry(mgr, "", sess)
			if i < len(sessions)-1 {
				fmt.Println()
			}
		}

		return nil
	},
}

func renderSessionTimeline(w io.Writer, timeline *session.SessionTimeline, elapsed time.Duration) error {
	var output strings.Builder
	sess := timeline.Session
	fmt.Fprintf(&output, "Session %d · %s\n", sess.ID, formatBranchDisplay(sess))
	fmt.Fprintf(&output, "%s · tracked active total %s\n", sess.State, formatDuration(elapsed))

	switch {
	case len(timeline.Events) == 0:
		fmt.Fprintln(&output, "No recorded lifecycle history · tracked total preserved")
	case timeline.HistoryComplete:
		fmt.Fprintln(&output, "Lifecycle history complete")
	default:
		fmt.Fprintln(&output, "Lifecycle history incomplete · tracked total preserved")
	}

	if len(timeline.Events) > 0 {
		fmt.Fprintln(&output, "\nRecorded events")
		for _, event := range timeline.Events {
			fmt.Fprintf(
				&output,
				"  %s · %s · elapsed %s\n",
				event.OccurredAt.UTC().Format(time.RFC3339Nano),
				strings.TrimPrefix(event.Kind, "session_"),
				formatDuration(time.Duration(event.TotalElapsed)),
			)
		}
	}

	if len(timeline.Intervals) > 0 {
		fmt.Fprintln(&output, "\nActive intervals")
		for _, interval := range timeline.Intervals {
			end := "ongoing"
			if interval.End != nil {
				end = interval.End.UTC().Format(time.RFC3339Nano)
			}
			fmt.Fprintf(
				&output,
				"  %s → %s · %s\n",
				interval.Start.UTC().Format(time.RFC3339Nano),
				end,
				formatDuration(interval.Duration),
			)
		}
	}

	if _, err := io.WriteString(w, output.String()); err != nil {
		return fmt.Errorf("failed to write session timeline: %w", err)
	}
	return nil
}

func printHistoryEntry(mgr *session.Manager, projectName string, sess *db.Session) {
	if projectName != "" {
		fmt.Printf("%s · %s · session %d\n", projectName, formatBranchDisplay(sess), sess.ID)
	} else {
		fmt.Printf("%s · session %d\n", formatBranchDisplay(sess), sess.ID)
	}

	elapsed := time.Duration(sess.TotalElapsed)
	if sess.State == db.StateActive {
		elapsed = mgr.GetCurrentElapsed(sess)
	}

	fmt.Printf("  %s · %s", sess.State, formatDuration(elapsed))
	if sess.IssueID != "" {
		fmt.Printf(" · #%s", sess.IssueID)
	}
	if sess.PRNumber != nil {
		fmt.Printf(" · PR #%d", *sess.PRNumber)
	}
	fmt.Println()

	fmt.Printf("  Started %s", formatRelativeTime(sess.StartTime))
	switch sess.State {
	case db.StatePaused:
		if sess.PauseTime != nil {
			fmt.Printf(", paused %s", formatRelativeTime(*sess.PauseTime))
		}
	case db.StateEnded:
		if sess.EndTime != nil {
			fmt.Printf(", ended %s", formatRelativeTime(*sess.EndTime))
		}
	}
	fmt.Println()

	if sess.IssueTitle != "" {
		fmt.Printf("  %s\n", sess.IssueTitle)
	}
	if sess.PRURL != "" {
		fmt.Printf("  %s\n", sess.PRURL)
	}
}

func formatBranchDisplay(sess *db.Session) string {
	if sess.BranchType == "" {
		return sess.Branch
	}
	return fmt.Sprintf("%s (%s)", sess.Branch, sess.BranchType)
}

func init() {
	historyCmd.Flags().BoolVar(&historyAll, "all", false, "Show recent sessions across all tracked projects")
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 10, "Number of sessions to show")
	rootCmd.AddCommand(historyCmd)
}
