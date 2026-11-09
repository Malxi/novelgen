package cmd

import (
	"novelgen/cmd/versioncmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return versioncmd.NewCommand()
	})
}
