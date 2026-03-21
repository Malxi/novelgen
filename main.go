package main

import (
	"novelgen/cmd"
)

func main() {
	// Register all commands (including plugin commands)
	cmd.RegisterAllCommands()
	cmd.Execute()
}
