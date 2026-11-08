package cmd

import (
	"novelgen/cmd/hookcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return hookcmd.NewCommand()
	})
}
