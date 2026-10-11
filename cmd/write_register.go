package cmd

import (
	"novelgen/cmd/writecmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return writecmd.NewCommand()
	})
}
