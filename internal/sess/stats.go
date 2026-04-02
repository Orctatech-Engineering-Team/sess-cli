package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session statistics",
	Long:  "Display aggregate session statistics for the tracked project in the current directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
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

		project, err := mgr.GetProject(cwd)
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}
		if project == nil {
			return fmt.Errorf("not a tracked project. Run 'sess start' first")
		}

		stats, err := mgr.GetProjectStats(project.ID)
		if err != nil {
			return fmt.Errorf("failed to get session stats: %w", err)
		}
		if stats.TotalSessions == 0 {
			fmt.Println("No session history")
			fmt.Println()
			fmt.Println("Run 'sess start' to begin tracking.")
			return nil
		}

		fmt.Println("Session stats")
		fmt.Println()
		fmt.Printf("%s · %d sessions · %s total\n", project.Name, stats.TotalSessions, formatDuration(stats.TotalElapsed))
		fmt.Printf("Average %s · longest %s · %d PRs\n", formatDuration(stats.AverageElapsed), formatDuration(stats.LongestElapsed), stats.SessionsWithPR)
		fmt.Printf("Active %d · paused %d · ended %d\n", stats.ActiveSessions, stats.PausedSessions, stats.EndedSessions)
		if stats.FirstSessionAt != nil {
			fmt.Printf("First session %s\n", formatRelativeTime(*stats.FirstSessionAt))
		}
		if stats.LastSessionAt != nil {
			fmt.Printf("Last session %s\n", formatRelativeTime(*stats.LastSessionAt))
		}

		if stats.LongestBranch != "" {
			fmt.Println()
			fmt.Println("Longest session")
			fmt.Printf("  %s · %s", stats.LongestBranch, formatDuration(stats.LongestElapsed))
			if stats.LongestIssueID != "" {
				fmt.Printf(" · #%s", stats.LongestIssueID)
			}
			fmt.Println()
			if stats.LongestIssue != "" {
				fmt.Printf("  %s\n", stats.LongestIssue)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
