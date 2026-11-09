package cmd

import (
	"novelgen/cmd/gencmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return gencmd.NewCommand()
	})
}
