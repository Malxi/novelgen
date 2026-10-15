// Package architecture pins the allowed import direction between internal
// packages so new cross-layer dependencies fail the build instead of silently
// eroding the layering.
package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// layer assigns each internal package a level. A package may import another
// package only when that package's level is lower or equal (same cluster).
var layer = map[string]int{
	"internal/models":                      0,
	"internal/utils":                       1,
	"internal/envutil":                     1,
	"internal/logger":                      1,
	"internal/usage":                       1,
	"internal/workerpool":                  1,
	"internal/cli":                         1,
	"internal/testfixtures":                1,
	"internal/toolcmd":                     1,
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
