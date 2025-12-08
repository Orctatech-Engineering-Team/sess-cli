/*
Copyright © 2025 Bernard Katamanso <bernard@orctatech.com>
*/
package sess

import (
	"github.com/Orctatech-Engineering-Team/Sess/internal/tui"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new feature branch workflow",
	Long:  ` This command will start a new workflow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return tui.RunStartTUI("")
		}
		return tui.RunStartTUI(args[0])
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
