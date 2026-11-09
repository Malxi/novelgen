package projectcmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/runmetrics"

	"github.com/spf13/cobra"
)

var (
	projectRunsJSON  bool
	projectRunsLimit int
	projectRunsStuck bool
	projectRunsSince string
	projectRunJSON   bool
)

var projectRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List Agent run metrics and stuck signals",
	Long: `List deterministic Agent run metrics recorded under logs/runs/.

Each run records the CLI command, model calls, requested/loaded skills, tool
commands, denials, durations, token usage and deterministic stuck signals.
Use --stuck to show only runs with a stuck signal, or --json for dashboards.`,
	Args: cobra.NoArgs,
	RunE: runProjectRuns,
}

var projectRunCmd = &cobra.Command{
	Use:   "run <run_id>",
	Short: "Show one Agent run timeline",
	Long: `Show the event timeline for one Agent run.

The detail view includes model calls, skill loads, tool commands, permission
decisions and stuck signals. Use --json for machine-readable output.`,
	Args: cobra.ExactArgs(1),
	RunE: runProjectRun,
}

func init() {
	projectRunsCmd.Flags().BoolVar(&projectRunsJSON, "json", false, "Print machine-readable run summaries")
	projectRunsCmd.Flags().IntVar(&projectRunsLimit, "limit", 20, "Maximum number of runs to show (0 = all)")
	projectRunsCmd.Flags().BoolVar(&projectRunsStuck, "stuck", false, "Only show runs with a stuck signal")
	projectRunsCmd.Flags().StringVar(&projectRunsSince, "since", "", "Only show runs at or after this time (RFC3339 or 2006-01-02)")
	projectRunCmd.Flags().BoolVar(&projectRunJSON, "json", false, "Print the complete event timeline as JSON")
	projectCmd.AddCommand(projectRunsCmd)
	projectCmd.AddCommand(projectRunCmd)
}

func runProjectRuns(cmd *cobra.Command, _ []string) error {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	opts := runmetrics.ListOptions{Limit: projectRunsLimit, Stuck: projectRunsStuck}
	if strings.TrimSpace(projectRunsSince) != "" {
		opts.Since, err = parseUsageSince(projectRunsSince)
		if err != nil {
			return err
		}
	}
	runs, err := runmetrics.List(book.Root(), opts)
	if err != nil {
		return fmt.Errorf("read run metrics: %w", err)
	}
	if projectRunsJSON {
		return printJSON(cmd, runs)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Agent runs for %s\n", book.Root().Dir())
	fmt.Fprintf(out, "Ledger: %s\n", book.Root().RunMetricsDir())
	if len(runs) == 0 {
		fmt.Fprintln(out, "No runs recorded yet. Run a creative or tool command first.")
		return nil
	}
	for _, run := range runs {
		stuck := ""
		if run.StuckEvents > 0 {
			reasons := make([]string, 0, len(run.Stuck))
			for _, signal := range run.Stuck {
				reasons = append(reasons, signal.Reason)
			}
			stuck = " stuck=" + truncateUsageText(strings.Join(reasons, ", "), 90)
		}
		fmt.Fprintf(out, "%s  %-9s %s", run.RunID, run.Status, truncateUsageText(run.Command, 72))
		meta := ""
		if origin := run.Metadata["origin"]; origin != "" {
			meta = " origin=" + origin
		}
		if tool := run.Metadata["tool"]; tool != "" {
			meta += " tool=" + tool
		}
		fmt.Fprintf(out, "  calls=%d tools=%d tokens=%d%s%s\n",
			run.AgentCalls, run.ToolCalls, run.TotalTokens, meta, stuck)
	}
	return nil
}

func runProjectRun(cmd *cobra.Command, args []string) error {
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	detail, err := runmetrics.Read(book.Root(), args[0])
	if err != nil {
		return fmt.Errorf("read run %s: %w", args[0], err)
	}
	if projectRunJSON {
		return printJSON(cmd, detail)
	}
	out := cmd.OutOrStdout()
	summary := detail.Summary
	fmt.Fprintf(out, "Run: %s\n", summary.RunID)
	fmt.Fprintf(out, "Command: %s\n", summary.Command)
	fmt.Fprintf(out, "Status: %s\n", summary.Status)
	if origin := summary.Metadata["origin"]; origin != "" {
		fmt.Fprintf(out, "Origin: %s\n", origin)
	}
	if tool := summary.Metadata["tool"]; tool != "" {
		fmt.Fprintf(out, "Tool: %s\n", tool)
	}
	if session := summary.Metadata["mcp_session"]; session != "" {
		fmt.Fprintf(out, "MCP session: %s\n", session)
	}
	fmt.Fprintf(out, "Window: %s .. %s\n", summary.StartedAt, summary.FinishedAt)
	fmt.Fprintf(out, "Calls: %d  Tools: %d  Denied: %d  Tokens: %d\n",
		summary.AgentCalls, summary.ToolCalls, summary.ToolDenied, summary.TotalTokens)
	if len(summary.Skills) > 0 {
		fmt.Fprintf(out, "Skills: %s\n", strings.Join(summary.Skills, ", "))
	}
	if len(summary.Stuck) > 0 {
		fmt.Fprintln(out, "Stuck signals:")
		for _, signal := range summary.Stuck {
			fmt.Fprintf(out, "  - %s\n", signal.Reason)
		}
	}
	fmt.Fprintln(out, "\nTimeline:")
	for _, event := range detail.Events {
		parts := []string{event.Time.Format(time.RFC3339), event.Event}
		if event.Status != "" {
			parts = append(parts, event.Status)
		}
		if event.Agent != "" {
			parts = append(parts, event.Agent)
		}
		if event.Kind != "" {
			parts = append(parts, event.Kind)
		}
		if event.DurationMS > 0 {
			parts = append(parts, fmt.Sprintf("%dms", event.DurationMS))
		}
		if event.Command != "" {
			parts = append(parts, truncateUsageText(event.Command, 120))
		}
		if event.Reason != "" {
			parts = append(parts, "reason="+truncateUsageText(event.Reason, 120))
		}
		fmt.Fprintln(out, strings.Join(parts, " | "))
	}
	return nil
}

func printJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
