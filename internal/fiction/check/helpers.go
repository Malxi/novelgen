package check

import (
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/craft"
	"novelgen/internal/platform/store"
)

func addToolCheckMeta(result *ToolCheckResult, key string, value interface{}) {
	if result == nil || strings.TrimSpace(key) == "" {
		return
	}
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta[key] = value
}

func toolChapterLengthHardMax(targetWords int) int {
	if targetWords <= 0 {
		return 0
	}
	hardMax := int(float64(targetWords) * 1.2)
	if targetWords+250 > hardMax {
		hardMax = targetWords + 250
	}
	return hardMax
}

func firstNStrings(values []string, maxItems int) []string {
	if maxItems > 0 && len(values) > maxItems {
		return values[:maxItems]
	}
	return values
}

// LoadRawCraftMap reads one craft element file as the string-keyed map the
// schema checks work with. Missing files yield an empty map.
func LoadRawCraftMap(root *store.Root, filename string) (map[string]map[string]interface{}, error) {
	kind, ok := store.CraftKindForFile(filename)
	if !ok {
		return nil, fmt.Errorf("unknown craft file %q", filename)
	}
	values, err := root.LoadCraftRaw(kind)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return values, nil
}

// ChapterRepairIssueCategoryFilter widens the default chapter repair category so
// simulation issues are included alongside quality categories.
func ChapterRepairIssueCategoryFilter(issueCategory string) string {
	category := strings.TrimSpace(issueCategory)
	switch NormalizeKey(category) {
	case "", "all", "simulation":
		return categoryOrAll("") + ",simulation"
	default:
		return category
	}
}

func normalizeCraftPatchTarget(target string) string {
	return craft.NormalizeTarget(target)
}

func craftPatchFilename(kind string) (string, error) {
	return craft.Filename(kind)
}

func resolveCraftPatchKey(values map[string]map[string]interface{}, id string) (string, bool) {
	return craft.ResolveKey(values, id)
}

func NormalizeCraftPatchObject(kind, key string, raw map[string]interface{}) (map[string]interface{}, error) {
	return craft.NormalizePatchObject(kind, key, raw)
}

func copyStringInterfaceMap(value map[string]interface{}) map[string]interface{} {
	return craft.CopyMap(value)
}

func loadRawCraftMap(root *store.Root, filename string) (map[string]map[string]interface{}, error) {
	return LoadRawCraftMap(root, filename)
}

func CloneOutline(outline *models.Outline) *models.Outline {
	return models.CloneOutline(outline)
}
