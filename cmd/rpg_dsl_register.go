package cmd

import (
	"novelgen/cmd/rpgdsl"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return rpgdsl.NewCommand()
	})
}
