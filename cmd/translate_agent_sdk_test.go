package cmd

import (
	"strings"
	"testing"
)

func TestTranslateCommandExposesAgentSDKFlag(t *testing.T) {
	flag := translateCmd.Flags().Lookup("agent-sdk")
	if flag == nil {
		t.Fatalf("translate missing --agent-sdk flag")
	}
	if !strings.Contains(flag.Usage, "Agent SDK") {
		t.Fatalf("translate --agent-sdk usage = %q, want Agent SDK mention", flag.Usage)
	}
	if !strings.Contains(translateCmd.Long, "--agent-sdk") {
		t.Fatalf("translate help should include an --agent-sdk example")
	}
}
