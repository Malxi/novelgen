package check

import (
	"path/filepath"
	"testing"

	"novelgen/internal/platform/store"
)

func TestRunToolCustomRulesOnSetup(t *testing.T) {
	root, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := root.Write(root.SetupJSON(), []byte(`{"title":"a deus ex ending"}`), 0o644); err != nil {
		t.Fatalf("write setup: %v", err)
	}
	rule := `{"id":"no-deus-ex","target":"setup","severity":"high","kind":"regex","pattern":"deus ex","message":"avoid deus ex","suggestion":"add a cost"}`
	if err := root.Write(filepath.Join(root.RulesDir(), "no-deus-ex.json"), []byte(rule), 0o644); err != nil {
		t.Fatalf("write rule: %v", err)
	}

	result, err := RunToolCustomRules(root, "setup", "", "")
	if err != nil {
		t.Fatalf("run custom rules: %v", err)
	}
	if !result.Blocking || result.OK {
		t.Fatalf("expected blocking result: %#v", result)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].Category != "no-deus-ex" || result.Issues[0].Priority != "high" {
		t.Fatalf("issue = %#v", result.Issues[0])
	}
	if result.Summary.High != 1 || result.Summary.Total != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
}
