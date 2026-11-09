// Package ideascmd exposes deterministic content reference draws.
package ideascmd

import (
	"github.com/spf13/cobra"

	"novelgen/internal/toolplane"
)

var ideasCmd = &cobra.Command{
	Use:   "ideas",
	Short: "Draw deterministic content references for generation",
	Long: `Draw deterministic content references for generation.

  draw   draw namecards, scenarios, axioms, world packs, storylines, ...
  types  list the available idea types

These are reference libraries, not project state; they do not require a project.`,
}

var ideasDrawCmd = &cobra.Command{
	Use:   "draw",
	Short: "Draw content ideas with a reproducible seed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return toolplane.ToolQueryCmd.RunE(cmd, []string{"ideas"})
	},
}

var ideasTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available idea types",
	Args:  cobra.NoArgs,
	RunE:  runIdeaTypes,
}

func init() {
	ideasDrawCmd.Flags().AddFlagSet(toolplane.ToolQueryCmd.Flags())
	ideasCmd.AddCommand(ideasDrawCmd)
	ideasCmd.AddCommand(ideasTypesCmd)
}

// NewCommand returns the `novelgen ideas` command tree.
func NewCommand() *cobra.Command {
	return ideasCmd
}

func runIdeaTypes(cmd *cobra.Command, _ []string) error {
	types := []string{
		"provocation", "axiom", "event", "law", "scenario", "engine",
		"worldpack", "premise", "storyline", "cast", "namecard",
		"resource", "itemcard", "reference", "name",
	}
	results := make([]map[string]string, 0, len(types))
	for _, name := range types {
		results = append(results, map[string]string{"name": name})
	}
	return toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{
		OK:      true,
		Section: "ideas",
		Count:   len(results),
		Results: results,
	})
}
