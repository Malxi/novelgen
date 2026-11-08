// Package mcpcmd exposes the local MCP stdio server.
package mcpcmd

import (
	"os"

	"novelgen/internal/agentmcp"
	"novelgen/internal/fiction/domain/project"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the Novelgen MCP server for agent harnesses",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve MCP tools over stdio",
	Long: `Serve Novelgen project tools over the Model Context Protocol stdio transport.

The server is local to one project. It exposes:
  novelgen_status
  novelgen_query
  novelgen_check
  novelgen_patch
  novelgen_replace
  novelgen_refresh

It does not open a network port.`,
	Args: cobra.NoArgs,
	RunE: runServe,
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}

// NewCommand returns the `novelgen mcp` command tree.
func NewCommand() *cobra.Command {
	return mcpCmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "novelgen"
	}
	server := agentmcp.New(book.Root(), exe)
	return server.Serve(cmd.Context(), os.Stdin, os.Stdout)
}
