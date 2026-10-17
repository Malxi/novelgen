package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeAgentStaysSplitByConcern pins the split of the former 4.4k-line
// compose_agent.go. A new concern must land in a named file instead of growing
// the core back into a monolith.
func TestComposeAgentStaysSplitByConcern(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "fiction", "agents", "compose")
	want := map[string][]string{
		"compose_agent.go":          {"type ComposeAgent struct", "func NewComposeAgent", "func (a *ComposeAgent) SetProjectRoot"},
		"compose_agent_sdk.go":      {"func (a *ComposeAgent) GenerateSkeletonWithAgentSDK", "func (a *ComposeAgent) IterateHierarchicalAgentSDK", "func applyAgentSDKVolumePatch"},
		"compose_agent_patch.go":    {"type composeVolumePatch struct", "func mergeOutlineScenesByOrder"},
		"compose_agent_loop.go":     {"type ImproveProgress struct", "func (a *ComposeAgent) improveVolumesWithCheckpoint("},
		"compose_agent_generate.go": {"type ComposeSkeletonInput struct", "func (a *ComposeAgent) Generate(", "func (a *ComposeAgent) GenerateSkeleton("},
		"compose_agent_prompt.go":   {"type composeGenPromptInput struct", "func (a *ComposeAgent) buildSetupBrief"},
	}

	seen := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "compose_agent") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		lines := strings.Count(string(data), "\n") + 1
		if lines > 1800 {
			t.Errorf("%s has %d lines; compose concerns must stay in their own file", name, lines)
		}
		markers, ok := want[name]
		if !ok {
			t.Errorf("%s is not classified by the compose layout test", name)
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
			t.Errorf("compose layout file %s is missing", name)
		}
	}
}
