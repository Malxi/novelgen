package cmd

import (
	"novelgen/cmd/mcpcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return mcpcmd.NewCommand()
	})
}
