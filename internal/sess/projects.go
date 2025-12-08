package sess

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all tracked projects",
	Long:  "Display all projects tracked by SESS with their status and last activity.",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// List all projects
		projects, err := mgr.ListProjects()
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if len(projects) == 0 {
			fmt.Println("No tracked projects found.")
			fmt.Println("\nRun 'sess start' in a directory to begin tracking a project.")
			return nil
		}

		fmt.Printf("Tracked Projects (%d)\n\n", len(projects))

		// Get current directory to highlight it
		cwd, _ := os.Getwd()
		cwdAbs, _ := filepath.Abs(cwd)

		for i, project := range projects {
			// Check if this is the current directory
			isCurrent := project.Path == cwdAbs

			// Get active session if any
			sess, _ := mgr.GetActiveSession(project.ID)

			// Format project name
			projectLine := fmt.Sprintf("%d. %s", i+1, project.Name)
			if isCurrent {
				projectLine += " (current)"
			}

			fmt.Println(projectLine)
			fmt.Printf("%s\n", project.Path)
			fmt.Printf("Base: %s\n", project.BaseBranch)

			if sess != nil {
				// Show session info
				stateEmoji := "🟢"
				if sess.State == db.StatePaused {
					stateEmoji = "🟡"
				}
				fmt.Printf("%s Session: %s on %s\n", stateEmoji, sess.State, sess.Branch)

				elapsed := mgr.GetCurrentElapsed(sess)
				fmt.Printf("Elapsed: %s\n", formatDuration(elapsed))

				if sess.IssueID != "" {
					fmt.Printf("Issue: #%s\n", sess.IssueID)
				}
			} else {
				fmt.Println("No active session")
			}

			fmt.Printf("Last used: %s\n", formatRelativeTime(project.LastUsedAt))
			fmt.Println()
		}

		fmt.Println("Tip: Use 'cd <path>' to navigate to a project, then 'sess status' to see details")

		return nil
	},
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("2006-01-02")
	}
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}
