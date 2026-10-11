package cmd

import (
	"novelgen/cmd/setupcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return setupcmd.NewCommand()
	})
}
