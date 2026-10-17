package store_test

import (
	"testing"

	"novelgen/internal/platform/store"
)

type memoryBackend struct {
	root *store.Root
}

func (memoryBackend) Name() string { return "memory" }

func (b memoryBackend) Open(string) (*store.Root, error) {
	return b.root, nil
}

func (b memoryBackend) Locate(string) (*store.Root, error) {
	return b.root, nil
}

func TestBackendSelectionDrivesProjectOpenAndDefault(t *testing.T) {
	root := store.MemRoot()
	store.SetBackend(memoryBackend{root: root})
	defer store.SetBackend(nil)
	store.ResetDefault()
	defer store.ResetDefault()

	opened, err := store.OpenProject("ignored")
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if opened != root {
		t.Fatalf("OpenProject returned %p, want %p", opened, root)
	}

	located, err := store.LocateProject("")
	if err != nil {
		t.Fatalf("LocateProject: %v", err)
	}
	if located != root {
		t.Fatalf("LocateProject returned %p, want %p", located, root)
	}

	ambient, err := store.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if ambient != root {
		t.Fatalf("Default returned %p, want %p", ambient, root)
	}
}
