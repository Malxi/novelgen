package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// toolCommandScopes are the packages that hold `novelgen tool ...` strings: the
// tool plane that returns them, the agents that hand them to the SDK workflow,
// the runtime that summarizes them, and the CLI.
var toolCommandScopes = []string{"internal/toolplane", "internal/fiction/agents", "internal/platform/agentruntime", "cmd"}

// toolCommandFlagOrder is the canonical flag order of the tool plane commands,
// the same order internal/contract/toolcmd emits. Flag order is part of the contract: the
// SDK runner matches a command against the workflow allowlist by canonical
// suffix, so a command that spells the same flags in a different order is
// denied, not normalized. Quoting is not part of the contract — the runner
// strips quotes before comparing.
var toolCommandFlagOrder = map[string][]string{
	"check":        {"--target", "--scope", "--id", "--category", "--min-priority", "--max-issues", "--target-words"},
	"query":        {"--type", "--id", "--chapter-id", "--volume-id", "--entity-type", "--name", "--fields", "--content", "--view", "--limit"},
	"patch":        {"--target", "--id", "--patch-json", "--patch-buffer", "--task", "--apply", "--refresh-derived", "--target-words"},
	"patch-buffer": {"--id", "--text", "--stdin", "--max-chars"},
	"refresh":      {"--id", "--batch-size"},
}

// TestToolCommandsUseCanonicalFlagOrder checks every `novelgen tool ...` string
// that is still written by hand. Commands built by internal/contract/toolcmd cannot drift
// — the builder owns their order — but the literals in prompt text, allowlists
// and documentation examples can, and an unknown flag, a misspelled subcommand
// or two flags in the wrong order is exactly the drift that makes an agent run
// something the workflow allowlist then refuses.
func TestToolCommandsUseCanonicalFlagOrder(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	checked := 0
	for _, scope := range toolCommandScopes {
		dir := filepath.Join(root, filepath.FromSlash(scope))
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			file, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				t.Fatalf("parse %s: %v", rel, parseErr)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr != nil {
					return true
				}
				command, ok := trimToToolCommand(value)
				if !ok {
					return true
				}
				checked++
				for _, problem := range toolCommandProblems(command) {
					t.Errorf("%s:%d: %s (%s)", filepath.ToSlash(rel), fset.Position(literal.Pos()).Line, problem, command)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", scope, err)
		}
	}
	// The floor catches a scanner that stopped matching the vocabulary rather
	// than pinning how many literals the repository happens to hold.
	if checked < 150 {
		t.Fatalf("only %d command strings checked; the scanner probably stopped matching the tool plane vocabulary", checked)
	}
}

// trimToToolCommand returns the `novelgen tool ...` command inside a string,
// which may be a bare command or a shell example with a printf stdin prefix.
func trimToToolCommand(value string) (string, bool) {
	index := strings.Index(value, "novelgen tool ")
	if index < 0 {
		return "", false
	}
	command := strings.TrimSpace(value[index:])
	if strings.Contains(command, "\n") {
		return "", false
	}
	return command, true
}

func toolCommandProblems(command string) []string {
	tokens := strings.Fields(command)
	if len(tokens) < 3 {
		return nil
	}
	subcommand := tokens[2]
	order, known := toolCommandFlagOrder[subcommand]
	if !known {
		// Prose such as "novelgen tool queries" or "novelgen tool query/check"
		// mentions the command family without invoking it. Only a token list
		// that carries flags is treated as a command invocation, so a misspelled
		// subcommand is still reported there.
		for _, token := range tokens[3:] {
			if strings.HasPrefix(token, "--") {
				return []string{"unknown tool subcommand"}
			}
		}
		return nil
	}
	rank := map[string]int{}
	for i, flag := range order {
		rank[flag] = i
	}

	var problems []string
	lastRank := -1
	for _, token := range tokens[3:] {
		if !strings.HasPrefix(token, "--") {
			continue
		}
		flag := token
		if index := strings.Index(flag, "="); index >= 0 {
			flag = flag[:index]
		}
		value, ok := rank[flag]
		if !ok {
			problems = append(problems, "unknown flag "+flag)
			continue
		}
		if value < lastRank {
			problems = append(problems, "flag "+flag+" is out of canonical order")
		}
		lastRank = value
	}
	return problems
}
