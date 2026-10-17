package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestReferencedSkillsExist keeps the two embedded prompt trees honest. The
// chat tree and the SDK workflow tree are intentionally different, but a typo
// in a Skills/SDKSkills literal currently only fails at runtime, after a model
// call has already started.
func TestReferencedSkillsExist(t *testing.T) {
	root := repoRoot(t)
	chatDir := filepath.Join(root, "internal", "agents", "base", "skills")
	sdkDir := filepath.Join(root, "internal", "agentruntime", "skills")

	chat := referencedSkillFields(t, filepath.Join(root, "internal", "agents"), "Skills")
	sdk := referencedSkillFields(t, filepath.Join(root, "internal", "agents"), "SDKSkills")
	for name := range referencedWorkflowLiterals(t, filepath.Join(root, "internal", "agents")) {
		sdk[name] = true
	}
	for name := range referencedWorkflowLiterals(t, filepath.Join(root, "internal", "agentruntime")) {
		sdk[name] = true
	}

	for _, name := range sortedSet(chat) {
		if _, err := os.Stat(filepath.Join(chatDir, name, "SKILL.md")); err != nil {
			t.Errorf("chat skill %q is referenced but %s does not exist", name, filepath.Join(chatDir, name, "SKILL.md"))
		}
	}
	for _, name := range sortedSet(sdk) {
		if _, err := os.Stat(filepath.Join(sdkDir, name, "SKILL.md")); err != nil {
			t.Errorf("SDK skill %q is referenced but %s does not exist", name, filepath.Join(sdkDir, name, "SKILL.md"))
		}
	}
	if len(chat) == 0 && len(sdk) == 0 {
		t.Fatal("no skill references found; the scanner stopped matching the skill vocabulary")
	}
}

func referencedSkillFields(t *testing.T, dir, field string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	walkSkillSources(t, dir, func(file *ast.File, fset *token.FileSet) {
		ast.Inspect(file, func(node ast.Node) bool {
			kv, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != field {
				return true
			}
			lit, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, elt := range lit.Elts {
				if value, ok := stringLiteral(elt); ok {
					out[value] = true
				}
			}
			return true
		})
	})
	return out
}

func referencedWorkflowLiterals(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	walkSkillSources(t, dir, func(file *ast.File, fset *token.FileSet) {
		ast.Inspect(file, func(node ast.Node) bool {
			if value, ok := stringLiteral(node); ok && strings.HasSuffix(value, "-workflow") {
				out[value] = true
			}
			return true
		})
	})
	return out
}

func walkSkillSources(t *testing.T, dir string, visit func(*ast.File, *token.FileSet)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		visit(file, fset)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
}

func stringLiteral(node ast.Node) (string, bool) {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func sortedSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
