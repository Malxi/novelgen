package main

import (
	"nolvegen/cmd"
)

func main() {
	// Register all commands (including plugin commands)
	cmd.RegisterAllCommands()
	cmd.Execute()
}
