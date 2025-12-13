package sess

import (
	"fmt"
	"os"
	"path/filepath"

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
			fmt.Println("No tracked projects")
			fmt.Println("\nRun 'sess start' in a directory to begin tracking.")
			return nil
		}

		// Get current directory to highlight it
		cwd, _ := os.Getwd()
		cwdAbs, _ := filepath.Abs(cwd)

		fmt.Printf("Tracked projects (%d)\n\n", len(projects))

		for _, project := range projects {
			// Check if this is the current directory
			isCurrent := project.Path == cwdAbs

			// Get active session if any
			sess, _ := mgr.GetActiveSession(project.ID)

			// Project name with current indicator
			projectName := project.Name
			if isCurrent {
				projectName += " *"
			}
			fmt.Println(projectName)

			// Path (indented)
			fmt.Printf("  %s\n", project.Path)

			// Session status or idle state
			if sess != nil {
				elapsed := mgr.GetCurrentElapsed(sess)

				stateText := "active"
				if sess.State == db.StatePaused {
					stateText = "paused"
				}

				// Compact session info line
				sessionInfo := fmt.Sprintf("  %s · %s · %s", stateText, sess.Branch, formatDuration(elapsed))
				if sess.IssueID != "" {
					sessionInfo += fmt.Sprintf(" · #%s", sess.IssueID)
				}
				fmt.Println(sessionInfo)
			} else {
				fmt.Printf("  idle · base %s\n", project.BaseBranch)
			}

			// Last used timestamp
			fmt.Printf("  %s\n", formatRelativeTime(project.LastUsedAt))

			fmt.Println()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}
