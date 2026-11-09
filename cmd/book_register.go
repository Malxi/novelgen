package cmd

import (
	"novelgen/cmd/bookcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return bookcmd.NewCommand()
	})
}
