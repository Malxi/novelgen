package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeCommandStaysSplitByWorkflow pins the split of the former 2.8k-line
// compose.go. Command definitions stay in compose.go; generation, improvement,
// review, pipeline, state and support helpers each live in their own file.
func TestComposeCommandStaysSplitByWorkflow(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "cmd", "composecmd")
	want := map[string][]string{
		"compose.go":          {"var composeCmd", "func NewCommand", "func init"},
		"compose_gen.go":      {"func runComposeGen", "func generateOutlineHierarchical"},
		"compose_improve.go":  {"func runComposeImprove", "func iterateOutlineImprovementAgentSDK"},
		"compose_review.go":   {"func runComposeReview", "func runComposeCheck"},
		"compose_pipeline.go": {"func runComposePipeline"},
		"compose_state.go":    {"func openComposeSession", "func saveOutlineMutation"},
		"compose_support.go":  {"func runComposeSkeletonImprove", "func isTransientAgentSDKError"},
	}

	seen := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "compose") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if lines := strings.Count(string(data), "\n") + 1; lines > 1000 {
			t.Errorf("%s has %d lines; compose workflows must stay in their own file", name, lines)
		}
		markers, ok := want[name]
		if !ok {
			t.Errorf("%s is not classified by the compose command layout test", name)
			continue
		}
		seen[name] = true
		for _, marker := range markers {
			if !strings.Contains(string(data), marker) {
				t.Errorf("%s no longer contains %q", name, marker)
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("compose command layout file %s is missing", name)
		}
	}
}
