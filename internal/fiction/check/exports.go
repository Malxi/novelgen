package check

import (
	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/store"
)

// Exported seams for the tool-plane adapter and mutation service. These wrap
// private check helpers without moving the CLI-facing command strings into the
// check package.
func ResolveCraftCheckScope(root *store.Root, scope, id string) (string, error) {
	return resolveCraftCheckScope(root, scope, id)
}

func ValidateCraftSchemaObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	validateCraftSchemaObject(gate, scope, key, value)
}

func LoadCraftCharacterNames(root *store.Root) map[string]bool {
	return loadCraftCharacterNames(root)
}

func ValidateCraftConsistencyObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}, knownCharacters map[string]bool) {
	validateCraftConsistencyObject(gate, scope, key, value, knownCharacters)
}

func ValidateCraftQualityObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	validateCraftQualityObject(gate, scope, key, value)
}

func ChapterTargetWords(root *store.Root) int {
	return toolChapterTargetWords(root)
}

func FinalChapterPathForID(root *store.Root, chapterID string) string {
	return toolFinalChapterPathForID(root, chapterID)
}

func RunToolChapterCheckAt(root *store.Root, kind, scope, id string, targetWordsOverride int) (*ToolCheckResult, error) {
	return runToolChapterCheckAt(root, kind, scope, id, targetWordsOverride)
}

func StripToolChapterMarkdownTitle(content, title string) string {
	return stripToolChapterMarkdownTitle(content, title)
}

func ChapterParagraphCount(content string) int {
	return toolChapterParagraphCount(content)
}

func RecapCheckForValue(kind, scope, id string, recapData *models.ChapterRecap, outlineChapter *models.Chapter) *ToolCheckResult {
	return runToolRecapCheckForValue(kind, scope, id, recapData, outlineChapter)
}

func NormalizeRecapPatchIdentity(value *models.ChapterRecap, id string, outlineChapter *models.Chapter, patchBytes []byte) []string {
	return normalizeRecapPatchIdentity(value, id, outlineChapter, patchBytes)
}

func CategoryOrAll(category string) string {
	return categoryOrAll(category)
}

func ToolCheckNextActions(result ToolCheckResult) []ToolNextAction {
	return toolCheckNextActions(result)
}

func ReviewPriorityRank(priority string) (int, bool) {
	return reviewPriorityRank(priority)
}
