// Package versioncmd prints the Novelgen CLI version.
package versioncmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is overridden by the release build with -X.
var Version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Novelgen CLI version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "novelgen %s\n", Version)
	},
}

// NewCommand returns the `novelgen version` command.
func NewCommand() *cobra.Command {
	return versionCmd
}
