package craft

import (
	"os"
	"testing"

	"novelgen/internal/store"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "novelgen-agent-logs-craft-")
	if err != nil {
		panic(err)
	}
	root, err := store.Open(dir)
	if err != nil {
		panic(err)
	}
	store.SetDefault(root)

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}
