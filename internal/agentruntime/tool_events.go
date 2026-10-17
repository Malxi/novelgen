package agentruntime

import "strings"

// SummarizeToolEvents builds the evidence counters from structured runner
// events. It is preferred over parsing the live JSONL log when the runner
// supplies events; the live summary remains the compatibility fallback.
func SummarizeToolEvents(events []ToolEvent) *LiveSummary {
	if len(events) == 0 {
		return nil
	}
	usePre := false
	for _, event := range events {
		if event.Source == "pre" {
			usePre = true
			break
		}
	}

	summary := &LiveSummary{}
	pendingApply := map[string]bool{}
	for _, event := range events {
		switch event.Source {
		case "post":
			summary.ToolDurationMS += event.DurationMS
			if event.DurationMS > summary.SlowestToolDurationMS {
				summary.SlowestToolDurationMS = event.DurationMS
				summary.SlowestToolCommand = event.Command
			}
			if isToolCheckCommand(event.Command) {
				clear(pendingApply)
			}
			continue
		case "pre":
			if !usePre {
				continue
			}
		case "permission":
			if usePre {
				continue
			}
		default:
			continue
		}

		summary.ToolCalls++
		if event.Allowed {
			summary.ToolAllowed++
			addAllowedToolCommand(summary, event.Command)
			countToolCommand(summary, event.Command)
			if event.PatchApply && event.PatchKey != "" {
				pendingApply[event.PatchKey] = true
			}
			continue
		}
		summary.ToolDenied++
		addDeniedToolCommand(summary, event.Command)
		if strings.TrimSpace(event.Reason) != "" {
			summary.WorkflowDeniedToolCommands = append(summary.WorkflowDeniedToolCommands, event.Command)
		}
	}
	summary.ApplyWithoutFollowupCheck = len(pendingApply)
	return summary
}
