// Package agents is the stable facade for the agent pipeline.
//
// The implementation is grouped by concern:
//
//	base/       BaseAgent, execution backends, logging, SDK params, skills
//	prompt/     shared prompt-budget helpers
//	review/     review focus, revision context, review matrix, SDK review DTOs
//	setup/      setup stage
//	compose/    outline stage
//	craft/      world-element stage
//	draft/      legacy draft stage
//	write/      chapter generation and scoring
//	recap/      chapter recap extraction
//	translate/  translation stage
//	chapter/    chapter analysis and chapter-to-DSL conversion
//	pipeline/   multi-stage AI/RPG pipeline
//	shared/     cross-stage DTOs
//
// External callers may keep importing this package; new code that only needs a
// single stage should import that stage package directly.
package agents

import (
	"novelgen/internal/agents/base"
	"novelgen/internal/agents/chapter"
	"novelgen/internal/agents/compose"
	"novelgen/internal/agents/craft"
	"novelgen/internal/agents/draft"
	"novelgen/internal/agents/pipeline"
	"novelgen/internal/agents/prompt"
	"novelgen/internal/agents/recap"
	"novelgen/internal/agents/review"
	"novelgen/internal/agents/setup"
	"novelgen/internal/agents/shared"
	"novelgen/internal/agents/translate"
	"novelgen/internal/agents/write"
)

// Core execution types.
type (
	BaseAgent               = base.BaseAgent
	BaseAgentConfig         = base.BaseAgentConfig
	InvokeParams            = base.InvokeParams
	SkillLoader             = base.SkillLoader
	ToolEvidenceRequirement = base.ToolEvidenceRequirement
)

// Shared prompt and review types.
type (
	ReviewFocus             = review.ReviewFocus
	ReviewMatrixSuggestion  = review.ReviewMatrixSuggestion
	RevisionSession         = review.RevisionSession
	RevisionSuggestionBrief = review.RevisionSuggestionBrief
	RevisionTurn            = review.RevisionTurn
	SDKReviewResult         = review.SDKReviewResult
	SDKReviewSuggestion     = review.SDKReviewSuggestion
)

type NextChapterInfo = shared.NextChapterInfo

// Setup stage.
type (
	SetupAgent                 = setup.SetupAgent
	SetupAgentSDKImproveInput  = setup.SetupAgentSDKImproveInput
	SetupAgentSDKImproveOutput = setup.SetupAgentSDKImproveOutput
	SetupGenInput              = setup.SetupGenInput
	SetupGenOutput             = setup.SetupGenOutput
	SetupImproveInput          = setup.SetupImproveInput
	SetupImproveOutput         = setup.SetupImproveOutput
	SetupReviewInput           = setup.SetupReviewInput
	SetupReviewOutput          = setup.SetupReviewOutput
)

// Compose stage.
type (
	ComposeAgent                       = compose.ComposeAgent
	ComposeAgentSDKImproveVolumeOutput = compose.ComposeAgentSDKImproveVolumeOutput
	ComposeChaptersInput               = compose.ComposeChaptersInput
	ComposeChaptersOutput              = compose.ComposeChaptersOutput
	ComposeGenInput                    = compose.ComposeGenInput
	ComposeGenOutput                   = compose.ComposeGenOutput
	ComposeImproveInput                = compose.ComposeImproveInput
	ComposeImproveOutput               = compose.ComposeImproveOutput
	ComposeImproveReportEntry          = compose.ComposeImproveReportEntry
	ComposeImproveVolumeInput          = compose.ComposeImproveVolumeInput
	ComposeImproveVolumeOutput         = compose.ComposeImproveVolumeOutput
	ComposeOutlineReviewInput          = compose.ComposeOutlineReviewInput
	ComposeRegenInput                  = compose.ComposeRegenInput
	ComposeRegenOutput                 = compose.ComposeRegenOutput
	ComposeReviewInput                 = compose.ComposeReviewInput
	ComposeReviewOutput                = compose.ComposeReviewOutput
	ComposeReviewVolumeInput           = compose.ComposeReviewVolumeInput
	ComposeReviewVolumeOutput          = compose.ComposeReviewVolumeOutput
	ComposeSkeletonImproveInput        = compose.ComposeSkeletonImproveInput
	ComposeSkeletonImproveOutput       = compose.ComposeSkeletonImproveOutput
	ComposeSkeletonInput               = compose.ComposeSkeletonInput
	ComposeSkeletonOutput              = compose.ComposeSkeletonOutput
	ComposeSkeletonReviewInput         = compose.ComposeSkeletonReviewInput
	ComposeSkeletonReviewOutput        = compose.ComposeSkeletonReviewOutput
	ImproveProgress                    = compose.ImproveProgress
)

// Craft stage.
type (
	CraftAgent                         = craft.CraftAgent
	CraftAgentSDKCharactersPromptInput = craft.CraftAgentSDKCharactersPromptInput
	CraftAgentSDKElementsPromptInput   = craft.CraftAgentSDKElementsPromptInput
	CraftChapterSummary                = craft.CraftChapterSummary
	CraftEventSummary                  = craft.CraftEventSummary
	CraftGenCharactersInput            = craft.CraftGenCharactersInput
	CraftGenCharactersOutput           = craft.CraftGenCharactersOutput
	CraftGenItemsInput                 = craft.CraftGenItemsInput
	CraftGenItemsOutput                = craft.CraftGenItemsOutput
	CraftGenLocationsInput             = craft.CraftGenLocationsInput
	CraftGenLocationsOutput            = craft.CraftGenLocationsOutput
	CraftGenOrganizationsInput         = craft.CraftGenOrganizationsInput
	CraftGenOrganizationsOutput        = craft.CraftGenOrganizationsOutput
	CraftImproveCharactersInput        = craft.CraftImproveCharactersInput
	CraftImproveCharactersOutput       = craft.CraftImproveCharactersOutput
	CraftImproveItemsInput             = craft.CraftImproveItemsInput
	CraftImproveItemsOutput            = craft.CraftImproveItemsOutput
	CraftImproveLocationsInput         = craft.CraftImproveLocationsInput
	CraftImproveLocationsOutput        = craft.CraftImproveLocationsOutput
	CraftImproveOrganizationsInput     = craft.CraftImproveOrganizationsInput
	CraftImproveOrganizationsOutput    = craft.CraftImproveOrganizationsOutput
	CraftIterationAgent                = craft.CraftIterationAgent
	CraftOutlineSummary                = craft.CraftOutlineSummary
	CraftPartSummary                   = craft.CraftPartSummary
	CraftReviewCharactersInput         = craft.CraftReviewCharactersInput
	CraftReviewCharactersOutput        = craft.CraftReviewCharactersOutput
	CraftReviewItemsInput              = craft.CraftReviewItemsInput
	CraftReviewItemsOutput             = craft.CraftReviewItemsOutput
	CraftReviewLocationsInput          = craft.CraftReviewLocationsInput
	CraftReviewLocationsOutput         = craft.CraftReviewLocationsOutput
	CraftReviewOrganizationsInput      = craft.CraftReviewOrganizationsInput
	CraftReviewOrganizationsOutput     = craft.CraftReviewOrganizationsOutput
	CraftStorySetupSummary             = craft.CraftStorySetupSummary
	CraftVolumeSummary                 = craft.CraftVolumeSummary
)

// Draft stage.
type (
	DraftAgent            = draft.DraftAgent
	DraftGenInput         = draft.DraftGenInput
	DraftGenOutput        = draft.DraftGenOutput
	DraftImproveInput     = draft.DraftImproveInput
	DraftImproveOutput    = draft.DraftImproveOutput
	DraftReview           = draft.DraftReview
	DraftReviewInput      = draft.DraftReviewInput
	DraftReviewOutput     = draft.DraftReviewOutput
	GateReview            = draft.GateReview
	CharacterReview       = draft.CharacterReview
	PacingReview          = draft.PacingReview
	PlotCoherenceReview   = draft.PlotCoherenceReview
	PresenceReview        = draft.PresenceReview
	RationalityReview     = draft.RationalityReview
	SceneContinuityReview = draft.SceneContinuityReview
	VolumeReview          = draft.VolumeReview
)

// Write stage.
type (
	WriteAgent               = write.WriteAgent
	ChapterContext           = write.ChapterContext
	ChapterScoreInput        = write.ChapterScoreInput
	ChapterScoreOutput       = write.ChapterScoreOutput
	ChapterScorerAgent       = write.ChapterScorerAgent
	CompactStorySetup        = write.CompactStorySetup
	ContextChapter           = write.ContextChapter
	ScoredChapterCopy        = write.ScoredChapterCopy
	VolumeChapterReview      = write.VolumeChapterReview
	VolumeReviewChapterInput = write.VolumeReviewChapterInput
	VolumeReviewInput        = write.VolumeReviewInput
	VolumeReviewOutput       = write.VolumeReviewOutput
	VolumeReviewResult       = write.VolumeReviewResult
	VolumeStructureAnalysis  = write.VolumeStructureAnalysis
	WriteGenInput            = write.WriteGenInput
	WriteGenOutput           = write.WriteGenOutput
	WriteImproveInput        = write.WriteImproveInput
	WriteImproveOutput       = write.WriteImproveOutput
	WriteReviewInput         = write.WriteReviewInput
	WriteReviewOutput        = write.WriteReviewOutput
)

// Recap and translation stages.
type (
	RecapAgent              = recap.RecapAgent
	RecapExtractInput       = recap.RecapExtractInput
	RecapExtractOutput      = recap.RecapExtractOutput
	TranslateAgent          = translate.TranslateAgent
	TranslateAgentSDKInput  = translate.TranslateAgentSDKInput
	TranslateAgentSDKOutput = translate.TranslateAgentSDKOutput
)

// Chapter analysis and DSL conversion.
type (
	ChapterAnalysisAgent        = chapter.ChapterAnalysisAgent
	ChapterAnalysisInput        = chapter.ChapterAnalysisInput
	ChapterAnalysisOutput       = chapter.ChapterAnalysisOutput
	ChapterData                 = chapter.ChapterData
	ChapterIssue                = chapter.ChapterIssue
	ChapterRecap                = chapter.ChapterRecap
	ChapterSummary              = chapter.ChapterSummary
	ChapterToDSLAgent           = chapter.ChapterToDSLAgent
	ChapterToDSLInput           = chapter.ChapterToDSLInput
	ChapterToDSLOutput          = chapter.ChapterToDSLOutput
	ChapterToDSLProgressionHint = chapter.ChapterToDSLProgressionHint
	ChapterToDSLRepairInput     = chapter.ChapterToDSLRepairInput
	ChapterToDSLResourceHint    = chapter.ChapterToDSLResourceHint
	ChapterToDSLSetupHints      = chapter.ChapterToDSLSetupHints
	CharacterInfo               = chapter.CharacterInfo
	EventInfo                   = chapter.EventInfo
	NovelgenProjectAdapter      = chapter.NovelgenProjectAdapter
	PowerChangeInfo             = chapter.PowerChangeInfo
	TimelineInfo                = chapter.TimelineInfo
)

// Multi-stage pipeline.
type (
	AIRPGPipeline   = pipeline.AIRPGPipeline
	ChapterInput    = pipeline.ChapterInput
	PipelineResult  = pipeline.PipelineResult
	ValidationIssue = pipeline.ValidationIssue
)

const (
	AgentLogsDisabledEnv = base.AgentLogsDisabledEnv
	SkillsDirEnv         = base.SkillsDirEnv
)

var ErrAgentSDKChapterTooLong = write.ErrAgentSDKChapterTooLong

var (
	NewBaseAgent                     = base.NewBaseAgent
	NewSkillLoader                   = base.NewSkillLoader
	AgentSDKLogToolAllowlist         = base.AgentSDKLogToolAllowlist
	AgentSDKNonASCIIPatchRule        = base.AgentSDKNonASCIIPatchRule
	AgentSDKOnlyAllowedWriteClause   = base.AgentSDKOnlyAllowedWriteClause
	AgentSDKOnlyAllowedWriteSentence = base.AgentSDKOnlyAllowedWriteSentence
	AgentSDKToolParams               = base.AgentSDKToolParams
	DedupeToolAllowlist              = base.DedupeToolAllowlist
	LiveSummaryHasToolCommand        = base.LiveSummaryHasToolCommand

	ClipForPrompt          = prompt.ClipForPrompt
	ClipReviewStringList   = prompt.ClipReviewStringList
	ClipReviewSuggestion   = prompt.ClipReviewSuggestion
	CompactReviewForPrompt = prompt.CompactReviewForPrompt
	NonEmpty               = prompt.NonEmpty

	AnnotateMatrixSuggestions = review.AnnotateMatrixSuggestions
	BuildRationalizePrompt    = review.BuildRationalizePrompt
	ClassifyReviewThemes      = review.ClassifyReviewThemes
	ListReviewFocusNames      = review.ListReviewFocusNames
	NewRevisionSession        = review.NewRevisionSession
	ResolveReviewFocusPrompt  = review.ResolveReviewFocusPrompt
	StratifiedSample          = review.StratifiedSample

	NewSetupAgent                        = setup.NewSetupAgent
	BuildSetupAgentSDKImprovePromptInput = setup.BuildSetupAgentSDKImprovePromptInput
	NewComposeAgent                      = compose.NewComposeAgent
	AgentSDKImproveBoundaryChapterIDs    = compose.AgentSDKImproveBoundaryChapterIDs
	NewCraftAgent                        = craft.NewCraftAgent
	NewCraftIterationAgent               = craft.NewCraftIterationAgent
	NewDraftAgent                        = draft.NewDraftAgent
	NewWriteAgent                        = write.NewWriteAgent
	NewChapterScorerAgent                = write.NewChapterScorerAgent
	IsAgentSDKChapterTooLong             = write.IsAgentSDKChapterTooLong
	SelectBestScoredChapter              = write.SelectBestScoredChapter
	ToCompact                            = write.ToCompact
	NewRecapAgent                        = recap.NewRecapAgent
	NewTranslateAgent                    = translate.NewTranslateAgent
	NewChapterAnalysisAgent              = chapter.NewChapterAnalysisAgent
	NewChapterToDSLAgent                 = chapter.NewChapterToDSLAgent
	LoadNovelgenProject                  = chapter.LoadNovelgenProject
	LoadNovelgenProjectAt                = chapter.LoadNovelgenProjectAt
	NewNovelgenProjectAdapter            = chapter.NewNovelgenProjectAdapter
	NewAIRPGPipeline                     = pipeline.NewAIRPGPipeline
)
