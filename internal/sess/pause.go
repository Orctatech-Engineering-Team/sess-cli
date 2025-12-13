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
			return fmt.Errorf("not a tracked project. Run 'sess start' first")
		}

		// Pause session
		sess, err := mgr.PauseSession(project.ID)
		if err != nil {
			return err
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		// Display pause info in clean format
		branchDisplay := sess.Branch
		if sess.BranchType != "" {
			branchDisplay = fmt.Sprintf("%s (%s)", sess.Branch, sess.BranchType)
		}

		fmt.Printf("Paused session on %s\n", branchDisplay)
		if sess.IssueID != "" {
			fmt.Printf("Issue #%s", sess.IssueID)
			if sess.IssueTitle != "" {
				fmt.Printf(" · %s", sess.IssueTitle)
			}
			fmt.Println()
		}
		fmt.Printf("Total elapsed: %s\n", formatDuration(elapsed))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}
