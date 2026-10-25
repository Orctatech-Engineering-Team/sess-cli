package sess

import (
	"fmt"
	"os"
	"strings"

	"github.com/Orctatech-Engineering-Team/Sess/internal/db"
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

var attemptStartInput struct {
	participantID    string
	participantKind  string
	participantLabel string
	identitySource   string
	role             string
	claims           []string
}

var attemptStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an agent work attempt",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		database, cwd, err := openAttemptDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		claims, err := parseAttemptClaims(attemptStartInput.claims)
		if err != nil {
			return err
		}
		result, err := tui.RunAttemptStart(cwd, database, tui.AttemptStartInput{
			ParticipantID:    attemptStartInput.participantID,
			ParticipantKind:  attemptStartInput.participantKind,
			ParticipantLabel: attemptStartInput.participantLabel,
			IdentitySource:   attemptStartInput.identitySource,
			Role:             attemptStartInput.role,
			Claims:           claims,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Started attempt %s · participant %s\n", result.Attempt.ID, result.Attempt.Participant.ID)
		for _, warning := range result.Warnings {
			fmt.Printf("Warning: %s\n", warning)
		}
		return nil
	},
}

var attemptBlockCmd = newAttemptTransitionCommand("block", "Block an agent work attempt", db.AttemptBlocked)
var attemptResumeCmd = newAttemptTransitionCommand("resume", "Resume a blocked agent work attempt", db.AttemptActive)

var attemptFinishState string
var attemptFinishCmd = &cobra.Command{
	Use:   "finish <attempt-id>",
	Short: "Finish an agent work attempt",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if attemptFinishState != db.AttemptCompleted &&
			attemptFinishState != db.AttemptCancelled &&
			attemptFinishState != db.AttemptFailed {
			return fmt.Errorf("--state must be completed, cancelled, or failed")
		}
		database, cwd, err := openAttemptDatabase()
		if err != nil {
			return err
		}
		defer database.Close()
		attempt, err := tui.RunAttemptTransition(cwd, database, args[0], attemptFinishState, "")
		if err != nil {
			return err
		}
		fmt.Printf("Finished attempt %s · %s\n", attempt.ID, attempt.State)
		return nil
	},
}

func newAttemptTransitionCommand(use, short, state string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <attempt-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, cwd, err := openAttemptDatabase()
			if err != nil {
				return err
			}
			defer database.Close()
			reason := ""
			if state == db.AttemptBlocked {
				reason, err = cmd.Flags().GetString("reason")
				if err != nil {
					return err
				}
				if strings.TrimSpace(reason) == "" {
					return fmt.Errorf("--reason is required when blocking an attempt")
				}
			}
			attempt, err := tui.RunAttemptTransition(cwd, database, args[0], state, reason)
			if err != nil {
				return err
			}
			fmt.Printf("Updated attempt %s · %s\n", attempt.ID, attempt.State)
			return nil
		},
	}
}

func openAttemptDatabase() (*db.DB, string, error) {
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

func parseAttemptClaims(values []string) ([]db.AttemptClaim, error) {
	claims := make([]db.AttemptClaim, 0, len(values))
	for _, value := range values {
		kind, claim, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(kind) == "" || strings.TrimSpace(claim) == "" {
			return nil, fmt.Errorf("claims must use type=value")
		}
		claims = append(claims, db.AttemptClaim{Type: kind, Value: claim})
	}
	return claims, nil
}

func init() {
	attemptStartCmd.Flags().StringVar(&attemptStartInput.participantID, "participant-id", "", "Stable participant ID")
	attemptStartCmd.Flags().StringVar(&attemptStartInput.participantKind, "kind", db.ParticipantHuman, "Participant kind: human, agent, or automation")
	attemptStartCmd.Flags().StringVar(&attemptStartInput.participantLabel, "label", "", "Participant display label")
	attemptStartCmd.Flags().StringVar(&attemptStartInput.identitySource, "identity-source", "", "Identity source: configured, orchestrator, or ephemeral")
	attemptStartCmd.Flags().StringVar(&attemptStartInput.role, "role", "implementation", "Attempt role")
	attemptStartCmd.Flags().StringSliceVar(&attemptStartInput.claims, "claim", nil, "Advisory claim in type=value form")
	attemptBlockCmd.Flags().String("reason", "", "Reason the attempt is blocked")
	attemptFinishCmd.Flags().StringVar(&attemptFinishState, "state", db.AttemptCompleted, "Terminal state: completed, cancelled, or failed")
	attemptCmd.AddCommand(attemptStartCmd, attemptBlockCmd, attemptResumeCmd, attemptFinishCmd)
	rootCmd.AddCommand(attemptCmd)
}

var attemptCmd = &cobra.Command{
	Use:   "attempt",
	Short: "Manage agent work attempts",
}
