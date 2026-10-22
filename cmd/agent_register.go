package cmd

import (
	"novelgen/cmd/agentcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return agentcmd.NewCommand()
	})
}
