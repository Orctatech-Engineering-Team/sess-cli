package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

var handoffInput struct {
	completed    []string
	remaining    []string
	risks        []string
	nextAction   string
	checkIDs     []string
	supersedesID string
}

var handoffCreateCmd = &cobra.Command{
	Use:   "create <attempt-id>",
	Short: "Create an append-only session handoff",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if handoffInput.nextAction == "" {
			return fmt.Errorf("--next is required")
		}
		database, cwd, err := openHandoffDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		handoff, err := tui.RunHandoffCreate(cwd, database, tui.HandoffCreateInput{
			AttemptID: args[0], Completed: handoffInput.completed,
			Remaining: handoffInput.remaining, Risks: handoffInput.risks,
			NextAction: handoffInput.nextAction, CheckIDs: handoffInput.checkIDs,
			SupersedesID: handoffInput.supersedesID,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Created handoff %s\n", handoff.ID)
		return nil
	},
}

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Manage session handoffs",
}

func openHandoffDatabase() (*db.DB, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("get current directory: %w", err)
	}
	path, err := db.GetDefaultDBPath()
	if err != nil {
		return nil, "", fmt.Errorf("get database path: %w", err)
	}
	database, err := db.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open database: %w", err)
	}
	return database, cwd, nil
}

func init() {
	handoffCreateCmd.Flags().StringSliceVar(&handoffInput.completed, "done", nil, "Completed work item")
	handoffCreateCmd.Flags().StringSliceVar(&handoffInput.remaining, "remaining", nil, "Remaining work item")
	handoffCreateCmd.Flags().StringSliceVar(&handoffInput.risks, "risk", nil, "Known risk")
	handoffCreateCmd.Flags().StringVar(&handoffInput.nextAction, "next", "", "Next useful action")
	handoffCreateCmd.Flags().StringSliceVar(&handoffInput.checkIDs, "check", nil, "Supporting check ID")
	handoffCreateCmd.Flags().StringVar(&handoffInput.supersedesID, "supersedes", "", "Earlier handoff ID being corrected")
	handoffCmd.AddCommand(handoffCreateCmd)
	rootCmd.AddCommand(handoffCmd)
}
