package cmd

import (
	"novelgen/cmd/storagecmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return storagecmd.NewCommand()
	})
}
