// Package project is the project-scoped view over internal/platform/store.
//
// It is the compatibility facade the CLI, the write pipeline and the tools have
// always used: the free functions at the bottom of this file resolve the
// project from the ambient root (cmd/root.go installs it once per invocation).
// Code that already holds a *store.Root uses Book, which is the same behavior
// without the ambient lookup — that is how tests drive a command against a
// temporary project instead of chdir-ing the process into it.
package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/logic"
	"novelgen/internal/platform/store"
)

// Book is a project root plus the chapter-text semantics that belong to the
// project layer: which legacy file name a chapter may live under, and how a
// chapter body is normalized before it is stored.
type Book struct {
	*store.Root
}

// Open wraps a storage root as a project view.
func Open(root *store.Root) *Book {
	return &Book{Root: root}
}

// Default returns the ambient project view, resolving the project root from the
// working directory when no root has been installed.
func Default() (*Book, error) {
	root, err := store.Default()
	if err != nil {
		return nil, err
	}
	return Open(root), nil
}

// ForCommand returns the project a command should operate on: a root attached to
// the command context, then the ambient root, then the project containing the
// working directory.
func ForCommand(cmd *cobra.Command) (*Book, error) {
	if cmd != nil {
		if root, ok := store.FromContext(cmd.Context()); ok {
			return Open(root), nil
		}
	}
	return Default()
}

// CommandDir returns the directory a command works in, without requiring a
// project to exist yet. Commands that create a project (init) and commands that
// only report "not a project" need this lenient form.
func CommandDir(cmd *cobra.Command) (*store.Root, error) {
	if cmd != nil {
		if root, ok := store.FromContext(cmd.Context()); ok {
			return root, nil
		}
	}
	if root, err := store.Default(); err == nil {
		return root, nil
	}
	return store.OpenProject(".")
}

// FindRoot walks up from the current working directory until it finds the
// directory containing novel.json.
func FindRoot() (string, error) {
	root, err := store.Default()
	if err != nil {
		return "", err
	}
	return root.Dir(), nil
}

// LoadConfig loads novel.json from the project root.
func LoadConfig() (*models.ProjectConfig, error) {
	book, err := Default()
	if err != nil {
		return nil, err
	}
	return book.LoadConfig()
}

// LoadStorySetup loads story/setup/story_setup.json from the project root.
func LoadStorySetup() (*models.StorySetup, error) {
	book, err := Default()
	if err != nil {
		return nil, err
	}
	return book.LoadSetup()
}

// LoadOutline loads story/compose/outline.json from the project root.
func LoadOutline() (*models.Outline, error) {
	book, err := Default()
	if err != nil {
		return nil, err
	}
	return book.LoadOutline()
}

// LoadAllElements loads the four craft element files and normalizes them.
// Missing files yield empty maps (not an error), matching the previous CLI
// behavior; parse failures are reported.
func LoadAllElements() (
	map[string]*models.Character,
	map[string]*models.Location,
	map[string]*models.Item,
	map[string]*models.Organization,
	error,
) {
	book, err := Default()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return book.LoadElements()
}

// NormalizeLoadedElements drops nil entries and applies per-type craft
// normalization in place.
func NormalizeLoadedElements(
	characters map[string]*models.Character,
	locations map[string]*models.Location,
	items map[string]*models.Item,
	organizations map[string]*models.Organization,
) {
	store.NormalizeElements(characters, locations, items, organizations)
}

// AllChapters returns every chapter in the outline, in reading order.
func AllChapters(outline *models.Outline) []*models.Chapter {
	idManager := logic.NewIDManager(outline)
	return idManager.GetAllChapters()
}

// KnownCharacterNames returns the sorted, de-duplicated set of character names
// referenced by any chapter in the outline.
func KnownCharacterNames(outline *models.Outline) []string {
	if outline == nil {
		return nil
	}
	set := map[string]bool{}
	for _, ch := range AllChapters(outline) {
		if ch == nil {
			continue
		}
		for _, name := range ch.Characters {
			if name = strings.TrimSpace(name); name != "" {
				set[name] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// VolumesForSelection resolves the volume/chapter selection flags shared by the
// draft and write commands: a chapter flag selects its volume, a volume flag
// selects that volume, and otherwise every volume is returned.
//
// An explicitly supplied selection that cannot be resolved is an error. Falling
// back to "all volumes" here would turn a typo such as --volume 999 into a
// command that rewrites the whole novel.
func VolumesForSelection(outline *models.Outline, volumeFlag, chapterFlag string) ([]*models.Volume, error) {
	idManager := logic.NewIDManager(outline)

	if chapterFlag != "" {
		chapterID, err := idManager.ResolveChapterID(chapterFlag, "", "")
		if err != nil {
			return nil, fmt.Errorf("resolve chapter %q: %w", chapterFlag, err)
		}
		_, vol, _ := idManager.GetChapterByID(chapterID)
		if vol == nil {
			return nil, fmt.Errorf("chapter %q does not belong to a volume", chapterID)
		}
		return []*models.Volume{vol}, nil
	} else if volumeFlag != "" {
		volumeID, err := idManager.ResolveVolumeID(volumeFlag, "")
		if err != nil {
			return nil, fmt.Errorf("resolve volume %q: %w", volumeFlag, err)
		}
		vol, _ := idManager.GetVolumeByID(volumeID)
		if vol == nil {
			return nil, fmt.Errorf("volume %q not found", volumeID)
		}
		return []*models.Volume{vol}, nil
	}

	return idManager.GetAllVolumes(), nil
}
