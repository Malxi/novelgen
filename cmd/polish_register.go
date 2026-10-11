package cmd

import (
	"novelgen/cmd/polishcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return polishcmd.NewCommand()
	})
}
