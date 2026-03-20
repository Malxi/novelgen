package cmd

import (
	"github.com/spf13/cobra"
)

// CommandRegistrar is a function that registers commands
// This allows commands to self-register via init() functions
type CommandRegistrar func() *cobra.Command

// commandRegistry holds all registered commands
var commandRegistry []CommandRegistrar

// RegisterCommand registers a command factory function
// Call this in your command file's init() function
func RegisterCommand(registrar CommandRegistrar) {
	commandRegistry = append(commandRegistry, registrar)
}

// RegisterAllCommands registers all commands that have been added to the registry
// Call this before Execute() in main.go
func RegisterAllCommands() {
	for _, registrar := range commandRegistry {
		if cmd := registrar(); cmd != nil {
			rootCmd.AddCommand(cmd)
		}
	}
}
