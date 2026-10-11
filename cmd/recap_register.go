package cmd

import (
	"novelgen/cmd/recap"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return recap.NewCommand()
	})
}
