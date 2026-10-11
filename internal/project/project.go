// Package project centralizes locating a Novelgen project directory and
// loading its persisted state (config, story setup, outline, craft elements).
//
// The CLI used to keep this logic as unexported helpers inside the single
// `cmd` package, which made it impossible to split commands into subpackages.
// Moving it here gives every command group one shared, testable entry point.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// FindRoot walks up from the current working directory until it finds the
// directory containing novel.json.
func FindRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := models.FindProjectRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("project root not found (novel.json not found in current or parent directories)")
	}
	return root, nil
}

// LoadConfig loads novel.json from the project root.
func LoadConfig() (*models.ProjectConfig, error) {
	root, err := FindRoot()
	if err != nil {
		return nil, err
	}
	return models.LoadProjectConfig(filepath.Join(root, "novel.json"))
}

// LoadStorySetup loads story/setup/story_setup.json from the project root.
func LoadStorySetup() (*models.StorySetup, error) {
	root, err := FindRoot()
	if err != nil {
		return nil, err
	}
	return models.LoadStorySetup(filepath.Join(root, "story", "setup", "story_setup.json"))
}

// LoadOutline loads story/compose/outline.json from the project root.
func LoadOutline() (*models.Outline, error) {
	root, err := FindRoot()
	if err != nil {
		return nil, err
	}
	return models.LoadOutline(filepath.Join(root, "story", "compose", "outline.json"))
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
	root, err := FindRoot()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	characters, err := loadCraftFile[models.Character](root, "characters.json")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse characters: %w", err)
	}
	locations, err := loadCraftFile[models.Location](root, "locations.json")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse locations: %w", err)
	}
	items, err := loadCraftFile[models.Item](root, "items.json")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse items: %w", err)
	}
	organizations, err := loadCraftFile[models.Organization](root, "organizations.json")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse organizations: %w", err)
	}

	NormalizeLoadedElements(characters, locations, items, organizations)
	return characters, locations, items, organizations, nil
}

func loadCraftFile[T any](root, filename string) (map[string]*T, error) {
	values := make(map[string]*T)
	data, err := os.ReadFile(filepath.Join(root, "story", "craft", filename))
	if err != nil {
		return values, nil
	}
	data = utils.StripUTF8BOM(data)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// NormalizeLoadedElements drops nil entries and applies per-type craft
// normalization in place.
func NormalizeLoadedElements(
	characters map[string]*models.Character,
	locations map[string]*models.Location,
	items map[string]*models.Item,
	organizations map[string]*models.Organization,
) {
	for name, character := range characters {
		if character == nil {
			delete(characters, name)
			continue
		}
		character.NormalizeForCraft(name)
	}
	for name, location := range locations {
		if location == nil {
			delete(locations, name)
			continue
		}
		location.NormalizeForCraft(name)
	}
	for name, item := range items {
		if item == nil {
			delete(items, name)
			continue
		}
		item.NormalizeForCraft(name)
	}
	for name, organization := range organizations {
		if organization == nil {
			delete(organizations, name)
			continue
		}
		organization.NormalizeForCraft(name)
	}
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
func VolumesForSelection(outline *models.Outline, volumeFlag, chapterFlag string) []*models.Volume {
	idManager := logic.NewIDManager(outline)

	if chapterFlag != "" {
		chapterID, err := idManager.ResolveChapterID(chapterFlag, "", "")
		if err == nil {
			_, vol, _ := idManager.GetChapterByID(chapterID)
			if vol != nil {
				return []*models.Volume{vol}
			}
		}
	} else if volumeFlag != "" {
		volumeID, err := idManager.ResolveVolumeID(volumeFlag, "")
		if err == nil {
			vol, _ := idManager.GetVolumeByID(volumeID)
			if vol != nil {
				return []*models.Volume{vol}
			}
		}
	}

	return idManager.GetAllVolumes()
}
