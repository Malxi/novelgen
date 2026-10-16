package dsl

import (
	"path/filepath"
	"testing"
)

func TestDebugLoadDSL(t *testing.T) {
	// The debug dumper only prints; it must not depend on a local books/ fixture
	// being present, so point it at the repository temp dir layout instead.
	DebugLoadDSL(filepath.Join(t.TempDir(), "story", "rpg"))
}
