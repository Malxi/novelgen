package cmd

import (
	"novelgen/cmd/initcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return initcmd.NewCommand()
	})
}
