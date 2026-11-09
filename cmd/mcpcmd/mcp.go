// Package mcpcmd exposes the local MCP stdio server.
package mcpcmd

import (
	"os"
	"strings"

	"novelgen/internal/agentmcp"
	"novelgen/internal/fiction/domain/project"

	"github.com/spf13/cobra"
)

var (
	mcpServeGlobal    bool
	mcpServeWorkspace string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the Novelgen MCP server for agent harnesses",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve MCP tools over stdio",
	Long: `Serve Novelgen project tools over the Model Context Protocol stdio transport.

Without --global the server is bound to one project. With --global it serves a
workspace: novelgen_init can create new projects and every project tool accepts
a projectDir argument. This is the mode for WorkBuddy's "Add MCP" form:

  Command:     C:\path\to\novelgen.exe
  Arguments:   mcp serve --global --workspace D:\path\to\novels

It exposes:
  novelgen_init (global mode)
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
	mcpServeCmd.Flags().BoolVar(&mcpServeGlobal, "global", false,
		"Workspace mode: allow novelgen_init and per-call projectDir")
	mcpServeCmd.Flags().StringVar(&mcpServeWorkspace, "workspace", "",
		"Workspace root for --global (default CODEBUDDY_PROJECT_DIR, CLAUDE_PROJECT_DIR, NOVELGEN_PROJECT_DIR, or cwd)")
	mcpCmd.AddCommand(mcpServeCmd)
}

// NewCommand returns the `novelgen mcp` command tree.
func NewCommand() *cobra.Command {
	return mcpCmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = "novelgen"
	}
	if mcpServeGlobal {
		workspace := strings.TrimSpace(mcpServeWorkspace)
		if workspace == "" {
			workspace = strings.TrimSpace(os.Getenv("CODEBUDDY_PROJECT_DIR"))
		}
		if workspace == "" {
			workspace = strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
		}
		if workspace == "" {
			workspace = strings.TrimSpace(os.Getenv("NOVELGEN_PROJECT_DIR"))
		}
		if workspace == "" {
			workspace, _ = os.Getwd()
		}
		return agentmcp.NewGlobal(workspace, exe).Serve(cmd.Context(), os.Stdin, os.Stdout)
	}
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	return agentmcp.New(book.Root(), exe).Serve(cmd.Context(), os.Stdin, os.Stdout)
}
