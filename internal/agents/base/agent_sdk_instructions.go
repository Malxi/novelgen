package base

import "fmt"

// AgentSDKNonASCIIPatchRule returns the rule for sending non-ASCII patch JSON to
// patchCommand. When stdinExample is true the rule also spells out the stdin
// pipe; call sites whose surrounding instruction already shows that pipe pass
// false. Compose, craft and setup previously each inlined their own copy of this
// rule, which made the flag advice easy to drift apart.
func AgentSDKNonASCIIPatchRule(patchCommand string, stdinExample bool) string {
	rule := "For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. "
	if stdinExample {
		rule += fmt.Sprintf("Pipe compact literal JSON on stdin instead: `printf '%%s' '<compact-json>' | %s`. ", patchCommand)
	}
	return rule + "Use --patch-json only for small ASCII-only patches."
}

// AgentSDKOnlyAllowedWriteClause returns the clause that pins the only permitted
// write to patchCommand with --apply after a successful dry-run. It is a
// sentence fragment so callers can attach it to their own "do not write files"
// sentence.
func AgentSDKOnlyAllowedWriteClause(patchCommand string) string {
	return fmt.Sprintf("the only allowed write is `%s --apply` after dry-run validation.", patchCommand)
}

// AgentSDKOnlyAllowedWriteSentence is AgentSDKOnlyAllowedWriteClause as a
// standalone sentence.
func AgentSDKOnlyAllowedWriteSentence(patchCommand string) string {
	clause := AgentSDKOnlyAllowedWriteClause(patchCommand)
	return "T" + clause[1:]
}
