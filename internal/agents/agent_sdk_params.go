package agents

import "strings"

// dedupeToolAllowlist trims each allowlist entry, drops blanks and repeats
// (keeping the first occurrence) and preserves input order across all sources.
// Compose and write previously carried identical private copies of this.
func dedupeToolAllowlist(sources ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, values := range sources {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// agentSDKToolParams builds the Agent SDK invocation shape shared by every stage
// that drives project facts through `novelgen tool ...` in Bash. It pins the
// policy those stages must agree on — a Bash-only tool surface, the
// non-interactive "dontAsk" permission mode, and the SDK requirement — so a
// stage only supplies its workflow skills, tool allowlist, evidence gate and
// turn/time budget.
//
// Stages that deliberately expose no tools (recap, translate) build InvokeParams
// directly instead of calling this helper.
func agentSDKToolParams(skills []string, command string, allowlist []string, evidence ToolEvidenceRequirement, maxTurns, timeout int) InvokeParams {
	return InvokeParams{
		SDKSkills:      skills,
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  allowlist,
		ToolEvidence:   evidence,
		MaxTurns:       maxTurns,
		Timeout:        timeout,
		Command:        command,
	}
}
