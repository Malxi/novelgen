package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "novel",
	Short: "A CLI tool for novel generation",
	Long: `Novel Generation Tool is a command-line interface designed to facilitate
step-by-step novel creation through a structured "decompress" workflow.

It guides users from initial idea to complete chapter drafts by progressively
expanding and injecting context at each stage, while ensuring consistency across
all story elements.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
