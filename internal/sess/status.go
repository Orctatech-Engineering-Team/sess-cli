package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session status",
	Long:  "Display the current session state, branch, issue, and elapsed time.",
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
			fmt.Println("❌ This directory is not a tracked SESS project.")
			fmt.Println("\nRun 'sess start' to begin a session and track this project.")
			return nil
		}

		// Get active session
		sess, err := mgr.GetActiveSession(project.ID)
		if err != nil {
			return fmt.Errorf("failed to get active session: %w", err)
		}

		if sess == nil {
			fmt.Printf("Project: %s\n", project.Name)
			fmt.Printf("Path: %s\n", project.Path)
			fmt.Printf("Base Branch: %s\n", project.BaseBranch)
			fmt.Println("\nState: IDLE")
			fmt.Println("\nNo active session. Run 'sess start' to begin.")
			return nil
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		// Display session info
		fmt.Printf("Project: %s\n", project.Name)
		fmt.Printf("Path: %s\n", project.Path)
		fmt.Printf("Base Branch: %s\n", project.BaseBranch)
		fmt.Println()

		// State indicator
		stateEmoji := "🟢"
		stateText := "ACTIVE"
		if sess.State == db.StatePaused {
			stateEmoji = "🟡"
			stateText = "PAUSED"
		}

		fmt.Printf("%s State: %s\n", stateEmoji, stateText)
		fmt.Printf("🌱 Branch: %s\n", sess.Branch)

		if sess.IssueID != "" {
			fmt.Printf("🎫 Issue: #%s - %s\n", sess.IssueID, sess.IssueTitle)
		}

		fmt.Printf("Elapsed: %s\n", formatDuration(elapsed))
		fmt.Printf("Started: %s\n", sess.StartTime.Format("2006-01-02 15:04:05"))

		if sess.State == db.StatePaused && sess.PauseTime != nil {
			fmt.Printf("Paused: %s\n", sess.PauseTime.Format("2006-01-02 15:04:05"))
		}

		fmt.Println()

		if sess.State == db.StateActive {
			fmt.Println("Next: 'sess pause' to pause, or continue working!")
		} else {
			fmt.Println("Next: 'sess resume' to continue working")
		}

		return nil
	},
}

func formatDuration(d interface{}) string {
	var seconds int64

	switch v := d.(type) {
	case int64:
		seconds = v
	default:
		// If it's a time.Duration or something else, try to get seconds
		return fmt.Sprintf("%v", d)
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
