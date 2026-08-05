package cmd

import (
	"strings"
	"testing"
)

func TestValidatePolishAgentApplyOptionRequiresAgentSDK(t *testing.T) {
	if err := validatePolishAgentApplyOption(true, true); err != nil {
		t.Fatalf("expected --agent-sdk --agent-apply to pass, got %v", err)
	}
	if err := validatePolishAgentApplyOption(false, false); err != nil {
		t.Fatalf("expected no agent flags to pass, got %v", err)
	}
	err := validatePolishAgentApplyOption(false, true)
	if err == nil || !strings.Contains(err.Error(), "--agent-apply requires --agent-sdk") {
		t.Fatalf("expected --agent-apply validation error, got %v", err)
	}
}

func TestPolishCommandExposesAgentSDKFlags(t *testing.T) {
	for _, name := range []string{"agent-sdk", "agent-apply", "recap-agent-sdk"} {
		if polishCmd.Flags().Lookup(name) == nil {
			t.Fatalf("polish flag %q is not registered", name)
		}
	}
}

func TestEffectivePolishConcurrency(t *testing.T) {
	if got := effectivePolishConcurrency(4, 10, true); got != 1 {
		t.Fatalf("agent-sdk concurrency = %d, want 1", got)
	}
	if got := effectivePolishConcurrency(4, 2, false); got != 2 {
		t.Fatalf("bounded concurrency = %d, want 2", got)
	}
	if got := effectivePolishConcurrency(0, 3, false); got != 1 {
		t.Fatalf("default concurrency = %d, want 1", got)
	}
	if got := effectivePolishConcurrency(2, 0, true); got != 0 {
		t.Fatalf("empty concurrency = %d, want 0", got)
	}
}
