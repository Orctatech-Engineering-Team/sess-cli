package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var statsAll bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session statistics",
	Long:  "Display aggregate session statistics for the tracked project in the current directory or across all tracked projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		)
		if statsAll {
			stats, err = mgr.GetGlobalStats()
			if err != nil {
				return fmt.Errorf("failed to get global session stats: %w", err)
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
		}

		if stats.TotalSessions == 0 {
			fmt.Println("No session history")
			fmt.Println()
			fmt.Println("Run 'sess start' to begin tracking.")
			return nil
		}

		fmt.Println("Session stats")
		fmt.Println()
		printStatsSummary(project, stats, statsAll)

		return nil
	},
}

func init() {
	statsCmd.Flags().BoolVar(&statsAll, "all", false, "Show stats across all tracked projects")
	rootCmd.AddCommand(statsCmd)
}
