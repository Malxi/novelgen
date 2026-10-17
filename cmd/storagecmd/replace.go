package storagecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/store"
	"novelgen/internal/toolplane"

	"github.com/spf13/cobra"
)

// runReplace is the editor-facing canonical write path. It accepts a complete
// artifact (not a patch) and dispatches it through MutationService, so schema
// checks, normalization, checkpoints and the project mutation lock still apply
// when the caller is an external editor rather than `tool patch`.
func runReplace(cmd *cobra.Command, _ []string) error {
	root, err := openProject()
	if err != nil {
		return fail(cmd, "invalid_project", err)
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fail(cmd, "read_stdin", err)
	}
	path, err := root.ResolveProjectPath(storagePath)
	if err != nil {
		return fail(cmd, "invalid_path", err)
	}
	path = filepath.Clean(path)
	service := toolplane.NewMutationService(root)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	opts := toolplane.MutationOptions{Validate: true, Source: "storage-replace"}

	switch {
	case samePath(path, root.SetupJSON()):
		var setup models.StorySetup
		if err := json.Unmarshal(stripBOM(data), &setup); err != nil {
			return fail(cmd, "invalid_json", err)
		}
		if _, err := service.ReplaceSetup(ctx, &setup, opts); err != nil {
			return fail(cmd, "mutation_error", err)
		}
	case samePath(path, root.OutlineJSON()):
		var outline models.Outline
		if err := json.Unmarshal(stripBOM(data), &outline); err != nil {
			return fail(cmd, "invalid_json", err)
		}
		if _, err := service.ReplaceOutline(ctx, &outline, opts); err != nil {
			return fail(cmd, "mutation_error", err)
		}
	case isCraftPath(root, path):
		kind := craftKindForPath(root, path)
		var elements map[string]json.RawMessage
		if err := json.Unmarshal(stripBOM(data), &elements); err != nil {
			return fail(cmd, "invalid_json", err)
		}
		if _, err := service.ReplaceCraftElements(ctx, kind, elements, opts); err != nil {
			return fail(cmd, "mutation_error", err)
		}
	case isChapterPath(root, path):
		id := chapterIDFromPath(path)
		if _, err := service.ReplaceChapter(ctx, id, string(data), opts); err != nil {
			return fail(cmd, "mutation_error", err)
		}
	case isRecapPath(root, path):
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		var recap models.ChapterRecap
		if err := json.Unmarshal(stripBOM(data), &recap); err != nil {
			return fail(cmd, "invalid_json", err)
		}
		if recap.ChapterID == "" {
			recap.ChapterID = id
		}
		if _, err := service.ReplaceRecap(ctx, &recap, opts); err != nil {
			return fail(cmd, "mutation_error", err)
		}
	default:
		return fail(cmd, "unsupported_path", fmt.Errorf("storage replace does not own %s", storagePath))
	}
	return succeed(cmd, response{OK: true})
}

func stripBOM(data []byte) []byte {
	return []byte(strings.TrimPrefix(string(data), "\ufeff"))
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func isCraftPath(root *store.Root, path string) bool {
	for _, kind := range []store.CraftKind{store.CraftCharacters, store.CraftLocations, store.CraftItems, store.CraftOrganizations} {
		if samePath(path, root.CraftFile(kind)) {
			return true
		}
	}
	return false
}

func craftKindForPath(root *store.Root, path string) store.CraftKind {
	for _, kind := range []store.CraftKind{store.CraftCharacters, store.CraftLocations, store.CraftItems, store.CraftOrganizations} {
		if samePath(path, root.CraftFile(kind)) {
			return kind
		}
	}
	return ""
}

func isChapterPath(root *store.Root, path string) bool {
	dir := filepath.Clean(root.ChaptersDir()) + string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(path), dir) && strings.HasSuffix(path, ".md")
}

func chapterIDFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.TrimPrefix(base, "chapter-")
}

func isRecapPath(root *store.Root, path string) bool {
	dir := filepath.Clean(root.RecapsDir()) + string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(path), dir) && strings.HasSuffix(path, ".json")
}
