package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var reportAll bool
var reportLimit int

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show a session report",
	Long:  "Display a compact session report for the tracked project in the current directory or across all tracked projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if reportLimit < 1 {
			return fmt.Errorf("report limit must be at least 1")
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

		var (
			project *db.Project
			stats   *session.ProjectStats
			entries []*session.SessionHistoryEntry
		)

		if reportAll {
			stats, err = mgr.GetGlobalStats()
			if err != nil {
				return fmt.Errorf("failed to get global session stats: %w", err)
			}
			entries, err = mgr.GetGlobalSessionHistory(reportLimit)
			if err != nil {
				return fmt.Errorf("failed to get global session history: %w", err)
			}
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			project, err = mgr.GetProject(cwd)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}
			if project == nil {
				return fmt.Errorf("not a tracked project. Run 'sess start' first")
			}

			stats, err = mgr.GetProjectStats(project.ID)
			if err != nil {
				return fmt.Errorf("failed to get session stats: %w", err)
			}

			sessions, err := mgr.GetSessionHistory(project.ID, reportLimit)
			if err != nil {
				return fmt.Errorf("failed to get session history: %w", err)
			}
			for _, sess := range sessions {
				entries = append(entries, &session.SessionHistoryEntry{
					Project: project,
					Session: sess,
				})
			}
		}

		if stats.TotalSessions == 0 {
			fmt.Println("No session history")
			fmt.Println()
			fmt.Println("Run 'sess start' to begin tracking.")
			return nil
		}

		fmt.Println("Session report")
		fmt.Println()
		printStatsSummary(project, stats, reportAll)

		if len(entries) == 0 {
			return nil
		}

		fmt.Println()
		fmt.Printf("Recent sessions (%d)\n\n", len(entries))
		for i, entry := range entries {
			projectName := ""
			if reportAll {
				projectName = entry.Project.Name
			}
			printHistoryEntry(mgr, projectName, entry.Session)
			if i < len(entries)-1 {
				fmt.Println()
			}
		}

		return nil
	},
}

func printStatsSummary(project *db.Project, stats *session.ProjectStats, isGlobal bool) {
	if isGlobal {
		fmt.Printf("All projects · %d projects · %d sessions · %s total\n", stats.TotalProjects, stats.TotalSessions, formatDuration(stats.TotalElapsed))
	} else {
		fmt.Printf("%s · %d sessions · %s total\n", project.Name, stats.TotalSessions, formatDuration(stats.TotalElapsed))
	}
	fmt.Printf("Average %s · longest %s · %d PRs\n", formatDuration(stats.AverageElapsed), formatDuration(stats.LongestElapsed), stats.SessionsWithPR)
	fmt.Printf("Active %d · paused %d · ended %d\n", stats.ActiveSessions, stats.PausedSessions, stats.EndedSessions)
	if stats.FirstSessionAt != nil {
		fmt.Printf("First session %s\n", formatRelativeTime(*stats.FirstSessionAt))
	}
	if stats.LastSessionAt != nil {
		fmt.Printf("Last session %s\n", formatRelativeTime(*stats.LastSessionAt))
	}

	if stats.LongestBranch == "" {
		return
	}

	fmt.Println()
	fmt.Println("Longest session")
	if isGlobal && stats.LongestProject != "" {
		fmt.Printf("  %s · %s · %s", stats.LongestProject, stats.LongestBranch, formatDuration(stats.LongestElapsed))
	} else {
		fmt.Printf("  %s · %s", stats.LongestBranch, formatDuration(stats.LongestElapsed))
	}
	if stats.LongestIssueID != "" {
		fmt.Printf(" · #%s", stats.LongestIssueID)
	}
	fmt.Println()
	if stats.LongestIssue != "" {
		fmt.Printf("  %s\n", stats.LongestIssue)
	}
}

func init() {
	reportCmd.Flags().BoolVar(&reportAll, "all", false, "Show a report across all tracked projects")
	reportCmd.Flags().IntVarP(&reportLimit, "limit", "n", 5, "Number of recent sessions to include")
	rootCmd.AddCommand(reportCmd)
}
