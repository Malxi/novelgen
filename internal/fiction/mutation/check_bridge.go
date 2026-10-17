package mutation

import (
	"strings"

	"novelgen/internal/fiction/check"
	"novelgen/internal/platform/utils"
)

// The mutation service consumes the deterministic check engine for dry-run
// validation and post-write result shaping. These aliases keep the moved patch
// implementation readable while the implementation lives in fiction/check.
type (
	ToolCheckResult    = check.ToolCheckResult
	ToolNextAction     = check.ToolNextAction
	ToolProjectContext = check.ProjectContext
)

func NormalizeKey(value string) string {
	return utils.NormalizeKey(value)
}

func addToolCheckMeta(result *ToolCheckResult, key string, value interface{}) {
	if result == nil || strings.TrimSpace(key) == "" {
		return
	}
	if result.Meta == nil {
		result.Meta = map[string]interface{}{}
	}
	result.Meta[key] = value
}

var (
	RunToolOutlineCheckAt                  = check.RunToolOutlineCheckAt
	ToolIssueNavigation                    = check.ToolIssueNavigation
	ApplyToolCheckIssueFilters             = check.ApplyToolCheckIssueFilters
	MakeToolCheckResult                    = check.MakeToolCheckResult
	ToolCheckResultTargetWords             = check.ToolCheckResultTargetWords
	reviewPriorityRank                     = check.ReviewPriorityRank
	categoryOrAll                          = check.CategoryOrAll
	runToolRecapCheckForValue              = check.RecapCheckForValue
	normalizeRecapPatchIdentity            = check.NormalizeRecapPatchIdentity
	validateCraftSchemaObject              = check.ValidateCraftSchemaObject
	loadCraftCharacterNames                = check.LoadCraftCharacterNames
	validateCraftConsistencyObject         = check.ValidateCraftConsistencyObject
	validateCraftQualityObject             = check.ValidateCraftQualityObject
	toolChapterTargetWords                 = check.ChapterTargetWords
	toolFinalChapterPathForID              = check.FinalChapterPathForID
	runToolChapterCheckAt                  = check.RunToolChapterCheckAt
	stripToolChapterMarkdownTitle          = check.StripToolChapterMarkdownTitle
	toolChapterParagraphCount              = check.ChapterParagraphCount
	BuildChapterSimulationSignalDiagnostic = check.BuildChapterSimulationSignalDiagnostic
	RunToolChapterSimulationGateAt         = check.RunToolChapterSimulationGateAt
	RunToolChapterQualityGate              = check.RunToolChapterQualityGate
	FilterToolScopedSuggestions            = check.FilterToolScopedSuggestions
	NormalizeScope                         = check.NormalizeScope
	ToolVolumeIDFromChapterID              = check.ToolVolumeIDFromChapterID
	ToolLooksLikeVolumeID                  = check.ToolLooksLikeVolumeID
	RunToolChapterCheckWithTargetWordsAt   = check.RunToolChapterCheckWithTargetWordsAt
	ToolNarrativeUnitCount                 = check.ToolNarrativeUnitCount
	RunToolSetupCheck                      = check.RunToolSetupCheck
	loadRawCraftMap                        = check.LoadRawCraftMap
)
