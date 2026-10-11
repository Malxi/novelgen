package cmd

import (
	"novelgen/cmd/projectcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return projectcmd.NewCommand()
	})
}
