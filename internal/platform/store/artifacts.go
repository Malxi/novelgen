package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/utils"
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
//
// Only a missing file counts as "nothing to merge into". A file that exists but
// cannot be read or parsed is an error: merging into an assumed-empty object
// would silently drop every key the caller did not mention.
func (r *Root) WriteJSONMerging(path string, value any) error {
	// Read-merge-write is not a backend transaction, but serializing it on this
	// Root prevents two in-process callers from silently dropping each other's
	// keys. Cross-process concurrency still needs a backend with CAS/transactions.
	r.mergeMu.Lock()
	defer r.mergeMu.Unlock()

	var existing []byte
	data, err := r.Read(path)
	if err == nil {
		existing = data
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read existing %s: %w", path, err)
	}

	merged, err := utils.MergeTopLevelJSON(existing, value)
	if err != nil {
		return fmt.Errorf("merge %s: %w", path, err)
	}
	return r.Write(path, merged, FileMode)
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
	return r.LoadSetupAt(r.SetupJSON())
}

// LoadSetupAt reads a story setup from an explicit path. It exists for the
// legacy flat story/setup.json that the RPG adapter still has to read.
func (r *Root) LoadSetupAt(path string) (*models.StorySetup, error) {
	data, err := r.Read(path)
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

// SaveOutlineAt writes an outline to an explicit path, normalized and
// atomically. It serves both the canonical outline.json and the compose
// pipeline's per-iteration progress files, which have the same shape.
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
		// A craft file that has not been generated yet is not an error; one that
		// exists but cannot be read is, because the caller is about to treat the
		// elements as "the project has none".
		if errors.Is(err, fs.ErrNotExist) {
			return values, nil
		}
		return nil, err
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
// empty when the chapter has not been written yet; a chapter file that exists
// but cannot be read is reported as an error instead of as "not written".
func (r *Root) LoadChapterContent(chapterID string) (string, string, error) {
	if err := ValidateKeyPart(chapterID); err != nil {
		return "", "", fmt.Errorf("invalid chapter id %q: %w", chapterID, err)
	}
	for _, path := range r.ChapterCandidates(chapterID) {
		data, err := r.Read(path)
		if err == nil {
			return string(data), path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", "", err
		}
	}
	return "", "", nil
}

// SaveChapterContent writes chapters/chapter-<id>.md.
func (r *Root) SaveChapterContent(chapterID, content string) error {
	if err := ValidateKeyPart(chapterID); err != nil {
		return fmt.Errorf("invalid chapter id %q: %w", chapterID, err)
	}
	return r.Write(r.ChapterFile(chapterID), []byte(content), FileMode)
}

// LoadDraftContent returns drafts/<chapterID>.md, or "" when it is missing.
func (r *Root) LoadDraftContent(chapterID string) (string, error) {
	if err := ValidateKeyPart(chapterID); err != nil {
		return "", fmt.Errorf("invalid chapter id %q: %w", chapterID, err)
	}
	data, err := r.Read(r.DraftFile(chapterID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SaveDraftContent writes drafts/<chapterID>.md.
func (r *Root) SaveDraftContent(chapterID, content string) error {
	if err := ValidateKeyPart(chapterID); err != nil {
		return fmt.Errorf("invalid chapter id %q: %w", chapterID, err)
	}
	return r.Write(r.DraftFile(chapterID), []byte(content), FileMode)
}

// LoadAllDrafts returns every drafts/*.md keyed by chapter ID. A project that
// never used the legacy draft pipeline has no drafts directory, which yields an
// empty map rather than an error.
func (r *Root) LoadAllDrafts() (map[string]string, error) {
	drafts := make(map[string]string)
	dir := r.DraftsDir()
	entries, err := r.FS().ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return drafts, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := r.Read(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		drafts[strings.TrimSuffix(entry.Name(), ".md")] = string(data)
	}
	return drafts, nil
}

// LoadRecap reads story/recaps/<chapterID>.json.
func (r *Root) LoadRecap(chapterID string) (*models.ChapterRecap, error) {
	if err := ValidateKeyPart(chapterID); err != nil {
		return nil, fmt.Errorf("invalid recap chapter id %q: %w", chapterID, err)
	}
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
	if err := ValidateKeyPart(recap.ChapterID); err != nil {
		return fmt.Errorf("invalid recap chapter id %q: %w", recap.ChapterID, err)
	}
	return r.WriteJSON(r.RecapFile(recap.ChapterID), recap)
}
