package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleBoundary pins the webui integration contract: reads are in-process
// and may only use novelgen/internal/models from the parent module; the webui
// must never import the CLI command packages (novelgen/cmd/...); all mutations
// and long tasks go through the CLI binary via exec.
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
			if strings.HasPrefix(path, "novelgen/internal/") && path != "novelgen/internal/models" {
				t.Errorf("%s imports %q: webui may only use novelgen/internal/models from the parent module", name, path)
			}
		}
	}
}
