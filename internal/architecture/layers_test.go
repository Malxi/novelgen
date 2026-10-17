// Package architecture pins the allowed import direction between internal
// packages so new cross-layer dependencies fail the build instead of silently
// eroding the layering.
package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ambientRootLayer is the first layer whose packages are pipeline stages rather
// than leaves: a stage that resolves the ambient project root can silently act
// on whatever project the process happens to be in, which already caused one
// reading-the-wrong-project bug (docs/ARCHITECTURE.md §10).
const ambientRootLayer = 5

// ambientRootAllowlist pins the files that still resolve the ambient root from a
// pipeline layer. New callers must take an explicit *store.Root instead; the
// entries below are the remaining ones that have no root parameter to use yet.
var ambientRootAllowlist = map[string]string{
	"internal/llm/config.go":              "provider/config resolution is reached without a project handle",
	"internal/llm/client.go":              "resolves the SDK workspace root; no root parameter on the client",
	"internal/agents/agent_base.go":       "skill directory lookup with no root parameter",
	"internal/agents/agent_logging.go":    "agent log root falls back to the ambient project",
	"internal/writersdl/write_rpg_dsl.go": "the failure dump only has a book name to work with",
}

// ambientProjectHelpers are the project package's convenience loaders. They
// resolve store.Default internally, so an AST check for .Default() alone misses
// them even though they cause the same wrong-project bug.
var ambientProjectHelpers = map[string]bool{
	"LoadConfig":      true,
	"LoadStorySetup":  true,
	"LoadOutline":     true,
	"LoadAllElements": true,
	"FindRoot":        true,
}

// TestAmbientProjectRootUseIsPinned keeps new pipeline code from reaching for the
// process-wide project root. store.Default / project.Default are fine at the
// edges (a command entry point, the project facade), and they are what make the
// CLI work without plumbing a root everywhere, but from a pipeline layer they
// mean "operate on whichever project the process is in", which is how a check
// once read a different project than the one the caller injected.
func TestAmbientProjectRootUseIsPinned(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var violations []string
	ambientRootAllowlistHits := map[string]int{}

	for pkgPath, level := range layer {
		if level < ambientRootLayer {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(pkgPath))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			rel := pkgPath + "/" + name
			_, allowed := ambientRootAllowlist[rel]
			if allowed {
				ambientRootAllowlistHits[rel] = 0
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", rel, err)
			}
			var hits []string
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Default" {
					if !ok {
						return true
					}
					pkg, isIdent := selector.X.(*ast.Ident)
					if !isIdent || pkg.Name != "project" || !ambientProjectHelpers[selector.Sel.Name] {
						return true
					}
					hits = append(hits, fmt.Sprintf("%s:%d: project.%s() in a pipeline layer; take an explicit *store.Root/project.Book instead",
						rel, fset.Position(call.Pos()).Line, selector.Sel.Name))
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok || (pkg.Name != "store" && pkg.Name != "project") {
					return true
				}
				hits = append(hits, fmt.Sprintf("%s:%d: %s.Default() in a pipeline layer; take an explicit *store.Root instead",
					rel, fset.Position(call.Pos()).Line, pkg.Name))
				return true
			})
			if allowed {
				ambientRootAllowlistHits[rel] = len(hits)
				continue
			}
			violations = append(violations, hits...)
		}
	}

	if len(violations) > 0 {
		t.Errorf("ambient project root must not be resolved by new pipeline code:\n  %s",
			strings.Join(violations, "\n  "))
	}
	for rel := range ambientRootAllowlist {
		if ambientRootAllowlistHits[rel] == 0 {
			t.Errorf("ambientRootAllowlist entry %q no longer resolves the ambient root; remove it", rel)
		}
	}
}

// layer assigns each internal package a level. A package may import another
// package only when that package's level is lower or equal (same cluster).
var layer = map[string]int{
	"internal/models":       0,
	"internal/utils":        1,
	"internal/envutil":      1,
	"internal/usage":        1,
	"internal/workerpool":   1,
	"internal/cli":          1,
	"internal/testfixtures": 1,
	"internal/toolcmd":      1,
	// logger writes into the project layout (the logs/ directory), so it sits at
	// the storage layer rather than beside the other leaf utilities.
	"internal/logger":                      2,
	"internal/store":                       2,
	"internal/rpg":                         2,
	"internal/rpg/dsl":                     2,
	"internal/rpg/benchmark":               2,
	"internal/logic":                       3,
	"internal/logic/continuity/character":  3,
	"internal/logic/continuity/recap":      3,
	"internal/logic/continuity/transition": 3,
	"internal/logic/style":                 3,
	"internal/project":                     4,
	"internal/agentruntime":                5,
	"internal/llm":                         6,
	"internal/agents":                      7,
	"internal/rpgllm":                      7,
	"internal/continuity":                  8,
	"internal/reviewstore":                 8,
	"internal/quality":                     8,
	"internal/app":                         8,
	"internal/writersdl":                   8,
	"internal/toolplane":                   8,
	"internal/writepipe":                   8,
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func TestInternalImportLayers(t *testing.T) {
	root := repoRoot(t)
	internalDir := filepath.Join(root, "internal")

	entries, err := os.ReadDir(internalDir)
	if err != nil {
		t.Fatalf("read internal dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkgDir := filepath.Join(internalDir, entry.Name())
		if err := checkPackageRecursive(t, root, pkgDir); err != nil {
			t.Error(err)
		}
	}
}

// indirectAmbientAllowlist pins the remaining pipeline files that resolve a
// project/config root without receiving one. These are the files reachable
// without a project handle (agent logs, provider config, and the refresh
// helper); everything else must take an explicit *store.Root or *project.Book.
var indirectAmbientAllowlist = map[string]string{
	"internal/agents/agent_base.go":       "SDK workspace fallback for callers without a root",
	"internal/agents/agent_logging.go":    "agent logs have no root parameter yet",
	"internal/app/session.go":             "composition root resolves global config once",
	"internal/llm/client.go":              "RuntimeClient workspace root",
	"internal/llm/config.go":              "provider config path discovery",
	"internal/toolplane/tool_refresh.go":  "refresh context has no session handle yet",
	"internal/writersdl/write_rpg_dsl.go": "failure dump only has a book name",
}

// TestPipelineLayersDoNotResolveAmbientRootIndirectly closes the gap left by
// TestAmbientProjectRootUseIsPinned: that test only sees literal .Default()
// calls and a few project helpers. A helper such as agentWorkspaceRoot() or a
// fresh store.Open(agentWorkspaceRoot()) is just as capable of reading the
// wrong project, so pipeline layers must not call it without an allowlist
// entry.
func TestPipelineLayersDoNotResolveAmbientRootIndirectly(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var violations []string
	hits := map[string]int{}

	for pkgPath, level := range layer {
		if level < ambientRootLayer {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(pkgPath))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			rel := pkgPath + "/" + name
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", rel, err)
			}
			_, allowed := indirectAmbientAllowlist[rel]
			if allowed {
				hits[rel] = 0
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				reason := indirectAmbientCall(call)
				if reason == "" {
					return true
				}
				if allowed {
					hits[rel]++
					return true
				}
				violations = append(violations, fmt.Sprintf("%s:%d: %s", rel, fset.Position(call.Pos()).Line, reason))
				return true
			})
		}
	}

	if len(violations) > 0 {
		t.Errorf("pipeline layers must not resolve the ambient project/config through helpers:\n  %s",
			strings.Join(violations, "\n  "))
	}
	for rel := range indirectAmbientAllowlist {
		if hits[rel] == 0 {
			t.Errorf("indirectAmbientAllowlist entry %q no longer resolves an ambient root; remove it", rel)
		}
	}
}

func indirectAmbientCall(call *ast.CallExpr) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if ok {
		pkg, isIdent := selector.X.(*ast.Ident)
		if !isIdent {
			return ""
		}
		switch pkg.Name {
		case "store":
			switch selector.Sel.Name {
			case "Default", "Open", "OpenProject", "LocateProject":
				return "store." + selector.Sel.Name + "() in a pipeline layer"
			}
		case "project":
			switch selector.Sel.Name {
			case "Default", "FindRoot", "LoadConfig", "LoadStorySetup", "LoadOutline", "LoadAllElements":
				return "project." + selector.Sel.Name + "() in a pipeline layer"
			}
		case "cli":
			if selector.Sel.Name == "CommandContext" {
				return "cli.CommandContext() in a pipeline layer"
			}
		case "llm":
			if selector.Sel.Name == "LoadOrCreateConfig" || selector.Sel.Name == "GetConfigPath" {
				return "llm." + selector.Sel.Name + "() in a pipeline layer"
			}
		}
		return ""
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	switch ident.Name {
	case "agentWorkspaceRoot", "agentLogRoot", "runtimeWorkspaceRoot":
		return ident.Name + "() in a pipeline layer"
	}
	return ""
}

func checkPackageRecursive(t *testing.T, root, dir string) error {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	hasGo := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			hasGo = true
			break
		}
	}
	if hasGo {
		if err := checkPackage(t, root, dir); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := checkPackageRecursive(t, root, filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPackage(t *testing.T, root, dir string) error {
	t.Helper()
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	from := filepath.ToSlash(rel)
	if from == "internal/architecture" {
		return nil
	}
	fromLayer, ok := layer[from]
	if !ok {
		t.Errorf("package %q is not classified in the architecture layer table", from)
		return nil
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if path == "novelgen/cmd" || strings.HasPrefix(path, "novelgen/cmd/") {
				t.Errorf("layer violation: %s imports %s; internal packages must never depend on the CLI command layer", from, path)
				continue
			}
			target := strings.TrimPrefix(path, "novelgen/")
			if !strings.HasPrefix(target, "internal/") {
				continue
			}
			toLayer, ok := layer[target]
			if !ok {
				t.Errorf("%s imports unclassified internal package %q", from, target)
				continue
			}
			if toLayer > fromLayer {
				t.Errorf("layer violation: %s (L%d) imports %s (L%d); imports may only go to lower layers", from, fromLayer, target, toLayer)
			}
		}
	}
	return nil
}

// TestCmdGroupIsolation pins the CLI packaging rule: a command group under
// cmd/<group>/ is an independent entry point. Groups may depend on internal/*
// and on shared internal libraries (for example internal/writepipe), but never
// on another command group or on the cmd root package, so importing one
// command can never drag in another command's wiring.
func TestCmdGroupIsolation(t *testing.T) {
	root := repoRoot(t)
	cmdDir := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("read cmd dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		group := entry.Name()
		groupDir := filepath.Join(cmdDir, group)
		files, err := os.ReadDir(groupDir)
		if err != nil {
			t.Fatalf("read %s: %v", groupDir, err)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(groupDir, f.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", f.Name(), err)
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path == "novelgen/cmd" || strings.HasPrefix(path, "novelgen/cmd/") {
					t.Errorf("cmd group %q imports %q; command groups must stay independent of other command packages", "cmd/"+group, path)
				}
			}
		}
	}
}

// TestCmdRootStaysWiringOnly keeps the root cmd package as pure wiring: files
// outside the registry may only pull in a command subpackage from a
// *_register.go file, so command logic cannot creep back into package cmd.
func TestCmdRootStaysWiringOnly(t *testing.T) {
	root := repoRoot(t)
	cmdDir := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("read cmd dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if strings.HasSuffix(name, "_register.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(cmdDir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, "novelgen/cmd/") {
				t.Errorf("cmd/%s imports %q; command subpackages must be wired only from *_register.go files", name, path)
			}
		}
	}
}
