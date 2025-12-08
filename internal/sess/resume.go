package sess

import (
	"context"
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/git"
	"github.com/Orctatech-Engineering-Team/Sess/internal/session"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused session",
	Long:  "Resume the paused session and continue time tracking.",
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

		// Get session before resuming
		sess, err := mgr.GetActiveSession(project.ID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		if sess == nil {
			return fmt.Errorf("no paused session found. Run 'sess start' to begin a new session")
		}

		// Check if we need to checkout the branch
		ctx := context.Background()
		currentBranch, err := git.CurrentBranch(ctx, cwd)
		if err == nil && currentBranch != sess.Branch {
			fmt.Printf("Checking out branch: %s\n", sess.Branch)
			if err := git.Checkout(ctx, cwd, sess.Branch); err != nil {
				fmt.Printf("⚠️Warning: Failed to checkout branch: %v\n", err)
				fmt.Println("You may need to checkout the branch manually.")
			} else {
				fmt.Println("✅ Branch checked out")
			}
		}

		// Resume session
		sess, err = mgr.ResumeSession(project.ID)
		if err != nil {
			return err
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		fmt.Println()
		fmt.Println("Session resumed")
		fmt.Printf("Branch: %s\n", sess.Branch)
		if sess.IssueID != "" {
			fmt.Printf("Issue: #%s - %s\n", sess.IssueID, sess.IssueTitle)
		}
		fmt.Printf("Total elapsed: %s\n", formatDuration(elapsed))
		fmt.Println()
		fmt.Println("Happy coding! Use 'sess pause' to pause again.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
