/*
Copyright © 2025 Bernard Katamanso <bernard@orctatech.com>
*/
package sess

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "Sess",
	Short: "Sess – SESS Enables Structured Sessions",
	Long:  `Enable developers to manage focused work sessions tied to GitHub issues directly from the terminal`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Run `sess --help` to see available commands")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}

func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = version

	// Clean up version output
	// For tagged releases: "SESS v0.2.0"
	// For dev builds: "SESS dev-abc1234"
	cleanVersion := version

	// Handle versioninfo pseudo-versions like "v0.0.0-20251208220651-7dba82a8368d+dirty"
	if strings.Contains(version, "-2025") || strings.Contains(version, "-2024") {
		// This is a pseudo-version from versioninfo (no git tag)
		shortCommit := commit
		if len(commit) > 7 {
			shortCommit = commit[:7]
		}
		if shortCommit != "" && shortCommit != "unknown" {
			cleanVersion = fmt.Sprintf("dev-%s", shortCommit)
		} else {
			cleanVersion = "dev"
		}

		// Add dirty flag if present
		if strings.HasSuffix(version, "+dirty") {
			cleanVersion += " (modified)"
		}
	} else if version == "(devel)" || version == "unknown" {
		// Development build without git info
		cleanVersion = "dev"
	}
	// else: keep the version as-is (proper semver tag like "v0.2.0")

	rootCmd.SetVersionTemplate(fmt.Sprintf("SESS %s\n", cleanVersion))
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.GitMate.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
