package base

import "testing"

func TestAgentSDKNonASCIIPatchRuleWithStdinExample(t *testing.T) {
	const command = "novelgen tool patch craft --target item --id <name>"
	got := AgentSDKNonASCIIPatchRule(command, true)
	want := "For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. " +
		"Pipe compact literal JSON on stdin instead: `printf '%s' '<compact-json>' | " + command + "`. " +
		"Use --patch-json only for small ASCII-only patches."
	if got != want {
		t.Fatalf("rule = %q, want %q", got, want)
	}
}

func TestAgentSDKNonASCIIPatchRuleWithoutStdinExample(t *testing.T) {
	got := AgentSDKNonASCIIPatchRule("novelgen tool patch setup", false)
	want := "For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. Use --patch-json only for small ASCII-only patches."
	if got != want {
		t.Fatalf("rule = %q, want %q", got, want)
	}
}

func TestAgentSDKOnlyAllowedWriteClauseAndSentence(t *testing.T) {
	const command = "novelgen tool patch setup ..."
	clause := AgentSDKOnlyAllowedWriteClause(command)
	if want := "the only allowed write is `" + command + " --apply` after dry-run validation."; clause != want {
		t.Fatalf("clause = %q, want %q", clause, want)
	}
	sentence := AgentSDKOnlyAllowedWriteSentence(command)
	if want := "The only allowed write is `" + command + " --apply` after dry-run validation."; sentence != want {
		t.Fatalf("sentence = %q, want %q", sentence, want)
	}
}
