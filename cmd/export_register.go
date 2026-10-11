package cmd

import (
	"novelgen/cmd/export"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return export.NewCommand()
	})
}
