package agentcmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/app"
	"novelgen/internal/platform/logger"

	"github.com/spf13/cobra"
)

var (
	agentRunWrite    bool
	agentRunSkills   []string
	agentRunMaxTurns int
	agentRunTimeout  int
	agentRunAttempts int
	agentRunModel    string
	agentRunRuntime  string
	agentRunResume   string
	agentRunJSON     bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run a goal-level autonomous agent",
	Long: `Run a goal-level autonomous agent over the current project.

Unlike a fixed stage command, the agent receives an objective, a skill catalog,
and project query/check/patch tools. It decides which skills to load and which
tools to call, then returns a validated completion report.`,
}

var agentRunCmd = &cobra.Command{
	Use:   "run [goal]",
	Short: "Run the autonomous agent toward a goal",
	Long: `Run the autonomous agent toward a goal.

The agent can discover skills with ` + "`novelgen tool skill list`" + ` and load one
with ` + "`novelgen tool skill read <name>`" + `. It can query and check project
state. Project writes are disabled unless --write is supplied; when enabled,
the agent must use the validated patch tools and run follow-up checks. With a
supporting SDK the project tools are typed MCP tools; older SDKs fall back to
the CLI allowlist. Every run returns a session id, usable with --resume.

Examples:
  novelgen agent run "检查第三卷大纲的节奏问题，只读"
  novelgen agent run --write "把 P1-V1-C3 的节奏改紧并确保检查通过"
  novelgen agent run --skill write-improve-workflow --write "修复 P1-V1-C4 的连续性问题"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAgentGoal,
}

func init() {
	agentRunCmd.Flags().BoolVar(&agentRunWrite, "write", false, "Allow validated project patch/refresh tools; default is read-only")
	agentRunCmd.Flags().StringArrayVar(&agentRunSkills, "skill", nil, "Restrict the visible skill catalog by name (see \"novelgen tool skill list\"); repeatable")
	agentRunCmd.Flags().IntVar(&agentRunMaxTurns, "max-turns", 40, "Maximum Agent SDK turns in the goal session")
	agentRunCmd.Flags().IntVar(&agentRunTimeout, "timeout", 1800, "Agent session timeout in seconds")
	agentRunCmd.Flags().IntVar(&agentRunAttempts, "max-attempts", 2, "Goal attempts; validation failures retry read-only")
	agentRunCmd.Flags().StringVar(&agentRunModel, "model", "", "Override the model for this goal session")
	agentRunCmd.Flags().StringVar(&agentRunRuntime, "runtime", "", "Agent runtime name from agent_config.json; empty uses the default")
	agentRunCmd.Flags().StringVar(&agentRunResume, "resume", "", "Resume a previous goal session by its returned session_id")
	agentRunCmd.Flags().BoolVar(&agentRunJSON, "json", false, "Print the validated goal report as JSON")
	agentCmd.AddCommand(agentRunCmd)
}

// NewCommand returns the `novelgen agent` command tree.
func NewCommand() *cobra.Command {
	return agentCmd
}

func runAgentGoal(cmd *cobra.Command, args []string) error {
	logger.SetDefault(logger.New(logger.InfoLevel))
	session, err := app.Open(cmd)
	if err != nil {
		return err
	}
	goal := strings.TrimSpace(strings.Join(args, " "))
	report, err := app.RunAgentGoal(cmd.Context(), app.AgentGoalOptions{
		Goal:          goal,
		Root:          session.Root(),
		AllowedSkills: agentRunSkills,
		AllowWrite:    agentRunWrite,
		MaxTurns:      agentRunMaxTurns,
		Timeout:       agentRunTimeout,
		MaxAttempts:   agentRunAttempts,
		Model:         agentRunModel,
		RuntimeName:   agentRunRuntime,
		ResumeSession: agentRunResume,
		Language:      session.Config().Language,
	})
	if err != nil {
		if report != nil {
			if agentRunJSON {
				if data, marshalErr := json.MarshalIndent(report, "", "  "); marshalErr == nil {
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				}
			} else {
				printAgentReport(report)
			}
		}
		return err
	}
	if agentRunJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		printAgentReport(report)
	}
	if report.Status == "blocked" {
		return fmt.Errorf("agent goal blocked: %s", strings.Join(report.RemainingIssues, "; "))
	}
	return nil
}

func printAgentReport(report *app.AgentGoalReport) {
	if report == nil {
		return
	}
	fmt.Printf("Status: %s\n", report.Status)
	fmt.Printf("Summary: %s\n", report.Summary)
	if len(report.SkillsUsed) > 0 {
		fmt.Printf("Skills used: %s\n", strings.Join(report.SkillsUsed, ", "))
	}
	if len(report.ChangedTargets) > 0 {
		fmt.Printf("Changed targets: %s\n", strings.Join(report.ChangedTargets, ", "))
	}
	if len(report.Plan) > 0 {
		fmt.Println("Plan:")
		for _, item := range report.Plan {
			fmt.Printf("  [%s] %s: %s\n", item.Status, item.ID, item.Text)
		}
	}
	if len(report.Verification) > 0 {
		fmt.Printf("Verification: %s\n", strings.Join(report.Verification, " | "))
	}
	if len(report.RemainingIssues) > 0 {
		fmt.Printf("Remaining issues: %s\n", strings.Join(report.RemainingIssues, " | "))
	}
	if report.LiveLogPath != "" {
		fmt.Printf("Live log: %s\n", report.LiveLogPath)
	}
	if report.SessionID != "" {
		fmt.Printf("Session: %s\n", report.SessionID)
	}
}
