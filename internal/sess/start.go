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
	Long:  "Start a new work session, optionally linked to a GitHub issue. Creates a feature branch and tracks time.",
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

		// Pass database to TUI
		featureName := ""
		if len(args) > 0 {
			featureName = args[0]
		}

		return tui.RunStartTUI(featureName, cwd, database)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
