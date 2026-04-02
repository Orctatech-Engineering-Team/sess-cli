package sess

import (
	"fmt"
	"os"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var historyLimit int
var historyAll bool

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent session history",
	Long:  "Display recent sessions for the tracked project in the current directory or across all tracked projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
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

func printHistoryEntry(mgr *session.Manager, projectName string, sess *db.Session) {
	if projectName != "" {
		fmt.Printf("%s · %s\n", projectName, formatBranchDisplay(sess))
	} else {
		fmt.Println(formatBranchDisplay(sess))
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
