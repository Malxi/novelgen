package cmd

import (
	"novelgen/cmd/checkcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return checkcmd.NewCommand()
	})
}
