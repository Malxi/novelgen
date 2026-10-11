package agents

import (
	"fmt"
	"novelgen/internal/agentruntime"
	"strings"
)

// agentEvidenceConfig maps a Go-side evidence requirement into the runner
// contract so the runner can enforce it in-flight (before the agent stops).
func agentEvidenceConfig(req ToolEvidenceRequirement) agentruntime.ToolEvidence {
	return agentruntime.ToolEvidence{
		MinQueryCalls:        req.MinQueryCalls,
		MinContextQueryCalls: req.MinContextQueryCalls,
		MinCheckCalls:        req.MinCheckCalls,
		MinPatchApplyCalls:   req.MinPatchApplyCalls,
		RequiredToolCommands: append([]string(nil), req.RequiredToolCommands...),
		RequireNoDeniedTools: req.RequireNoDeniedTools,
	}
}

func validateAgentRuntimeToolEvidence(agentName string, requirement ToolEvidenceRequirement, result *agentruntime.Result) error {
	if requirement.MinQueryCalls <= 0 &&
		requirement.MinContextQueryCalls <= 0 &&
		requirement.MinCheckCalls <= 0 &&
		requirement.MinPatchApplyCalls <= 0 &&
		requirement.MaxQueryCalls <= 0 &&
		requirement.MaxContextQueryCalls <= 0 &&
		!requirement.DisallowQueryBriefCalls &&
		!requirement.DisallowQueryFullCalls &&
		!requirement.RequirePatchApplyFollowupCheck &&
		!requirement.RequireNoDeniedTools &&
		len(requirement.RequiredToolCommands) == 0 {
		return nil
	}
	if result == nil || result.LiveSummary == nil {
		return fmt.Errorf("%s Agent SDK live summary is required to verify tool execution", agentName)
	}
	summary := result.LiveSummary
	if summary.QueryCalls < requirement.MinQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: query calls=%d, want at least %d", agentName, summary.QueryCalls, requirement.MinQueryCalls)
	}
	if summary.ContextQueryCalls < requirement.MinContextQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: context query calls=%d, want at least %d", agentName, summary.ContextQueryCalls, requirement.MinContextQueryCalls)
	}
	if summary.CheckCalls < requirement.MinCheckCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: check calls=%d, want at least %d", agentName, summary.CheckCalls, requirement.MinCheckCalls)
	}
	if summary.PatchApplies < requirement.MinPatchApplyCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: patch apply calls=%d, want at least %d", agentName, summary.PatchApplies, requirement.MinPatchApplyCalls)
	}
	if requirement.MaxQueryCalls > 0 && summary.QueryCalls > requirement.MaxQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: query calls=%d, want at most %d", agentName, summary.QueryCalls, requirement.MaxQueryCalls)
	}
	if requirement.MaxContextQueryCalls > 0 && summary.ContextQueryCalls > requirement.MaxContextQueryCalls {
		return fmt.Errorf("%s Agent SDK tool evidence failed: context query calls=%d, want at most %d", agentName, summary.ContextQueryCalls, requirement.MaxContextQueryCalls)
	}
	if requirement.DisallowQueryBriefCalls && summary.QueryBriefCalls > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: brief query calls=%d, want 0", agentName, summary.QueryBriefCalls)
	}
	if requirement.DisallowQueryFullCalls && summary.QueryFullCalls > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: full query calls=%d, want 0", agentName, summary.QueryFullCalls)
	}
	if requirement.RequirePatchApplyFollowupCheck && summary.ApplyWithoutFollowupCheck > 0 {
		return fmt.Errorf("%s Agent SDK tool evidence failed: patch apply calls without follow-up check=%d", agentName, summary.ApplyWithoutFollowupCheck)
	}
	if requirement.RequireNoDeniedTools && summary.ToolDenied > 0 {
		blockingDenied := blockingDeniedToolCommands(summary, requirement)
		if len(blockingDenied) > 0 && !summary.DenialsResolved {
			return fmt.Errorf("%s Agent SDK tool evidence failed: denied tool calls=%d commands=%s", agentName, len(blockingDenied), strings.Join(blockingDenied, " | "))
		}
		if len(summary.DeniedToolCommands) == 0 && len(summary.WorkflowDeniedToolCommands) == 0 {
			return fmt.Errorf("%s Agent SDK tool evidence failed: denied tool calls=%d", agentName, summary.ToolDenied)
		}
	}
	for _, required := range requirement.RequiredToolCommands {
		if !liveSummaryHasToolCommand(summary, required) {
			return fmt.Errorf("%s Agent SDK tool evidence failed: required tool command not observed: %s", agentName, required)
		}
	}
	return nil
}

func liveSummaryHasToolCommand(summary *agentruntime.LiveSummary, required string) bool {
	if summary == nil {
		return false
	}
	required = normalizeToolEvidenceCommand(required)
	if required == "" {
		return true
	}
	for _, command := range summary.AllowedToolCommands {
		observed := normalizeToolEvidenceCommand(command)
		if observed == required || strings.Contains(observed, required) || strings.Contains(required, observed) {
			return true
		}
	}
	return false
}

func normalizeToolEvidenceCommand(command string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	command = strings.ReplaceAll(command, `"`, "")
	command = strings.Join(strings.Fields(command), " ")
	return command
}

func blockingDeniedToolCommands(summary *agentruntime.LiveSummary, requirement ToolEvidenceRequirement) []string {
	if summary == nil {
		return nil
	}
	commands := append([]string(nil), summary.DeniedToolCommands...)
	if len(commands) == 0 {
		return nil
	}
	toleratePatchApplyRetry := summary.PatchApplies >= maxInt(1, requirement.MinPatchApplyCalls)
	if requirement.RequirePatchApplyFollowupCheck && summary.ApplyWithoutFollowupCheck > 0 {
		toleratePatchApplyRetry = false
	}
	blocking := make([]string, 0, len(commands))
	for _, command := range commands {
		if summaryHasWorkflowDeniedCommand(summary, command) {
			continue
		}
		if toleratePatchApplyRetry && (isDeniedPatchCommand(command) || isDeniedPatchBufferCommand(command) || isDeniedClaudeToolResultRead(command) || isDeniedPostApplyInspectionCommand(command) || isDeniedPostApplyContextExpansionCommand(command) || isDeniedPostApplyRefreshCommand(command)) {
			continue
		}
		blocking = append(blocking, command)
	}
	return blocking
}

func summaryHasWorkflowDeniedCommand(summary *agentruntime.LiveSummary, command string) bool {
	if summary == nil {
		return false
	}
	normalized := normalizeToolEvidenceCommand(command)
	for _, workflowCommand := range summary.WorkflowDeniedToolCommands {
		observed := normalizeToolEvidenceCommand(workflowCommand)
		if observed == normalized {
			return true
		}
	}
	return false
}

func isDeniedPatchCommand(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(normalized, " tool patch ")
}

func isDeniedPatchBufferCommand(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(normalized, " tool patch-buffer ")
}

func isDeniedClaudeToolResultRead(command string) bool {
	normalized := strings.ToLower(strings.TrimSpace(command))
	if (strings.HasPrefix(normalized, "type ") || strings.HasPrefix(normalized, "get-content ")) &&
		strings.Contains(normalized, "tool-results") {
		return true
	}
	if strings.Contains(normalized, "get-content") &&
		strings.Contains(normalized, `\temp\claude\`) &&
		strings.Contains(normalized, `\tasks\`) &&
		strings.Contains(normalized, ".output") {
		return true
	}
	return false
}

func isDeniedPostApplyInspectionCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	if strings.Contains(normalized, "novelgen tool query chapter") &&
		strings.Contains(normalized, "--content") {
		return true
	}
	return false
}

func isDeniedPostApplyContextExpansionCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	return strings.Contains(normalized, "novelgen tool query context --type chapter-repair") &&
		strings.Contains(normalized, "--view brief")
}

func isDeniedPostApplyRefreshCommand(command string) bool {
	normalized := normalizeToolEvidenceCommand(command)
	return strings.Contains(normalized, "novelgen tool refresh chapter-dsl")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
