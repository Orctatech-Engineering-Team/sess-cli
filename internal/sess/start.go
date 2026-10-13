/*
Copyright © 2025 Bernard Katamanso <bernard@orctatech.com>
*/
package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [feature-name]",
	Short: "Start a new session",
	Long: "Start a new work session, optionally linked to a GitHub issue. Creates a feature branch and tracks time.\n" +
		"Use --adopt <branch> to start a new tracked session on an existing exact local branch without an issue.\n" +
		"Adoption does not fetch, reset, or pull and cannot be combined with a feature name.",
	Args: validateStartArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		adoptBranch, err := cmd.Flags().GetString("adopt")
		if err != nil {
			return err
		}

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

		if cmd.Flags().Changed("adopt") {
			return tui.RunAdoptStart(cwd, database, adoptBranch)
		}

		// Pass database to TUI
		featureName := ""
		if len(args) > 0 {
			featureName = args[0]
		}

		return tui.RunStartTUI(featureName, cwd, database)
	},
}

func validateStartArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return err
	}
	if !cmd.Flags().Changed("adopt") {
		return nil
	}
	branch, err := cmd.Flags().GetString("adopt")
	if err != nil {
		return err
	}
	if branch == "" {
		return fmt.Errorf("--adopt requires a non-empty branch name")
	}
	if len(args) != 0 {
		return fmt.Errorf("--adopt cannot be combined with a feature name")
	}
	return nil
}

func init() {
	startCmd.Flags().String("adopt", "", "Start a new session on an existing exact local branch without an issue")
	rootCmd.AddCommand(startCmd)
}
