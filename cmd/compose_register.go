package cmd

import (
	"novelgen/cmd/composecmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return composecmd.NewCommand()
	})
}
