package cmd

import (
	"novelgen/cmd/config"

	"github.com/spf13/cobra"
)

// Registers the extracted config command group. Command groups split into
// cmd/<group>/ subpackages expose a NewCommand() constructor; this file is the
// single place where the root command wires them back in.
func init() {
	RegisterCommand(func() *cobra.Command {
		return config.NewCommand()
	})
}
