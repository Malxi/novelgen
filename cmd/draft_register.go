package cmd

import (
	"novelgen/cmd/draftcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return draftcmd.NewCommand()
	})
}
