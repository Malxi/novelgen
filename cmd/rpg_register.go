package cmd

import (
	"novelgen/cmd/rpgcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return rpgcmd.NewCommand()
	})
}
