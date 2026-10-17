// Cloning and checkpointing project state around a write, so a patch can be rolled back.

package mutation

import (
	"encoding/json"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

func CloneOutline(outline *models.Outline) *models.Outline {
	if outline == nil {
		return nil
	}
	data, _ := json.Marshal(outline)
	var cloned models.Outline
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}

func CloneStorySetup(setup *models.StorySetup) models.StorySetup {
	if setup == nil {
		return models.StorySetup{}
	}
	data, _ := json.Marshal(setup)
	var cloned models.StorySetup
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

// saveToolCheckpointJSON snapshots the state a patch is about to replace.
//
// This used to be five near-identical functions, one per patch section, each
// with its own copy of the checkpoint directory and timestamp format. Naming and
// writing now live in internal/platform/store (SaveCheckpoint), which also makes the
// snapshot write atomic.
func saveToolCheckpointJSON(root *store.Root, scope store.Scope, prefix string, value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return writeToolCheckpoint(root, scope, prefix, ".json", data)
}

func writeToolCheckpoint(root *store.Root, scope store.Scope, prefix, ext string, data []byte) (string, error) {
	return root.SaveCheckpoint(scope, prefix, ext, data)
}

func saveToolChapterPatchCheckpoint(root *store.Root, chapterID, beforePath, beforeContent string) (string, error) {
	var sb strings.Builder
	sb.WriteString("<!-- novelgen tool patch chapter checkpoint")
	if strings.TrimSpace(beforePath) != "" {
		sb.WriteString(" source=\"")
		sb.WriteString(beforePath)
		sb.WriteString("\"")
	}
	sb.WriteString(" -->\n\n")
	sb.WriteString(beforeContent)
	return writeToolCheckpoint(root, store.ScopeChapters, chapterID, ".md", []byte(sb.String()))
}
