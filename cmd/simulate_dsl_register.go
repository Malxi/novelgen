package cmd

import (
	"novelgen/cmd/simulatedsl"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return simulatedsl.NewCommand()
	})
}
