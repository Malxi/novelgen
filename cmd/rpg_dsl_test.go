package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"novelgen/internal/rpg/dsl"
)

func TestLoadDSLFragments(t *testing.T) {
	chdirRepoRoot(t)

	bookName := "fire-galaxy"
	rpgDir := filepath.Join("books", bookName, "story", "rpg")

	fmt.Printf("RPG Dir: %s\n", rpgDir)
	fmt.Printf("Exists: %v\n", dirExists(rpgDir))

	// List files
	entries, err := os.ReadDir(rpgDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	fmt.Printf("Files in dir:\n")
	for _, entry := range entries {
		fmt.Printf("  - %s\n", entry.Name())
	}

	// Test glob
	pattern := filepath.Join(rpgDir, "*.rpg")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}

	fmt.Printf("\nGlob pattern: %s\n", pattern)
	fmt.Printf("Matches: %v\n", matches)

	// Try loading fragments
	fragments, err := loadDSLFragments(bookName)
	if err != nil {
		t.Fatalf("loadDSLFragments failed: %v", err)
	}

	fmt.Printf("\nLoaded %d fragments\n", len(fragments))
	for _, f := range fragments {
		fmt.Printf("  - %s (%s)\n", f.FilePath, f.Phase)
	}
}

// chdirRepoRoot switches to the module root so the test can read books/...,
// independent of the package directory depth.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", wd)
		}
		dir = parent
	}
	if dir != wd {
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("chdir to module root failed: %v", err)
		}
		t.Cleanup(func() { _ = os.Chdir(wd) })
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func TestDSLWriteAndRead(t *testing.T) {
	// Create a simple DSL
	dslData := &dsl.DSL{
		Metadata: &dsl.Metadata{
			Title:      "Test",
			DSLVersion: "0.2.0",
		},
		Characters: &dsl.Characters{
			Player: &dsl.Player{
				ID:   "test_player",
				Name: "Test Player",
				Stats: dsl.Stats{
					HP: 100,
				},
			},
		},
	}

	// Write to temp file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.rpg")

	if err := dslData.WriteToFile(tempFile); err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	fmt.Printf("Written to: %s\n", tempFile)

	// Read it back
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	fmt.Printf("Content:\n%s\n", string(content))

	// Parse it
	parser := dsl.NewParser(string(content))
	parsed, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("\nParsed successfully!\n")
	fmt.Printf("Title: %s\n", parsed.Metadata.Title)
}
