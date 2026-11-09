// Package checkcmd exposes the agent-facing findings command.
package checkcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/toolplane"
)

var (
	checkProfile     string
	checkMinSeverity string
	checkMaxFindings int
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run deterministic checks and return findings",
	Long: `Run deterministic checks and return findings.

Profiles:
  quality   structural/contract rules
  schema    typed craft or recap schema validation
  rpg       in-memory RPG simulation
  all       quality + rpg
  custom    project rules under .novelgen/rules/`,
	Args: cobra.NoArgs,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().AddFlagSet(toolplane.ToolCheckCmd.Flags())
	checkCmd.Flags().StringVar(&checkProfile, "profile", "all", "Check profile: quality, schema, rpg, all, custom")
	checkCmd.Flags().StringVar(&checkMinSeverity, "min-severity", "", "Only return findings at or above severity: low, medium, high, critical")
	checkCmd.Flags().IntVar(&checkMaxFindings, "max-findings", 0, "Maximum number of findings to return")
}

// NewCommand returns the `novelgen check` command.
func NewCommand() *cobra.Command {
	return checkCmd
}

func runCheck(cmd *cobra.Command, _ []string) error {
	kind := ""
	switch strings.ToLower(strings.TrimSpace(checkProfile)) {
	case "", "all":
		kind = "all"
	case "quality":
		kind = "quality"
	case "schema":
		kind = "schema"
	case "rpg", "simulation":
		kind = "simulation"
	case "custom":
		kind = "custom"
	default:
		return fmt.Errorf("unsupported --profile %q; valid profiles: quality, schema, rpg, all, custom", checkProfile)
	}
	if value := strings.TrimSpace(checkMinSeverity); value != "" {
		toolplane.ToolCheckFlags.MinPriority = value
	}
	if checkMaxFindings > 0 {
		toolplane.ToolCheckFlags.MaxIssues = checkMaxFindings
	}
	return toolplane.ToolCheckCmd.RunE(cmd, []string{kind})
}
