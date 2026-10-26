package sess

import (
	"fmt"
	"os"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

var checkRecordInput struct {
	name           string
	procedureType  string
	procedureValue string
	outcome        string
	summary        string
	exitCode       int
}

var checkRecordCmd = &cobra.Command{
	Use:   "record <attempt-id>",
	Short: "Record validation evidence for an attempt",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateCheckRecordFlags(); err != nil {
			return err
		}
		database, cwd, err := openCheckDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		var exitCode *int
		if checkRecordInput.exitCode >= 0 {
			value := checkRecordInput.exitCode
			exitCode = &value
		}
		check, err := tui.RunCheckRecord(cwd, database, tui.CheckRecordInput{
			AttemptID: args[0], Name: checkRecordInput.name,
			ProcedureType:  checkRecordInput.procedureType,
			ProcedureValue: checkRecordInput.procedureValue,
			Outcome:        checkRecordInput.outcome, Summary: checkRecordInput.summary,
			ExitCode: exitCode,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Recorded check %s · %s\n", check.ID, check.Outcome)
		return nil
	},
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Manage validation checks",
}

func openCheckDatabase() (*db.DB, string, error) {
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

func validateCheckRecordFlags() error {
	if checkRecordInput.name == "" || checkRecordInput.procedureValue == "" {
		return fmt.Errorf("--name and --procedure are required")
	}
	if checkRecordInput.procedureType != db.CheckCommand && checkRecordInput.procedureType != db.CheckManual {
		return fmt.Errorf("--procedure-type must be command or manual")
	}
	switch checkRecordInput.outcome {
	case db.CheckPassed, db.CheckFailed, db.CheckCancelled, db.CheckUnavailable:
	default:
		return fmt.Errorf("--outcome must be passed, failed, cancelled, or unavailable")
	}
	if checkRecordInput.exitCode < -1 {
		return fmt.Errorf("--exit-code must be -1 or greater")
	}
	return nil
}

func init() {
	checkRecordCmd.Flags().StringVar(&checkRecordInput.name, "name", "", "Check name")
	checkRecordCmd.Flags().StringVar(&checkRecordInput.procedureType, "procedure-type", db.CheckCommand, "Procedure type: command or manual")
	checkRecordCmd.Flags().StringVar(&checkRecordInput.procedureValue, "procedure", "", "Command or manual procedure")
	checkRecordCmd.Flags().StringVar(&checkRecordInput.outcome, "outcome", db.CheckPassed, "Outcome: passed, failed, cancelled, or unavailable")
	checkRecordCmd.Flags().StringVar(&checkRecordInput.summary, "summary", "", "Short redacted result summary")
	checkRecordCmd.Flags().IntVar(&checkRecordInput.exitCode, "exit-code", -1, "Command exit code, or -1 when not applicable")
	checkCmd.AddCommand(checkRecordCmd)
	rootCmd.AddCommand(checkCmd)
}
