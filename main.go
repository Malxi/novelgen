package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"novelgen/cmd"
)

func main() {
	// Ctrl+C / SIGTERM cancels the command context so long pipelines can stop
	// at the next safe checkpoint instead of being killed mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Register all commands (including plugin commands)
	cmd.RegisterAllCommands()
	cmd.ExecuteContext(ctx)
}
