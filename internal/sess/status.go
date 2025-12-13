package sess

import (
	"fmt"
	"os"
	"time"

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
			fmt.Println("Not a tracked project")
			fmt.Println("\nInitialize with 'sess start' to begin tracking.")
			return nil
		}

		// Get active session
		sess, err := mgr.GetActiveSession(project.ID)
		if err != nil {
			return fmt.Errorf("failed to get active session: %w", err)
		}

		if sess == nil {
			printIdleStatus(project)
			return nil
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		// Display session info
		printActiveStatus(project, sess, elapsed)

		return nil
	},
}

// printIdleStatus displays status when no session is active
func printIdleStatus(project *db.Project) {
	fmt.Printf("On branch %s\n", project.BaseBranch)
	fmt.Println("No active session")
	fmt.Println()
	fmt.Println("Start a new session with 'sess start'")
}

// printActiveStatus displays status when a session is active or paused
func printActiveStatus(project *db.Project, sess *db.Session, elapsed time.Duration) {
	// First line: branch info (like git)
	branchDisplay := sess.Branch
	if sess.BranchType != "" {
		branchDisplay = fmt.Sprintf("%s (%s)", sess.Branch, sess.BranchType)
	}
	fmt.Printf("On branch %s\n", branchDisplay)

	// Second line: session state with key info
	stateDisplay := "Session active"
	if sess.State == db.StatePaused {
		stateDisplay = "Session paused"
	}
	fmt.Printf("%s · %s", stateDisplay, formatDuration(elapsed))

	// Add issue info inline if available
	if sess.IssueID != "" {
		fmt.Printf(" · #%s", sess.IssueID)
	}
	fmt.Println()

	// Third line: timing details
	fmt.Printf("Started %s", formatRelativeTime(sess.StartTime))
	if sess.State == db.StatePaused && sess.PauseTime != nil {
		fmt.Printf(", paused %s", formatRelativeTime(*sess.PauseTime))
	}
	fmt.Println()

	// Issue title on separate line if exists (can be long)
	if sess.IssueTitle != "" {
		fmt.Println()
		fmt.Printf("  %s\n", sess.IssueTitle)
	}

	// Help text
	fmt.Println()
	if sess.State == db.StateActive {
		fmt.Println("  sess pause    Pause the current session")
		fmt.Println("  sess end      End and save the session")
	} else {
		fmt.Println("  sess resume   Resume the paused session")
		fmt.Println("  sess end      End and save the session")
	}
}

// formatDuration formats a time.Duration into a compact human-readable string
func formatDuration(d time.Duration) string {
	seconds := int64(d.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// formatRelativeTime formats a timestamp relative to now
func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes())

	if days > 1 {
		return fmt.Sprintf("%d days ago", days)
	} else if days == 1 {
		return "yesterday"
	} else if hours > 1 {
		return fmt.Sprintf("%d hours ago", hours)
	} else if hours == 1 {
		return "1 hour ago"
	} else if minutes > 1 {
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if minutes == 1 {
		return "1 minute ago"
	}
	return "just now"
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
