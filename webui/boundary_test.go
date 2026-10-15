package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleBoundary pins the webui integration contract: the parent module is
// reached only through the novelgen binary (exec). The webui must not import
// novelgen/cmd/** or any novelgen/internal/** package — reads pass project
// files through as raw JSON and every derived artifact is written by the CLI.
func TestModuleBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, "novelgen/cmd") {
				t.Errorf("%s imports %q: webui must not depend on the CLI command packages", name, path)
			}
			if strings.HasPrefix(path, "novelgen/internal/") {
				t.Errorf("%s imports %q: webui must reach the parent module only through the novelgen binary", name, path)
			}
		}
	}
}
