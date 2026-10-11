package cmd

import (
	"novelgen/cmd/craftcmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return craftcmd.NewCommand()
	})
}
