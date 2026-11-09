package cmd

import (
	"novelgen/cmd/ideascmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return ideascmd.NewCommand()
	})
}
