package cmd

import (
	"novelgen/cmd/translate"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return translate.NewCommand()
	})
}
