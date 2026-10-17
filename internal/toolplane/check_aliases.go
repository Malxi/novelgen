package toolplane

import "novelgen/internal/fiction/check"

// Type aliases keep the existing `novelgen tool ...` JSON surface and the
// in-process callers source-compatible while the implementations live in the
// fiction/check engine.
type (
	ToolProjectContext = check.ProjectContext
	ToolCheckResult    = check.ToolCheckResult
	ToolCheckSummary   = check.ToolCheckSummary
	ToolNextAction     = check.ToolNextAction
)

var (
	MakeToolCheckResult                    = check.MakeToolCheckResult
	ApplyToolCheckIssueFilters             = check.ApplyToolCheckIssueFilters
	ToolIssueNavigation                    = check.ToolIssueNavigation
	ToolCheckResultTargetWords             = check.ToolCheckResultTargetWords
	ToolVolumeIDFromChapterID              = check.ToolVolumeIDFromChapterID
	ToolLooksLikeVolumeID                  = check.ToolLooksLikeVolumeID
	RunToolSetupCheck                      = check.RunToolSetupCheck
	RunToolOutlineCheck                    = check.RunToolOutlineCheck
	RunToolOutlineCheckAt                  = check.RunToolOutlineCheckAt
	RunToolCraftSchemaCheck                = check.RunToolCraftSchemaCheck
	RunToolCraftSchemaCheckAt              = check.RunToolCraftSchemaCheckAt
	RunToolRecapCheck                      = check.RunToolRecapCheck
	RunToolRecapCheckAt                    = check.RunToolRecapCheckAt
	RunToolChapterCheckWithTargetWords     = check.RunToolChapterCheckWithTargetWords
	RunToolChapterCheckWithTargetWordsAt   = check.RunToolChapterCheckWithTargetWordsAt
	RunToolChapterQualityGate              = check.RunToolChapterQualityGate
	RunToolChapterSimulationGate           = check.RunToolChapterSimulationGate
	RunToolChapterSimulationGateAt         = check.RunToolChapterSimulationGateAt
	BuildChapterSimulationSignalDiagnostic = check.BuildChapterSimulationSignalDiagnostic
	ToolNarrativeUnitCount                 = check.ToolNarrativeUnitCount
	NormalizeScope                         = check.NormalizeScope
	FilterToolScopedSuggestions            = check.FilterToolScopedSuggestions
	resolveCraftCheckScope                 = check.ResolveCraftCheckScope
	validateCraftSchemaObject              = check.ValidateCraftSchemaObject
	loadCraftCharacterNames                = check.LoadCraftCharacterNames
	validateCraftConsistencyObject         = check.ValidateCraftConsistencyObject
	validateCraftQualityObject             = check.ValidateCraftQualityObject
	toolChapterTargetWords                 = check.ChapterTargetWords
	toolFinalChapterPathForID              = check.FinalChapterPathForID
	runToolChapterCheckAt                  = check.RunToolChapterCheckAt
	stripToolChapterMarkdownTitle          = check.StripToolChapterMarkdownTitle
	toolChapterParagraphCount              = check.ChapterParagraphCount
	runToolRecapCheckForValue              = check.RecapCheckForValue
	runToolRecapCheckAt                    = check.RunToolRecapCheckAt
	normalizeRecapPatchIdentity            = check.NormalizeRecapPatchIdentity
	categoryOrAll                          = check.CategoryOrAll
	toolCheckNextActions                   = check.ToolCheckNextActions
	reviewPriorityRank                     = check.ReviewPriorityRank
)

func normalizeToolScope(scope string) string {
	return check.NormalizeScope(scope)
}
