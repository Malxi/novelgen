package agentcmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/agentkit"
	"novelgen/internal/fiction/domain/project"

	"github.com/spf13/cobra"
)

var (
	agentInstallTargets    string
	agentInstallApply      bool
	agentInstallForce      bool
	agentInstallAllowHooks bool
	agentInstallJSON       bool
	agentInstallCodexHome  string
)

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Novelgen adapters for Claude Code, Codex, or DSH",
	Long: `Install project-local agent adapters.

The shared stage workflows are written to .novelgen/workflows/. Target-specific
files are thin adapters:

  claude  .claude-plugin plugin, commands, skill, hooks and MCP config
  codex   AGENTS.md block, .codex MCP config, skill
  dsh     shared workflows and a pointer to the Cordis bundle

The command is dry-run by default. Pass --apply to write. Existing files are
preserved unless --force is supplied; the managed AGENTS.md block is updated in
place. Claude hooks are skipped unless --allow-hooks is explicit.`,
	Args: cobra.NoArgs,
	RunE: runAgentInstall,
}

func init() {
	agentInstallCmd.Flags().StringVar(&agentInstallTargets, "target", "claude,codex",
		"Comma-separated targets: claude, codex, dsh; all = claude+codex")
	agentInstallCmd.Flags().BoolVar(&agentInstallApply, "apply", false, "Write adapter files")
	agentInstallCmd.Flags().BoolVar(&agentInstallForce, "force", false, "Overwrite existing adapter files")
	agentInstallCmd.Flags().BoolVar(&agentInstallAllowHooks, "allow-hooks", false,
		"Install executable Claude hooks (PreToolUse write guard, SessionStart status)")
	agentInstallCmd.Flags().BoolVar(&agentInstallJSON, "json", false, "Print the install report as JSON")
	agentInstallCmd.Flags().StringVar(&agentInstallCodexHome, "codex-home", "",
		"Write the Codex MCP entry into this CODEX_HOME/config.toml (Codex CLI does not read a project .codex/config.toml)")
	agentCmd.AddCommand(agentInstallCmd)
}

func runAgentInstall(cmd *cobra.Command, _ []string) error {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	targets := strings.Split(agentInstallTargets, ",")
	result, err := agentkit.Install(book.Root().Dir(), targets, agentkit.Options{
		Apply:      agentInstallApply,
		Force:      agentInstallForce,
		AllowHooks: agentInstallAllowHooks,
		CodexHome:  agentInstallCodexHome,
	})
	if err != nil {
		return err
	}
	if agentInstallJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}
	mode := "dry-run"
	if agentInstallApply {
		mode = "applied"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Novelgen adapter install (%s) at %s\n", mode, result.Root)
	for _, file := range result.Files {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-9s %s\n", file.Action, file.Path)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
	}
	if !agentInstallApply {
		fmt.Fprintln(cmd.OutOrStdout(), "dry-run: pass --apply to write these files")
	}
	return nil
}
