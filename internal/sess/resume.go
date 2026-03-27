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
			return fmt.Errorf("not a tracked project. Run 'sess start' first")
		}

		// Get session before resuming
		sess, err := mgr.GetActiveSession(project.ID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}
		if sess == nil {
			return fmt.Errorf("no paused session found. Run 'sess start' to begin")
		}
		if sess.State != db.StatePaused {
			return fmt.Errorf("session is already active")
		}

		// Check if we need to checkout the branch
		ctx := context.Background()
		currentBranch, err := git.CurrentBranch(ctx, cwd)
		if err != nil {
			return fmt.Errorf("determine current branch before resuming: %w", err)
		}
		if currentBranch != sess.Branch {
			fmt.Printf("Switching to branch %s...", sess.Branch)
			if err := git.Checkout(ctx, cwd, sess.Branch); err != nil {
				fmt.Println(" failed")
				return fmt.Errorf("switch to session branch %s before resuming: %w", sess.Branch, err)
			} else {
				fmt.Println(" done")
			}
		}

		// Resume session
		sess, err = mgr.ResumeSession(project.ID)
		if err != nil {
			return err
		}

		// Calculate elapsed time
		elapsed := mgr.GetCurrentElapsed(sess)

		// Display resume info in clean format
		branchDisplay := sess.Branch
		if sess.BranchType != "" {
			branchDisplay = fmt.Sprintf("%s (%s)", sess.Branch, sess.BranchType)
		}

		fmt.Printf("Resumed session on %s\n", branchDisplay)
		if sess.IssueID != "" {
			fmt.Printf("Issue #%s", sess.IssueID)
			if sess.IssueTitle != "" {
				fmt.Printf(" · %s", sess.IssueTitle)
			}
			fmt.Println()
		}
		fmt.Printf("Elapsed time: %s\n", formatDuration(elapsed))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
