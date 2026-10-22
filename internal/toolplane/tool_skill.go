package toolplane

import (
	"novelgen/internal/platform/agentruntime"
	"strings"

	"github.com/spf13/cobra"
)

// ToolSkillCmd exposes the bundled skill catalog to autonomous agents. The
// catalog is intentionally separate from project data: skills describe how to
// work, while query/check/patch tools describe the project itself.
var ToolSkillCmd = &cobra.Command{
	Use:   "skill [list|read]",
	Short: "Discover and load agent skills",
	Long: `Discover and load agent skills.

list returns the skill name and description catalog. read returns one complete
SKILL.md body. Agents should load only the skills relevant to their current goal
instead of receiving every workflow prompt up front.`,
}

var ToolSkillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agent skills",
	Args:  cobra.NoArgs,
	RunE:  runToolSkillList,
}

var ToolSkillReadCmd = &cobra.Command{
	Use:   "read <name>",
	Short: "Read one agent skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runToolSkillRead,
}

func runToolSkillList(cmd *cobra.Command, _ []string) error {
	skills, err := agentruntime.ListEmbeddedSkills()
	if err != nil {
		return err
	}
	return WriteToolJSON(cmd, ToolResponse{
		OK:      true,
		Section: "skill",
		Count:   len(skills),
		Results: skills,
	})
}

func runToolSkillRead(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	content, err := agentruntime.ReadEmbeddedSkill(name)
	if err != nil {
		return err
	}
	return WriteToolJSON(cmd, ToolResponse{
		OK:      true,
		Section: "skill",
		Count:   1,
		Results: []map[string]string{{
			"name":    name,
			"content": content,
		}},
	})
}

func init() {
	ToolSkillCmd.AddCommand(ToolSkillListCmd)
	ToolSkillCmd.AddCommand(ToolSkillReadCmd)
}
