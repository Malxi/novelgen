// Package gencmd exposes read-only generation guidance: workflows, skills and references.
package gencmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/agentkit"
	"novelgen/internal/toolplane"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Read generation guidance for an agent",
	Long: `Read generation guidance for an agent.

  workflow   shared stage workflows (.novelgen/workflows)
  skill      bundled Agent SDK skill catalog
  reference  schemas, contracts and rule references

These commands are read-only and do not require a project.`,
}

var genWorkflowCmd = &cobra.Command{Use: "workflow", Short: "Read stage workflows"}
var genSkillCmd = &cobra.Command{Use: "skill", Short: "Read agent skills"}
var genReferenceCmd = &cobra.Command{Use: "reference", Short: "Read reference documents"}

func init() {
	genWorkflowCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Args:  cobra.NoArgs,
		RunE:  runWorkflowList,
	})
	genWorkflowCmd.AddCommand(&cobra.Command{
		Use:   "read <name>",
		Short: "Read one workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowRead,
	})

	genSkillCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List skills",
		Args:  cobra.NoArgs,
		RunE:  toolplane.ToolSkillListCmd.RunE,
	})
	genSkillCmd.AddCommand(&cobra.Command{
		Use:   "read <name>",
		Short: "Read one skill",
		Args:  cobra.ExactArgs(1),
		RunE:  toolplane.ToolSkillReadCmd.RunE,
	})

	genReferenceCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List references",
		Args:  cobra.NoArgs,
		RunE:  runReferenceList,
	})
	genReferenceCmd.AddCommand(&cobra.Command{
		Use:   "read <name>",
		Short: "Read one reference",
		Args:  cobra.ExactArgs(1),
		RunE:  runReferenceRead,
	})

	genCmd.AddCommand(genWorkflowCmd, genSkillCmd, genReferenceCmd)
}

// NewCommand returns the `novelgen gen` command tree.
func NewCommand() *cobra.Command {
	return genCmd
}

func runWorkflowList(cmd *cobra.Command, _ []string) error {
	items, err := agentkit.ListWorkflows()
	if err != nil {
		return err
	}
	return toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{
		OK:      true,
		Section: "workflow",
		Count:   len(items),
		Results: items,
	})
}

func runWorkflowRead(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	content, err := agentkit.ReadWorkflow(name)
	if err != nil {
		return fmt.Errorf("read workflow %q: %w", name, err)
	}
	return toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{
		OK:      true,
		Section: "workflow",
		Count:   1,
		Results: []map[string]string{{"name": name, "content": content}},
	})
}

func runReferenceList(cmd *cobra.Command, _ []string) error {
	items, err := agentkit.ListReferences()
	if err != nil {
		return err
	}
	return toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{
		OK:      true,
		Section: "reference",
		Count:   len(items),
		Results: items,
	})
}

func runReferenceRead(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	content, err := agentkit.ReadReference(name)
	if err != nil {
		return fmt.Errorf("read reference %q: %w", name, err)
	}
	return toolplane.WriteToolJSON(cmd, toolplane.ToolResponse{
		OK:      true,
		Section: "reference",
		Count:   1,
		Results: []map[string]string{{"name": name, "content": content}},
	})
}
