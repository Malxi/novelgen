package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// internalModule assigns a package to the new top-level module layout:
//
//	contract  -> pure persisted contracts and tiny shared vocabulary
//	platform  -> storage, LLM/runtime, logging and leaf utilities
//	fiction   -> the story/RPG authoring domain
//	toolplane -> the agent-facing tool adapter
//	app       -> the composition root
//
// The numeric layer table still orders packages inside a module; this test
// keeps the module graph one-way.
func internalModule(pkg string) string {
	switch {
	case pkg == "internal/architecture":
		return "meta"
	case pkg == "internal/contract" || strings.HasPrefix(pkg, "internal/contract/"):
		return "contract"
	case strings.HasPrefix(pkg, "internal/platform/"):
		return "platform"
	case strings.HasPrefix(pkg, "internal/fiction/"):
		return "fiction"
	case pkg == "internal/toolplane":
		return "toolplane"
	case pkg == "internal/agentkit" || pkg == "internal/agentmcp":
		return "app"
	case pkg == "internal/app":
		return "app"
	default:
		return ""
	}
}

func moduleRank(module string) int {
	switch module {
	case "contract":
		return 0
	case "platform":
		return 1
	case "fiction":
		return 2
	case "toolplane":
		return 3
	case "app":
		return 4
	default:
		return -1
	}
}

// fictionOrder orders the bounded submodules inside fiction. A package may
// import only a submodule at the same or a lower rank. In particular, rpg is a
// leaf subdomain, checks/mutations sit below agents, and the writing pipeline is
// the outermost fiction service.
func fictionOrder(pkg string) int {
	rest := strings.TrimPrefix(pkg, "internal/fiction/")
	switch {
	case rest == "rpg" || strings.HasPrefix(rest, "rpg/"):
		return 0
	case strings.HasPrefix(rest, "domain/"):
		return 1
	case strings.HasPrefix(rest, "check"):
		return 2
	case strings.HasPrefix(rest, "mutation"):
		return 3
	case strings.HasPrefix(rest, "agents"):
		return 4
	case strings.HasPrefix(rest, "derive"):
		return 5
	case strings.HasPrefix(rest, "refresh"):
		return 6
	case strings.HasPrefix(rest, "pipeline"):
		return 7
	default:
		return -1
	}
}

func TestInternalModuleBoundaries(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	for from := range layer {
		if from == "internal/architecture" {
			continue
		}
		fromModule := internalModule(from)
		if fromModule == "" {
			t.Errorf("internal package %q is not assigned to a module", from)
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(from))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", filepath.Join(from, name), err)
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if !strings.HasPrefix(path, "novelgen/internal/") {
					continue
				}
				to := strings.TrimPrefix(path, "novelgen/")
				toModule := internalModule(to)
				if toModule == "" {
					t.Errorf("%s imports %s, which is not assigned to a module", from, to)
					continue
				}
				if moduleRank(toModule) > moduleRank(fromModule) {
					t.Errorf("module violation: %s (%s) imports %s (%s); dependencies must point inward", from, fromModule, to, toModule)
				}
				if fromModule == "fiction" && toModule == "fiction" {
					fromOrder, toOrder := fictionOrder(from), fictionOrder(to)
					if fromOrder < 0 || toOrder < 0 {
						t.Errorf("fiction package %s or %s is missing a fiction submodule order", from, to)
						continue
					}
					if toOrder > fromOrder {
						t.Errorf("fiction submodule violation: %s (order %d) imports %s (order %d)", from, fromOrder, to, toOrder)
					}
				}
			}
		}
	}
}
