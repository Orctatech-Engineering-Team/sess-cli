/*
Copyright © 2025 Bernard Katamanso <bernard@orctatech.com>
*/
package sess

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var pseudoVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?(?:-|\.)\d{14}-[0-9a-f]{12}(?:\+dirty)?$`)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sess",
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
	if isPseudoVersion(version) {
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

func isPseudoVersion(version string) bool {
	return pseudoVersionPattern.MatchString(version)
}
