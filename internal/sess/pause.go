package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current session",
	Long:  "Pause the active session and stop time tracking. You can resume later with 'sess resume'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Open database
		dbPath, err := db.GetDefaultDBPath()
		if err != nil {
			return fmt.Errorf("failed to get database path: %w", err)
		}

		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Create session manager
		mgr := session.NewManager(database)

		// Get project
		project, err := mgr.GetProject(cwd)
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		if project == nil {
			return fmt.Errorf("this directory is not a tracked SESS project. Run 'sess start' first")
		}

		// Pause session
		sess, err := mgr.PauseSession(project.ID)
		if err != nil {
			return err
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		fmt.Println("Session paused")
		fmt.Printf("Branch: %s\n", sess.Branch)
		fmt.Printf("Total elapsed: %s\n", formatDuration(elapsed))
		fmt.Println()
		fmt.Println("Resume anytime with: sess resume")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}
