package cmd

import (
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return toolplane.NewCommand()
	})
}
