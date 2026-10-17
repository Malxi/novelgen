package base

// AgentSDKLogToolAllowlist returns the shared read-only log query commands
// available to SDK agents.
func AgentSDKLogToolAllowlist() []string {
	return []string{
		"novelgen tool query logs --view index",
		"novelgen tool query logs --view index --limit 5",
		"novelgen tool query logs --view brief",
		"novelgen tool query logs --type prompts --view index",
		"novelgen tool query logs --type prompts --view index --limit 5",
		"novelgen tool query logs --type prompts --view brief",
		"novelgen tool query logs --type responses --view index",
		"novelgen tool query logs --type responses --view index --limit 5",
		"novelgen tool query logs --type responses --view brief",
		"novelgen tool query logs --type agent-live --view index",
		"novelgen tool query logs --type agent-live --view index --limit 5",
		"novelgen tool query logs --type agent-live --view brief",
		"novelgen tool query logs --id",
	}
}
