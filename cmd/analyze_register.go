package cmd

import (
	"novelgen/cmd/analyzecmd"

	"github.com/spf13/cobra"
)

func init() {
	RegisterCommand(func() *cobra.Command {
		return analyzecmd.NewAIRPGCommand()
	})
	RegisterCommand(func() *cobra.Command {
		return analyzecmd.NewChapterAICommand()
	})
}
