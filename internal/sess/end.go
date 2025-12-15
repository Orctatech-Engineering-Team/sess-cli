package sess

import (
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

var endCmd = &cobra.Command{
	Use:   "end",
	Short: "End the current session",
	Long:  "End the current session and stop time tracking.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunEndTUI()
	},
}

func init() {
	rootCmd.AddCommand(endCmd)
}
