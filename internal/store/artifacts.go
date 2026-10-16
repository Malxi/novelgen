package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// ReadJSON decodes a JSON project file into out. A UTF-8 BOM is tolerated,
// because hand-edited craft and outline files regularly carry one.
func (r *Root) ReadJSON(path string, out any) error {
	data, err := r.Read(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(utils.StripUTF8BOM(data), out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// WriteJSON writes indented JSON to a project path, atomically.
func (r *Root) WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return r.Write(path, data, FileMode)
}

// WriteJSONMerging writes value as indented JSON, merging its top-level keys
// into whatever object already lives at path (new keys win). Callers use it to
// update a subset of fields without dropping the rest.
func (r *Root) WriteJSONMerging(path string, value any) error {
	existing := make(map[string]any)
	if data, err := r.Read(path); err == nil {
		if err := json.Unmarshal(utils.StripUTF8BOM(data), &existing); err != nil {
			return fmt.Errorf("failed to parse existing data: %w", err)
		}
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal new data: %w", err)
	}
	var incoming map[string]any
	if err := json.Unmarshal(encoded, &incoming); err != nil {
		return fmt.Errorf("failed to parse new data: %w", err)
	}
	for key, item := range incoming {
		existing[key] = item
	}
	return r.WriteJSON(path, existing)
}

// HasConfig reports whether novel.json exists.
func (r *Root) HasConfig() bool { return r.Exists(r.NovelJSON()) }

// LoadConfig reads novel.json.
func (r *Root) LoadConfig() (*models.ProjectConfig, error) {
	data, err := r.Read(r.NovelJSON())
	if err != nil {
		return nil, err
	}
	return models.ParseProjectConfig(utils.StripUTF8BOM(data))
}

// SaveConfig writes novel.json, stamping UpdatedAt.
func (r *Root) SaveConfig(config *models.ProjectConfig) error {
	if config == nil {
		return errors.New("cannot save nil project config")
	}
	data, err := config.MarshalForSave()
	if err != nil {
		return err
	}
	return r.Write(r.NovelJSON(), data, FileMode)
}

// HasSetup reports whether the story setup JSON exists.
func (r *Root) HasSetup() bool { return r.Exists(r.SetupJSON()) }

// LoadSetup reads story/setup/story_setup.json.
func (r *Root) LoadSetup() (*models.StorySetup, error) {
	data, err := r.Read(r.SetupJSON())
	if err != nil {
		return nil, err
	}
	return models.ParseStorySetup(utils.StripUTF8BOM(data))
}

// SaveSetup writes story/setup/story_setup.json, normalized.
func (r *Root) SaveSetup(setup *models.StorySetup) error {
	if setup == nil {
		return errors.New("cannot save nil story setup")
	}
	data, err := setup.MarshalForSave()
	if err != nil {
		return err
	}
	return r.Write(r.SetupJSON(), data, FileMode)
}

// HasOutline reports whether the outline JSON exists.
func (r *Root) HasOutline() bool { return r.Exists(r.OutlineJSON()) }

// LoadOutline reads story/compose/outline.json. The outline is a project's
// largest artifact and the failure mode that motivated this package: a
// half-written outline.json used to be indistinguishable from a valid one.
func (r *Root) LoadOutline() (*models.Outline, error) {
	return r.LoadOutlineAt(r.OutlineJSON())
}

// LoadOutlineAt reads an outline from an explicit path. It is used for the
// per-iteration and progress files the compose pipeline writes, which are the
// same shape as the canonical outline.
func (r *Root) LoadOutlineAt(path string) (*models.Outline, error) {
	data, err := r.Read(path)
	if err != nil {
		return nil, err
	}
	return models.ParseOutline(utils.StripUTF8BOM(data))
}

// SaveOutline writes story/compose/outline.json, normalized and atomically.
func (r *Root) SaveOutline(outline *models.Outline) error {
	return r.SaveOutlineAt(r.OutlineJSON(), outline)
}

// SaveOutlineAt writes an outline to an explicit path.
func (r *Root) SaveOutlineAt(path string, outline *models.Outline) error {
	if outline == nil {
		return errors.New("cannot save nil outline")
	}
	data, err := outline.MarshalForSave()
	if err != nil {
		return err
	}
	return r.Write(path, data, FileMode)
}

// LoadElements loads and normalizes the four craft element files. Missing files
// yield empty maps rather than an error, matching the CLI behavior the pipeline
// stages rely on for partially built projects.
func (r *Root) LoadElements() (
	map[string]*models.Character,
	map[string]*models.Location,
	map[string]*models.Item,
	map[string]*models.Organization,
	error,
) {
	characters, err := loadCraftFile[models.Character](r, CraftCharacters)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse characters: %w", err)
	}
	locations, err := loadCraftFile[models.Location](r, CraftLocations)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse locations: %w", err)
	}
	items, err := loadCraftFile[models.Item](r, CraftItems)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse items: %w", err)
	}
	organizations, err := loadCraftFile[models.Organization](r, CraftOrganizations)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to parse organizations: %w", err)
	}

	NormalizeElements(characters, locations, items, organizations)
	return characters, locations, items, organizations, nil
}

func loadCraftFile[T any](r *Root, kind CraftKind) (map[string]*T, error) {
	values := make(map[string]*T)
	data, err := r.Read(r.CraftFile(kind))
	if err != nil {
		return values, nil
	}
	if err := json.Unmarshal(utils.StripUTF8BOM(data), &values); err != nil {
		return nil, err
	}
	return values, nil
}

// NormalizeElements drops nil entries and applies per-type craft normalization
// in place.
func NormalizeElements(
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

// readCraftJSON returns the raw bytes of one craft file. found is false when the
// file does not exist, which the patch plane treats as "create it".
func (r *Root) readCraftJSON(kind CraftKind) ([]byte, bool, error) {
	data, err := r.Read(r.CraftFile(kind))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return utils.StripUTF8BOM(data), true, nil
}

// LoadCraftRaw decodes one craft file into a string-keyed map, the shape the
// patch plane and the schema checks work with.
func (r *Root) LoadCraftRaw(kind CraftKind) (map[string]map[string]any, error) {
	values := make(map[string]map[string]any)
	data, found, err := r.readCraftJSON(kind)
	if err != nil || !found {
		return values, err
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// LoadChapterContent returns the stored markdown for a chapter and the path it
// was found at, walking the legacy file-name candidates. Both return values are
// empty when the chapter has not been written yet.
func (r *Root) LoadChapterContent(chapterID string) (string, string) {
	for _, path := range r.ChapterCandidates(chapterID) {
		if data, err := r.Read(path); err == nil {
			return string(data), path
		}
	}
	return "", ""
}

// SaveChapterContent writes chapters/chapter-<id>.md.
func (r *Root) SaveChapterContent(chapterID, content string) error {
	if strings.TrimSpace(chapterID) == "" {
		return errors.New("chapter id is required")
	}
	return r.Write(r.ChapterFile(chapterID), []byte(content), FileMode)
}

// LoadDraftContent returns drafts/<chapterID>.md, or "" when it is missing.
func (r *Root) LoadDraftContent(chapterID string) string {
	data, err := r.Read(r.DraftFile(chapterID))
	if err != nil {
		return ""
	}
	return string(data)
}

// SaveDraftContent writes drafts/<chapterID>.md.
func (r *Root) SaveDraftContent(chapterID, content string) error {
	if strings.TrimSpace(chapterID) == "" {
		return errors.New("chapter id is required")
	}
	return r.Write(r.DraftFile(chapterID), []byte(content), FileMode)
}

// LoadAllDrafts returns every drafts/*.md keyed by chapter ID.
func (r *Root) LoadAllDrafts() map[string]string {
	drafts := make(map[string]string)
	dir := r.DraftsDir()
	entries, err := r.FS().ReadDir(dir)
	if err != nil {
		return drafts
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if data, err := r.Read(filepath.Join(dir, entry.Name())); err == nil {
			drafts[strings.TrimSuffix(entry.Name(), ".md")] = string(data)
		}
	}
	return drafts
}

// LoadRecap reads story/recaps/<chapterID>.json.
func (r *Root) LoadRecap(chapterID string) (*models.ChapterRecap, error) {
	var recap models.ChapterRecap
	if err := r.ReadJSON(r.RecapFile(chapterID), &recap); err != nil {
		return nil, err
	}
	return &recap, nil
}

// SaveRecap writes story/recaps/<chapterID>.json.
func (r *Root) SaveRecap(recap *models.ChapterRecap) error {
	if recap == nil {
		return errors.New("cannot save nil recap")
	}
	if strings.TrimSpace(recap.ChapterID) == "" {
		return errors.New("recap chapter id is required")
	}
	return r.WriteJSON(r.RecapFile(recap.ChapterID), recap)
}

// ReadDir returns the directory entries at a project path.
func (r *Root) ReadDir(path string) ([]fs.DirEntry, error) {
	return r.FS().ReadDir(path)
}
