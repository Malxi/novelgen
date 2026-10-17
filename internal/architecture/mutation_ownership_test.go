package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentArtifactWritesUseMutationService pins the ownership boundary of the
// migration: command-level Agent outputs must not write canonical setup,
// outline, craft, chapter, recap, or draft artifacts directly. Those writes go
// through internal/fiction/mutation.MutationService so normalization, checkpoints,
// atomic writes and the in-process mutation lock are applied once.
//
// Derived files (outline.md/story_setup.md), logs, reports, exports and
// pipeline progress snapshots are intentionally outside this guard.
func TestAgentArtifactWritesUseMutationService(t *testing.T) {
	root := repoRoot(t)
	checks := map[string][]string{
		"cmd/setupcmd/setup.go":                {"book.SaveSetup("},
		"cmd/composecmd/compose.go":            {"book.SaveSetup(", "book.SaveOutline("},
		"cmd/craftcmd/craft.go":                {"WriteJSONMerging("},
		"cmd/writecmd/write.go":                {"book.SaveFinalChapter("},
		"cmd/polishcmd/polish.go":              {"book.SaveFinalChapter("},
		"cmd/recap/recap.go":                   {"store.Save("},
		"cmd/draftcmd/draft.go":                {"book.SaveDraft(", "recapStore.Save("},
		"internal/fiction/pipeline/context.go": {"book.SaveFinalChapter("},
		"internal/fiction/pipeline/recap.go":   {"recapStore.Save("},
	}
	for rel, forbidden := range checks {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		source := string(data)
		for _, needle := range forbidden {
			if strings.Contains(source, needle) {
				t.Errorf("%s still writes an Agent artifact directly with %q; route it through fiction/mutation.MutationService", rel, needle)
			}
		}
	}
}
