package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"novelgen/internal/agentruntime"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic"
	"novelgen/internal/models"
	"novelgen/internal/utils"
)

// ComposeGenInput is the input for outline generation
type ComposeGenInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeGenOutput is the output for outline generation
type ComposeGenOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Generated complete story outline with parts, volumes, chapters"`
}

// ComposeRegenInput is the input for outline regeneration
type ComposeRegenInput struct {
	Outline     models.Outline `json:"outline" md:"outline" desc:"Existing outline to regenerate from"`
	ElementType string         `json:"element_type" md:"element_type" desc:"Type of element to regenerate: part, volume, or chapter"`
	ElementID   string         `json:"element_id" md:"element_id" desc:"ID of the element to regenerate"`
	Suggestions string         `json:"suggestions" md:"suggestions" desc:"User suggestions for regeneration"`
	Context     string         `json:"context,omitempty" md:"context,omitempty" desc:"Surrounding context for continuity"`
}

// ComposeRegenOutput is the output for outline regeneration
type ComposeRegenOutput struct {
	Part    *models.Part    `json:"part,omitempty" md:"part,omitempty" desc:"Regenerated part (if element_type is part)"`
	Volume  *models.Volume  `json:"volume,omitempty" md:"volume,omitempty" desc:"Regenerated volume (if element_type is volume)"`
	Chapter *models.Chapter `json:"chapter,omitempty" md:"chapter,omitempty" desc:"Regenerated chapter (if element_type is chapter)"`
}

// ComposeReviewInput is the input for outline review
type ComposeReviewInput struct {
	ExistingOutline models.Outline    `json:"existing_outline" md:"existing_outline" desc:"Existing outline to review"`
	Setup           models.StorySetup `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	UserPrompt      string            `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

// ComposeReviewOutput is the output for outline review
type ComposeReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveInput is the input for outline improvement
type ComposeImproveInput struct {
	ExistingOutline models.Outline      `json:"existing_outline" md:"existing_outline" desc:"Existing outline to improve"`
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup           models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	RevisionContext string              `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier outline review and improve rounds"`
}

// ComposeImproveOutput is the output for outline improvement
type ComposeImproveOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Improved story outline"`
}

// ComposeSkeletonInput is the input for generating outline skeleton (parts and volumes only)
type ComposeSkeletonInput struct {
	Setup     models.StorySetup     `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

// ComposeSkeletonOutput is the output for outline skeleton generation
type ComposeSkeletonOutput struct {
	Parts []models.Part `json:"parts" md:"parts" desc:"Generated story parts with volumes"`
}

// ComposeSkeletonReviewInput is the input for reviewing an outline skeleton
// before chapter-level generation. It intentionally focuses on parts, volumes,
// summaries, payoff contracts, and long-form escalation rather than chapter
// completeness.
type ComposeSkeletonReviewInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to review"`
	Setup           models.StorySetup     `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

// ComposeSkeletonReviewOutput is the output for outline skeleton review.
type ComposeSkeletonReviewOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeSkeletonImproveInput is the input for improving an outline skeleton.
type ComposeSkeletonImproveInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to improve"`
	ReviewResult    models.ReviewResult   `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup           models.StorySetup     `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	RevisionContext string                `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier skeleton review and improve rounds"`
}

// ComposeSkeletonImproveOutput is the output for outline skeleton improvement.
type ComposeSkeletonImproveOutput struct {
	Outline models.Outline `json:"outline" md:"outline" desc:"Improved outline skeleton"`
}

// ComposeChaptersInput is the input for generating chapters for a volume
type ComposeChaptersInput struct {
	Setup          models.StorySetup `json:"setup" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	Part           models.Part       `json:"part" md:"part" desc:"Current part information"`
	Volume         models.Volume     `json:"volume" md:"volume" desc:"Current volume information"`
	VolumeIndex    int               `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes   int               `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
	ChaptersPerVol int               `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Number of chapters per volume"`
	PreviousVolume *models.Volume    `json:"previous_volume,omitempty" md:"previous_volume,omitempty" desc:"Previous volume for continuity"`
	OutlineContext string            `json:"outline_context" md:"outline_context" desc:"Context from previously generated outline"`
}

// ComposeChaptersOutput is the output for chapter generation
type ComposeChaptersOutput struct {
	Chapters []models.Chapter `json:"chapters" md:"chapters" desc:"Generated chapters for the volume"`
}

// Compact compose prompt payloads keep large story setup files from crowding
// out the outline task. Public inputs stay rich for CLI/internal contracts.
type composeGenPromptInput struct {
	SetupBrief string                `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, rules, progression limits, resources, core cast seeds, storylines"`
	Structure  models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target chapters and volumes"`
}

type composeReviewPromptInput struct {
	ExistingOutline models.Outline `json:"existing_outline" md:"existing_outline" desc:"Existing outline to review"`
	SetupBrief      string         `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for checking outline alignment"`
	UserPrompt      string         `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

type composeImprovePromptInput struct {
	ExistingOutline models.Outline      `json:"existing_outline" md:"existing_outline" desc:"Existing outline to improve"`
	ReviewResult    models.ReviewResult `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	SetupBrief      string              `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for preserving setup promises"`
	RevisionContext string              `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier outline review and improve rounds"`
}

type composeSkeletonPromptInput struct {
	SetupBrief string                `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, long-form plan, core cast seeds, storylines, progression systems"`
	Structure  models.StoryStructure `json:"structure" md:"structure" desc:"Story structure including target parts and volumes"`
}

type composeSkeletonReviewPromptInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to review"`
	SetupBrief      string                `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for checking skeleton alignment"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for review focus"`
}

type composeSkeletonImprovePromptInput struct {
	ExistingOutline models.Outline        `json:"existing_outline" md:"existing_outline" desc:"Existing outline skeleton to improve"`
	ReviewResult    models.ReviewResult   `json:"review_result,omitempty" md:"review_result,omitempty" desc:"Review result for improvement guidance"`
	UserPrompt      string                `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	SetupBrief      string                `json:"setup_brief,omitempty" md:"setup_brief" desc:"Compact story contract for preserving setup promises"`
	Structure       models.StoryStructure `json:"structure,omitempty" md:"structure" desc:"Expected project structure"`
	RevisionContext string                `json:"revision_context,omitempty" md:"revision_context,omitempty" desc:"Compact session trail from earlier skeleton review and improve rounds"`
}

type composeChaptersPromptInput struct {
	SetupBrief     string         `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: rules, resources, core cast seeds, long-form plan, storylines"`
	Part           models.Part    `json:"part" md:"part" desc:"Current part information"`
	Volume         models.Volume  `json:"volume" md:"volume" desc:"Current volume information"`
	VolumeIndex    int            `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes   int            `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
	ChaptersPerVol int            `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Number of chapters to generate in this call"`
	PreviousVolume *models.Volume `json:"previous_volume,omitempty" md:"previous_volume,omitempty" desc:"Previous volume for continuity"`
	OutlineContext string         `json:"outline_context" md:"outline_context" desc:"Context from previously generated outline"`
}

type composeAgentSDKSkeletonPromptInput struct {
	Structure       models.StoryStructure `json:"structure" md:"structure" desc:"Expected parts, volumes, and chapters per volume"`
	RequiredQueries []string              `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions    []string              `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type composeAgentSDKChaptersPromptInput struct {
	TargetPartID     string   `json:"target_part_id" md:"target_part_id" desc:"Part ID containing the target volume"`
	TargetVolumeID   string   `json:"target_volume_id" md:"target_volume_id" desc:"Volume ID to generate"`
	TargetVolumeName string   `json:"target_volume_name" md:"target_volume_name" desc:"Current target volume title for display only"`
	VolumeIndex      int      `json:"volume_index" md:"volume_index" desc:"1-based global volume index"`
	TotalVolumes     int      `json:"total_volumes" md:"total_volumes" desc:"Total volume count"`
	ChaptersPerVol   int      `json:"chapters_per_volume" md:"chapters_per_volume" desc:"Exact number of chapters to return"`
	PreviousVolumeID string   `json:"previous_volume_id,omitempty" md:"previous_volume_id" desc:"Previous volume ID for continuity queries"`
	RequiredQueries  []string `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

// ComposeReviewVolumeInput is the input for reviewing a specific volume
type ComposeReviewVolumeInput struct {
	Outline      models.Outline `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part    `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume  `json:"volume" md:"volume" desc:"Volume to review"`
	VolumeIndex  int            `json:"volume_index" md:"volume_index" desc:"Index of current volume"`
	TotalVolumes int            `json:"total_volumes" md:"total_volumes" desc:"Total number of volumes"`
}

// ComposeReviewVolumeOutput is the output for volume review
type ComposeReviewVolumeOutput struct {
	Result models.ReviewResult `json:"result" md:"result" desc:"Review result with scores and suggestions"`
}

// ComposeImproveVolumeInput is the input for improving a specific volume
type ComposeImproveVolumeInput struct {
	Outline      models.Outline      `json:"outline" md:"outline" desc:"Complete story outline"`
	Part         models.Part         `json:"part" md:"part" desc:"Current part information"`
	Volume       models.Volume       `json:"volume" md:"volume" desc:"Volume to improve"`
	ReviewResult models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result for improvement guidance"`
	UserPrompt   string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	Setup        models.StorySetup   `json:"setup,omitempty" md:"setup" desc:"Story setup including premise, genres, themes, rules"`
	// CrossVolumeIDs, when non-empty, switches this session into cross-volume mode:
	// the agent may query/check/patch ALL of the listed volumes in one session,
	// so cross-volume issues (e.g. volume-end hook + next-volume payoff) can be
	// fixed consistently instead of per-volume in isolation.
	CrossVolumeIDs []string `json:"cross_volume_ids,omitempty" md:"cross_volume_ids" desc:"Optional list of additional volume IDs this session may patch, enabling cross-volume improvement in one session"`
}

// composeImproveVolumePromptInput is the compact prompt payload sent to the LLM.
// The public input above can stay rich for internal routing/checkpoint logic;
// this shape avoids repeating the full outline, part, volume, and setup in one prompt.
type composeImproveVolumePromptInput struct {
	SetupBrief     string              `json:"setup_brief" md:"setup_brief" desc:"Compact story contract: premise, rules, progression limits, resources, storylines"`
	OutlineContext string              `json:"outline_context" md:"outline_context" desc:"Compact continuity context around the target volume"`
	TargetVolume   models.Volume       `json:"target_volume" md:"target_volume" desc:"Only this volume should be improved and returned"`
	ReviewResult   models.ReviewResult `json:"review_result" md:"review_result" desc:"Filtered review/DSL feedback relevant to this volume"`
	UserPrompt     string              `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
}

type composeAgentSDKImproveVolumePromptInput struct {
	TargetPartID     string                            `json:"target_part_id" md:"target_part_id" desc:"Part ID containing the target volume"`
	TargetVolumeID   string                            `json:"target_volume_id" md:"target_volume_id" desc:"Only this volume may be reviewed and returned"`
	TargetVolumeName string                            `json:"target_volume_name" md:"target_volume_name" desc:"Current target volume title for display only"`
	ChapterCount     int                               `json:"chapter_count" md:"chapter_count" desc:"Exact number of chapters that volume.chapters must contain"`
	ReviewResult     composeAgentSDKPromptReviewResult `json:"review_result,omitempty" md:"review_result" desc:"Optional quality gate or user review feedback relevant to this volume, including focused navigation queries"`
	UserPrompt       string                            `json:"user_prompt,omitempty" md:"user_prompt" desc:"Additional user suggestions for improvement"`
	ApplyPatches     bool                              `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use tool patch outline --apply after a successful dry-run"`
	ForceIssueRepair bool                              `json:"force_issue_repair" md:"force_issue_repair" desc:"When true, focused review_result suggestions are an explicit repair task list, including directly fixable low-priority issues"`
	RequiredQueries  []string                          `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string                          `json:"instructions" md:"instructions" desc:"Workflow constraints for the SDK agent"`
}

type composeAgentSDKGlobalRepairPromptInput struct {
	ReviewResult     composeAgentSDKPromptReviewResult `json:"review_result" md:"review_result" desc:"Global outline check issues, including navigation when available"`
	ApplyPatches     bool                              `json:"apply_patches" md:"apply_patches" desc:"Whether the workflow may use validated setup/outline patch --apply"`
	ForceIssueRepair bool                              `json:"force_issue_repair" md:"force_issue_repair" desc:"Global issues are explicit repair tasks when they have patch_query and patch_shape"`
	RequiredQueries  []string                          `json:"required_queries" md:"required_queries" desc:"Read-only novelgen tool query commands that must be used before answering"`
	Instructions     []string                          `json:"instructions" md:"instructions" desc:"Workflow constraints for global repair"`
}

type composeAgentSDKPromptReviewResult struct {
	OverallScore float64                                 `json:"overall_score,omitempty" md:"overall_score" desc:"0-100 score from deterministic checks or prior review"`
	Summary      string                                  `json:"summary,omitempty" md:"summary" desc:"Compact review/check summary"`
	Suggestions  []composeAgentSDKPromptReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"Focused issues for this target volume"`
}

type composeAgentSDKPromptReviewSuggestion struct {
	Category   string                               `json:"category,omitempty" md:"category" desc:"Issue category such as logic, structure, state_anchor, resource_ledger"`
	TargetID   string                               `json:"target_id,omitempty" md:"target_id" desc:"Target volume or chapter ID"`
	TargetName string                               `json:"target_name,omitempty" md:"target_name" desc:"Short target display name"`
	Issue      string                               `json:"issue" md:"issue" desc:"Short issue description"`
	Suggestion string                               `json:"suggestion,omitempty" md:"suggestion" desc:"Short suggested fix"`
	Priority   string                               `json:"priority" md:"priority" desc:"critical, high, medium, or low"`
	Navigation *composeAgentSDKSuggestionNavigation `json:"navigation,omitempty" md:"navigation" desc:"Focused commands to gather the smallest useful context for this issue"`
}

type composeAgentSDKSuggestionNavigation struct {
	TargetKind         string                 `json:"target_kind,omitempty" md:"target_kind" desc:"chapter, volume, or outline"`
	VolumeID           string                 `json:"volume_id,omitempty" md:"volume_id" desc:"Volume ID to patch when repairing this issue"`
	DetailQueries      []string               `json:"detail_queries,omitempty" md:"detail_queries" desc:"Read-only queries to inspect before broadening context"`
	FocusedCheckQuery  string                 `json:"focused_check_query,omitempty" md:"focused_check_query" desc:"Smallest useful check command for this issue"`
	RepairRouteQuery   string                 `json:"repair_route_query,omitempty" md:"repair_route_query" desc:"Index-sized route query with next_actions for this issue"`
	RepairContextQuery string                 `json:"repair_context_query,omitempty" md:"repair_context_query" desc:"Smallest useful context bundle for this issue"`
	PatchQuery         string                 `json:"patch_query,omitempty" md:"patch_query" desc:"Patch command prefix for dry-run/apply"`
	PatchShape         map[string]interface{} `json:"patch_shape,omitempty" md:"patch_shape" desc:"Minimal JSON patch shape hint"`
}

// ComposeImproveVolumeOutput is the output for volume improvement
type ComposeImproveVolumeOutput struct {
	Volume models.Volume `json:"volume" md:"volume" desc:"Improved volume with chapters"`
}

type ComposeAgentSDKImproveVolumeOutput struct {
	ReviewResult models.ReviewResult `json:"review_result" md:"review_result" desc:"Review result for the target volume"`
	Volume       models.Volume       `json:"volume" md:"volume" desc:"Improved target volume with chapters"`
}

// ComposeOutlineReviewInput is the input for an Agent SDK outline review.
// VolumeID is optional; empty means the whole outline.
type ComposeOutlineReviewInput struct {
	Outline    models.Outline
	Setup      models.StorySetup
	UserPrompt string
	VolumeID   string
	// CrossVolume 为 true 时按跨卷模式审查 (范围: FromVolumeIndex..ToVolumeIndex, 1-based 全局索引)。
	CrossVolume     bool
	FromVolumeIndex int
	ToVolumeIndex   int
	// SurfaceOnly 为 true 时只暴露读者可见字段 (title/summary/opening_beat/scenes),
	// 不暴露 events/payoff_contract/state_anchor 等内部结构字段——用于审"读者读到的",
	// 而非"作者的面板"(后者常被误判为 AI 味)。
	SurfaceOnly bool
}

type composeAgentSDKOutlineReviewPromptInput struct {
	TargetVolumeID  string   `json:"target_volume_id,omitempty" md:"target_volume_id" desc:"Optional volume scope to review; empty means the whole outline"`
	VolumeCount     int      `json:"volume_count,omitempty" md:"volume_count" desc:"Total number of volumes in scope"`
	ChapterCount    int      `json:"chapter_count,omitempty" md:"chapter_count" desc:"Total number of chapters in scope"`
	UserPrompt      string   `json:"user_prompt,omitempty" md:"user_prompt" desc:"Optional user review focus; the highest-priority review task when present"`
	ReviewFocus     string   `json:"review_focus,omitempty" md:"review_focus" desc:"Review dimensions to cover when no user prompt is given"`
	RequiredQueries []string `json:"required_queries" md:"required_queries" desc:"Exact tool commands to run first, in order"`
	Instructions    []string `json:"instructions,omitempty" md:"instructions" desc:"Workflow rules for this review run"`
}

type composeAgentSDKImprovePatchOutput struct {
	ReviewResult composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for the target volume. Keep it short and focused on applied or remaining issues."`
	VolumePatch  composeVolumePatch          `json:"volume_patch" md:"volume_patch" desc:"Patch for the target volume. Return only changed volume fields and complete changed chapters; Go will merge it into the original volume."`
}

type composeAgentSDKApplyOutput struct {
	ReviewResult      composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for the target volume after any applied patches"`
	AppliedPatches    bool                        `json:"applied_patches" md:"applied_patches" desc:"True only if the workflow successfully used tool patch outline --apply"`
	AppliedPatchCount int                         `json:"applied_patch_count,omitempty" md:"applied_patch_count" desc:"Number of successful apply calls, if any"`
	FinalCheck        string                      `json:"final_check,omitempty" md:"final_check" desc:"Compact final tool check summary, or why no patch/check was needed"`
}

type composeAgentSDKGlobalRepairOutput struct {
	ReviewResult      composeAgentSDKReviewResult `json:"review_result" md:"review_result" desc:"Compact review result for global outline issues after any applied patches"`
	AppliedPatches    bool                        `json:"applied_patches" md:"applied_patches" desc:"True only if the workflow successfully used a validated tool patch --apply"`
	AppliedPatchCount int                         `json:"applied_patch_count,omitempty" md:"applied_patch_count" desc:"Number of successful apply calls, if any"`
	FinalCheck        string                      `json:"final_check,omitempty" md:"final_check" desc:"Compact final tool check summary, or why no patch/check was needed"`
}

type composeAgentSDKReviewResult struct {
	OverallScore float64                           `json:"overall_score" md:"overall_score" desc:"0-100 score for the target volume after the returned patch"`
	Summary      string                            `json:"summary" md:"summary" desc:"Compact Chinese summary under 500 characters"`
	Suggestions  []composeAgentSDKReviewSuggestion `json:"suggestions,omitempty" md:"suggestions" desc:"At most 8 remaining issues or applied patch notes"`
}

type composeAgentSDKReviewSuggestion struct {
	Category   string `json:"category,omitempty" md:"category" desc:"Issue category such as logic, structure, state_anchor, resource_ledger"`
	TargetID   string `json:"target_id,omitempty" md:"target_id" desc:"Target volume or chapter ID"`
	TargetName string `json:"target_name,omitempty" md:"target_name" desc:"Short target display name"`
	Issue      string `json:"issue" md:"issue" desc:"Short issue or applied-change description"`
	Suggestion string `json:"suggestion,omitempty" md:"suggestion" desc:"Short fix, or why no patch is needed"`
	Priority   string `json:"priority" md:"priority" desc:"critical, high, medium, or low"`
}

type composeVolumePatch struct {
	ID             string                       `json:"id,omitempty" md:"id" desc:"Target volume ID. Must match target_volume_id when present"`
	Title          string                       `json:"title,omitempty" md:"title" desc:"New volume title only if it changes"`
	Summary        string                       `json:"summary,omitempty" md:"summary" desc:"New volume summary only if it changes"`
	PayoffContract *models.VolumePayoffContract `json:"payoff_contract,omitempty" md:"payoff_contract" desc:"New payoff contract only if it changes"`
	Chapters       []composeChapterPatch        `json:"changed_chapters,omitempty" md:"changed_chapters" desc:"Minimal patch objects for changed chapters only. Keep existing chapter IDs and omit unchanged chapters; Go overlays non-empty fields onto the original chapter."`
	Events         []composeChangedEventPatch   `json:"changed_events,omitempty" md:"changed_events" desc:"Minimal patch objects for changed events only. Use chapter_id plus 0-based event_index from tool query outline --type events."`
}

type composeChangedEventPatch struct {
	ChapterID  string   `json:"chapter_id" md:"chapter_id" desc:"Existing chapter ID that owns this event"`
	EventIndex int      `json:"event_index" md:"event_index" desc:"0-based event index returned by tool query outline --type events"`
	Type       string   `json:"type,omitempty" md:"type" desc:"New event type only if it changes"`
	Characters []string `json:"characters,omitempty" md:"characters" desc:"Replacement character list only if it changes"`
	Subject    string   `json:"subject,omitempty" md:"subject" desc:"Changed entity or state type"`
	Change     string   `json:"change,omitempty" md:"change" desc:"Change verb such as started, progressed, completed, get, lost, resolved"`
	Details    string   `json:"details,omitempty" md:"details" desc:"Short extra detail"`
	Actor      string   `json:"actor,omitempty" md:"actor" desc:"Actor performing the action"`
	Action     string   `json:"action,omitempty" md:"action" desc:"Action verb such as acquire, use, lose, move, combat, learn, discover"`
	Target     string   `json:"target,omitempty" md:"target" desc:"Action target"`
	TargetType string   `json:"target_type,omitempty" md:"target_type" desc:"item, character, location, skill, status, relationship, or knowledge"`
	Context    string   `json:"context,omitempty" md:"context" desc:"Where or why it happens"`
	Result     string   `json:"result,omitempty" md:"result" desc:"Narrative result"`
}

type composeChapterPatch struct {
	ID                string                       `json:"id" md:"id" desc:"Existing target chapter ID"`
	Title             string                       `json:"title,omitempty" md:"title" desc:"New chapter title only if it changes"`
	Summary           string                       `json:"summary,omitempty" md:"summary" desc:"New chapter summary only if it changes"`
	Characters        []string                     `json:"characters,omitempty" md:"characters" desc:"Replacement character list only if it changes"`
	Location          string                       `json:"location,omitempty" md:"location" desc:"New chapter location only if it changes"`
	Events            []composeEventPatch          `json:"events,omitempty" md:"events" desc:"Replacement event list only if events change"`
	Scenes            []models.OutlineScene        `json:"scenes,omitempty" md:"scenes" desc:"Scenes you change, matched to the original chapter by scene order. Scenes not included in this list are preserved; keep existing scene orders and indexes, and query scenes via tool query outline --type chapter --fields scenes."`
	OpeningBeat       string                       `json:"opening_beat,omitempty" md:"opening_beat" desc:"New chapter opening beat only if it changes; transition checks read this through normalized beats"`
	ClosingBeat       string                       `json:"closing_beat,omitempty" md:"closing_beat" desc:"New chapter closing beat only if it changes"`
	StateChange       string                       `json:"state_change,omitempty" md:"state_change" desc:"Primary state change only if it changes"`
	Conflict          string                       `json:"conflict,omitempty" md:"conflict" desc:"Conflict text only if it changes"`
	Pacing            string                       `json:"pacing,omitempty" md:"pacing" desc:"Pacing text only if it changes"`
	Timeline          models.ChapterTimeline       `json:"timeline,omitempty" md:"timeline" desc:"Replacement timeline only if timeline changes"`
	StateAnchor       models.StateAnchor           `json:"state_anchor,omitempty" md:"state_anchor" desc:"Replacement state anchor only if it changes"`
	Enemies           []models.OutlineEnemy        `json:"enemies,omitempty" md:"enemies" desc:"Replacement enemy list only if it changes"`
	ResourceLedger    []models.ResourceLedgerEntry `json:"resource_ledger,omitempty" md:"resource_ledger" desc:"Replacement resource ledger only if it changes"`
	Mysteries         models.ChapterMysteries      `json:"mysteries,omitempty" md:"mysteries" desc:"Mystery updates only if planted or resolved mysteries change"`
	StorylineAdvances []models.StorylineAdvance    `json:"storyline_advances,omitempty" md:"storyline_advances" desc:"Replacement storyline advances only if they change"`
	ChapterPayoff     *models.ChapterPayoff        `json:"chapter_payoff,omitempty" md:"chapter_payoff" desc:"Replacement chapter payoff only if it changes"`
}

type composeEventPatch struct {
	Type       string   `json:"type" md:"type" desc:"relationship, goal, item, premise, storyline, gate, or status"`
	Characters []string `json:"characters,omitempty" md:"characters" desc:"Characters involved in the event"`
	Subject    string   `json:"subject" md:"subject" desc:"Changed entity or state type"`
	Change     string   `json:"change" md:"change" desc:"Change verb such as started, progressed, completed, get, lost, resolved"`
	Details    string   `json:"details,omitempty" md:"details" desc:"Short extra detail"`
	Actor      string   `json:"actor,omitempty" md:"actor" desc:"Actor performing the action"`
	Action     string   `json:"action,omitempty" md:"action" desc:"Action verb such as acquire, use, lose, move, combat, learn, discover"`
	Target     string   `json:"target,omitempty" md:"target" desc:"Action target"`
	TargetType string   `json:"target_type,omitempty" md:"target_type" desc:"item, character, location, skill, status, relationship, or knowledge"`
	Context    string   `json:"context,omitempty" md:"context" desc:"Where or why it happens"`
	Result     string   `json:"result,omitempty" md:"result" desc:"Narrative result"`
}

// ImproveProgress tracks the progress of hierarchical improvement
type ImproveProgress struct {
	Iteration        int                 `json:"iteration"`          // Current iteration number
	TotalIterations  int                 `json:"total_iterations"`   // Total iterations planned
	CurrentVolumeIdx int                 `json:"current_volume_idx"` // Index of next volume to improve (0-based)
	TotalVolumes     int                 `json:"total_volumes"`      // Total volumes to improve
	TargetVolumes    []string            `json:"target_volumes,omitempty"`
	CompletedVolumes []string            `json:"completed_volumes"` // List of completed volume IDs
	Outline          models.Outline      `json:"outline"`           // Current state of outline
	ReviewResult     models.ReviewResult `json:"review_result"`     // Review result for current iteration
}

// ComposeAgent handles AI generation for story outline
// It wraps BaseAgent to provide type-safe methods
type ComposeAgent struct {
	base *BaseAgent
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *ComposeAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "ComposeAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &ComposeAgent{base: base}
}

// SetLanguage sets the output language
func (a *ComposeAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// SetModelOverride overrides the project model for this agent's chat options.
func (a *ComposeAgent) SetModelOverride(model string) {
	if a != nil && a.base != nil {
		a.base.SetModelOverride(model)
	}
}

func composeAgentSDKParams(command, workflowSkill string, maxTurns int, toolAllowlist []string) InvokeParams {
	if maxTurns <= 0 {
		maxTurns = 12
	}
	toolAllowlist = append(append([]string(nil), toolAllowlist...), agentSDKLogToolAllowlist()...)
	return InvokeParams{
		SDKSkills:      []string{"novel-tools-core", workflowSkill},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  dedupeComposeToolAllowlist(toolAllowlist),
		MaxTurns:       maxTurns,
		Timeout:        900,
		Command:        command,
	}
}

func composeRequiredQueryAllowlist(requiredQueries []string) []string {
	return dedupeComposeToolAllowlist(requiredQueries)
}

func composeImproveToolAllowlist(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) []string {
	allowlist := composeRequiredQueryAllowlist(promptInput.RequiredQueries)
	allowlist = append(allowlist, composeImproveVolumeToolAllowlist(volume, promptInput, applyPatches)...)
	for _, suggestion := range promptInput.ReviewResult.Suggestions {
		if suggestion.Navigation == nil {
			continue
		}
		allowlist = append(allowlist, suggestion.Navigation.DetailQueries...)
		allowlist = append(allowlist, suggestion.Navigation.FocusedCheckQuery)
		allowlist = append(allowlist, suggestion.Navigation.RepairRouteQuery)
		allowlist = append(allowlist, suggestion.Navigation.RepairContextQuery)
	}
	return dedupeComposeToolAllowlist(allowlist)
}

// composeImproveVolumeToolAllowlist returns the query/check/patch allowlist for
// ONE volume (used by single-volume improve and reused per volume in
// cross-volume mode so the agent can patch several volumes in one session).
func composeImproveVolumeToolAllowlist(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) []string {
	allowlist := composeRequiredQueryAllowlist(promptInput.RequiredQueries)
	volumeID := strings.TrimSpace(volume.ID)
	detailChapterIDs, restrictDetailChapters := composeImproveDetailChapterIDs(volume, promptInput)
	allowExploratoryDetail := composeImproveHasFocusedTasks(promptInput) && !composeImproveUsesCleanProbe(promptInput)
	if volumeID != "" {
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query context --type outline-volume --id %s --view brief", volumeID),
			fmt.Sprintf("novelgen tool query context --type outline-repair --id %s --name", volumeID),
			fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q", volumeID),
		)
		if allowExploratoryDetail {
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %s", volumeID),
				fmt.Sprintf("novelgen tool query outline --type volume --id %q", volumeID),
				"novelgen tool query outline --type refs --entity-type character --name",
				"novelgen tool query outline --type refs --entity-type item --name",
				"novelgen tool query outline --type refs --entity-type location --name",
				"novelgen tool query story-setup --type search",
				"novelgen tool query story-setup --type core-cast --name",
				"novelgen tool query story-setup --type storyline --name",
				"novelgen tool query story-setup --type premise --name",
				"novelgen tool query story-setup --type resource --name",
				"novelgen tool query story-setup --type timeline --name",
				fmt.Sprintf("novelgen tool query outline --type events --volume-id %q --view brief", volumeID),
				fmt.Sprintf("novelgen tool query outline --type events --volume-id %q --fields action,actor,target,target_type,type,details,result --view brief", volumeID),
			)
		}
	}
	for _, chapter := range volume.Chapters {
		chapterID := strings.TrimSpace(chapter.ID)
		if chapterID == "" {
			continue
		}
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query context --type outline-repair --id %s --name", chapterID),
			fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q", chapterID),
		)
		if allowExploratoryDetail && (!restrictDetailChapters || detailChapterIDs[chapterID]) {
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", chapterID),
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --fields scenes --view brief", chapterID),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --view brief", chapterID),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --fields result,details,target,target_type,actor,action --view brief", chapterID),
			)
		}
	}
	patchTool := "novelgen tool patch outline --target volume"
	if volumeID != "" {
		patchTool += fmt.Sprintf(" --id %q", volumeID)
	}
	if applyPatches {
		patchTool += " --apply"
	}
	allowlist = append(allowlist, patchTool)
	return allowlist
}

// composeAdjacentVolumeIDs returns the IDs of the volumes immediately before
// and after the given volume in the flattened outline order. The current
// volume itself is never included, and volumes at part boundaries still
// consider the neighboring part's first/last volume as adjacent.
func composeAdjacentVolumeIDs(outline models.Outline, volumeID string) []string {
	volumeIDs := make([]string, 0)
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			volumeIDs = append(volumeIDs, volume.ID)
		}
	}
	for index, id := range volumeIDs {
		if id != volumeID {
			continue
		}
		adjacent := make([]string, 0, 2)
		if index > 0 {
			adjacent = append(adjacent, volumeIDs[index-1])
		}
		if index+1 < len(volumeIDs) {
			adjacent = append(adjacent, volumeIDs[index+1])
		}
		return adjacent
	}
	return nil
}

// composeAdjacentVolumeQueryAllowlist adds read-only payoff_contract/summary
// queries for the immediately adjacent volumes, so the Agent SDK workflow can
// verify cross-volume continuity facts without being denied by the allowlist.
// Adjacent volumes are never patchable in this workflow.
func composeAdjacentVolumeQueryAllowlist(outline models.Outline, volumeID string) []string {
	allowlist := make([]string, 0, 2)
	for _, adjacentID := range composeAdjacentVolumeIDs(outline, volumeID) {
		if strings.TrimSpace(adjacentID) == "" {
			continue
		}
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query outline --type volume --id %q --fields payoff_contract,summary --view brief", adjacentID),
		)
	}
	return allowlist
}

func composeImproveHasFocusedTasks(promptInput composeAgentSDKImproveVolumePromptInput) bool {
	return promptInput.ForceIssueRepair ||
		strings.TrimSpace(promptInput.UserPrompt) != "" ||
		len(promptInput.ReviewResult.Suggestions) > 0
}

func composeImproveUsesCleanProbe(promptInput composeAgentSDKImproveVolumePromptInput) bool {
	return len(promptInput.ReviewResult.Suggestions) == 0 && composePromptRequestsCheckFirstNoop(promptInput.UserPrompt)
}

func composeImproveDetailChapterIDs(volume models.Volume, promptInput composeAgentSDKImproveVolumePromptInput) (map[string]bool, bool) {
	if !composeImproveHasFocusedTasks(promptInput) {
		return nil, false
	}
	volumeChapterIDs := map[string]bool{}
	for _, chapter := range volume.Chapters {
		if id := strings.TrimSpace(chapter.ID); id != "" {
			volumeChapterIDs[id] = true
		}
	}
	ids := map[string]bool{}
	for _, suggestion := range promptInput.ReviewResult.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID == strings.TrimSpace(volume.ID) {
			return nil, false
		}
		if volumeChapterIDs[targetID] {
			ids[targetID] = true
			composeAddAdjacentChapterIDs(ids, volume, targetID)
		}
	}
	for _, id := range composeImproveBoundaryChapterIDs(volume, promptInput.UserPrompt) {
		if volumeChapterIDs[id] {
			ids[id] = true
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

func composeAddAdjacentChapterIDs(ids map[string]bool, volume models.Volume, targetID string) {
	for i, chapter := range volume.Chapters {
		if strings.TrimSpace(chapter.ID) != targetID {
			continue
		}
		if i > 0 {
			if id := strings.TrimSpace(volume.Chapters[i-1].ID); id != "" {
				ids[id] = true
			}
		}
		if i+1 < len(volume.Chapters) {
			if id := strings.TrimSpace(volume.Chapters[i+1].ID); id != "" {
				ids[id] = true
			}
		}
		return
	}
}

func composeImproveBoundaryChapterIDs(volume models.Volume, prompt string) []string {
	rawPrompt := strings.TrimSpace(prompt)
	prompt = strings.ToLower(rawPrompt)
	if prompt == "" || composeImprovePromptRequestsAllChapters(prompt) {
		return nil
	}
	ids := []string{}
	for i, chapter := range volume.Chapters {
		id := strings.TrimSpace(chapter.ID)
		if id == "" {
			continue
		}
		if strings.Contains(prompt, strings.ToLower(id)) || composeImprovePromptMentionsChapterOrdinal(prompt, i+1) {
			ids = append(ids, id)
		}
	}
	if composeImprovePromptMentionsFirstChapter(prompt) && len(volume.Chapters) > 0 {
		if id := strings.TrimSpace(volume.Chapters[0].ID); id != "" {
			if !composeStringSliceContains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	if composeImprovePromptMentionsLastChapter(prompt) && len(volume.Chapters) > 0 {
		if id := strings.TrimSpace(volume.Chapters[len(volume.Chapters)-1].ID); id != "" && !composeStringSliceContains(ids, id) {
			ids = append(ids, id)
		}
	}
	// A prompt that enumerates every chapter in a multi-chapter volume is a
	// whole-volume revision, not a boundary-scoped request: there is no
	// narrower subset to restrict queries to, and demanding a focused check
	// per chapter would be redundant with the count-based evidence minimums
	// (and would fail runs where the agent reasonably skips checks for
	// unchanged chapters). Single-chapter volumes keep the boundary so a
	// specific chapter target still filters global suggestions.
	if len(volume.Chapters) > 1 && len(ids) == len(volume.Chapters) {
		return nil
	}
	return ids
}

func AgentSDKImproveBoundaryChapterIDs(volume models.Volume, prompt string) []string {
	return append([]string(nil), composeImproveBoundaryChapterIDs(volume, prompt)...)
}

func composeImproveRequiredFocusedChecks(volume models.Volume, prompt string) []string {
	ids := composeImproveBoundaryChapterIDs(volume, prompt)
	if len(ids) == 0 {
		return nil
	}
	categories := composeImprovePromptCheckCategories(prompt)
	checks := make([]string, 0, len(ids)*len(categories))
	for _, id := range ids {
		for _, category := range categories {
			if strings.EqualFold(strings.TrimSpace(category), "all") {
				// "all" 意味着任意 category 均可：用裸命令(不带 --category)作为要求，
				// 这样 agent 实际跑的 --category pacing/logic/... 变体都能被 contains 匹配。
				checks = append(checks, fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q", id))
				continue
			}
			checks = append(checks, fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --category %s --min-priority low --max-issues 8", id, category))
		}
	}
	return checks
}

func composeImprovePromptCheckCategories(prompt string) []string {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	categories := []string{}
	if strings.Contains(prompt, "logic") || strings.Contains(prompt, "逻辑") || strings.Contains(prompt, "连贯") || strings.Contains(prompt, "承接") {
		categories = append(categories, "logic")
	}
	if strings.Contains(prompt, "transition") || strings.Contains(prompt, "过渡") || strings.Contains(prompt, "地点") || strings.Contains(prompt, "移动") {
		categories = append(categories, "transition")
	}
	if strings.Contains(prompt, "state_anchor") || strings.Contains(prompt, "状态锚点") || strings.Contains(prompt, "修炼境界") {
		categories = append(categories, "state_anchor")
	}
	if len(categories) == 0 {
		categories = append(categories, "all")
	}
	return categories
}

func composeStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func composeImprovePromptRequestsAllChapters(prompt string) bool {
	for _, marker := range []string{"每章", "每一章", "所有章节", "全部章节", "逐章", "全卷", "整卷", "all chapters", "every chapter", "whole volume"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeImprovePromptMentionsChapterOrdinal(prompt string, ordinal int) bool {
	if ordinal <= 0 {
		return false
	}
	markers := []string{
		fmt.Sprintf("第%d章", ordinal),
		fmt.Sprintf("第 %d 章", ordinal),
		fmt.Sprintf("chapter %d", ordinal),
		fmt.Sprintf("ch%d", ordinal),
	}
	if cn := smallChineseOrdinal(ordinal); cn != "" {
		markers = append(markers, "第"+cn+"章")
	}
	for _, marker := range markers {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func smallChineseOrdinal(value int) string {
	if value <= 0 || value > 99 {
		return ""
	}
	digits := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if value < 10 {
		return digits[value]
	}
	if value == 10 {
		return "十"
	}
	tens := value / 10
	ones := value % 10
	if tens == 1 {
		if ones == 0 {
			return "十"
		}
		return "十" + digits[ones]
	}
	if ones == 0 {
		return digits[tens] + "十"
	}
	return digits[tens] + "十" + digits[ones]
}

func composeImprovePromptMentionsFirstChapter(prompt string) bool {
	for _, marker := range []string{"首章", "第一章", "开头", "开篇", "卷首", "first chapter", "opening chapter"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeImprovePromptMentionsLastChapter(prompt string) bool {
	for _, marker := range []string{"卷尾", "尾章", "最后一章", "末章", "结尾", "收束", "卷末", "final chapter", "last chapter", "ending"} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func composeGlobalRepairToolAllowlist(applyPatches bool) []string {
	allowlist := []string{
		"novelgen tool query context --type outline-global-repair",
		"novelgen tool query story-setup --type search",
		"novelgen tool check all --target outline",
		"novelgen tool check all --target setup",
		"novelgen tool patch setup",
		"novelgen tool patch outline --target volume --id",
	}
	if applyPatches {
		allowlist = append(allowlist, "novelgen tool patch setup --apply")
		allowlist = append(allowlist, "novelgen tool patch outline --target volume --id --apply")
	}
	return dedupeComposeToolAllowlist(allowlist)
}

func composeGlobalRepairToolAllowlistForReview(review models.ReviewResult, applyPatches bool) []string {
	allowlist := []string{
		"novelgen tool query context --type outline-global-repair",
		"novelgen tool query story-setup --type search",
		"novelgen tool patch setup",
		"novelgen tool patch outline --target volume --id",
	}
	checks := composeGlobalRepairRequiredPostPatchChecks(review)
	if len(checks) == 0 {
		allowlist = append(allowlist,
			"novelgen tool check all --target outline",
			"novelgen tool check all --target setup",
		)
	} else {
		allowlist = append(allowlist, checks...)
	}
	if applyPatches {
		allowlist = append(allowlist, "novelgen tool patch setup --apply")
		allowlist = append(allowlist, "novelgen tool patch outline --target volume --id --apply")
	}
	return dedupeComposeToolAllowlist(allowlist)
}

func composeGlobalRepairRequiredPostPatchChecks(review models.ReviewResult) []string {
	for _, suggestion := range review.Suggestions {
		category := composeAgentSDKCheckCategory(suggestion.Category)
		if category == "" {
			continue
		}
		targetID := strings.TrimSpace(suggestion.TargetID)
		if category == "faction_tier" && targetID != "" && !strings.EqualFold(targetID, "global") {
			return []string{fmt.Sprintf("novelgen tool check all --target setup --category %s --min-priority low --max-issues 12", category)}
		}
		return []string{fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", category)}
	}
	return nil
}

func dedupeComposeToolAllowlist(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// GenerateSkeletonWithAgentSDK asks the Claude Agent SDK workflow to query
// project facts and return an outline skeleton. Go still validates and saves.
func (a *ComposeAgent) GenerateSkeletonWithAgentSDK(ctx context.Context, input ComposeSkeletonInput) (ComposeSkeletonOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Generating Outline Skeleton")
	var output ComposeSkeletonOutput
	promptInput := composeAgentSDKSkeletonPromptInput{
		Structure: input.Structure,
		RequiredQueries: []string{
			"novelgen tool query story-setup --view brief",
		},
		Instructions: []string{
			"Do not use the setup_brief shortcut; query the project facts with the required command.",
			"Use --view brief or --view index before asking for full detail; avoid broad full-output queries.",
			"Return only the parts/volumes skeleton. Chapters must be empty arrays.",
		},
	}
	params := composeAgentSDKParams("generate the story outline skeleton using read-only project queries", "outline-compose-skeleton-workflow", 10, composeRequiredQueryAllowlist(promptInput.RequiredQueries))
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeSkeletonOutput{}, err
	}
	if len(output.Parts) != input.Structure.TargetParts {
		return ComposeSkeletonOutput{}, fmt.Errorf("agent SDK generated %d parts, but %d were requested", len(output.Parts), input.Structure.TargetParts)
	}
	for i, part := range output.Parts {
		if len(part.Volumes) != input.Structure.TargetVolumes {
			return ComposeSkeletonOutput{}, fmt.Errorf("agent SDK part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), input.Structure.TargetVolumes)
		}
	}
	return output, nil
}

// GenerateChaptersForVolumeWithAgentSDK asks the SDK workflow to fill one
// volume. Only returned chapters are accepted; volume identity stays in Go.
func (a *ComposeAgent) GenerateChaptersForVolumeWithAgentSDK(ctx context.Context, input ComposeChaptersInput) (ComposeImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Generating Chapters for Volume")
	logger.Info("Volume: %s (%d/%d)", input.Volume.Title, input.VolumeIndex, input.TotalVolumes)

	var output ComposeImproveVolumeOutput
	promptInput := buildAgentSDKChaptersPromptInput(input)
	params := composeAgentSDKParams("generate chapters for this volume using read-only project queries", "outline-compose-volume-workflow", 24, composeRequiredQueryAllowlist(promptInput.RequiredQueries))
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeImproveVolumeOutput{}, err
	}
	volume, err := prepareAgentSDKReturnedVolume(input.Volume, output.Volume, input.ChaptersPerVol)
	if err != nil {
		return ComposeImproveVolumeOutput{}, err
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}
	output.Volume = volume
	return output, nil
}

// ImproveVolumeWithAgentSDK lets the SDK workflow query context, review a
// target volume, and return an improved volume plus review metadata.
func (a *ComposeAgent) ImproveVolumeWithAgentSDK(ctx context.Context, input ComposeImproveVolumeInput, applyPatches bool, forceIssueRepair bool) (ComposeAgentSDKImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT SDK - Volume Review/Improvement")
	crossVolume := len(input.CrossVolumeIDs) > 0
	if crossVolume {
		logger.Info("Cross-volume mode: target=%s cross=%v", input.Volume.ID, input.CrossVolumeIDs)
	} else {
		logger.Info("Volume: %s (single-volume mode, CrossVolumeIDs=%d)", input.Volume.Title, len(input.CrossVolumeIDs))
	}
	if applyPatches {
		logger.Info("Agent apply enabled: SDK may write through validated outline patch tools")
	}
	if forceIssueRepair {
		logger.Info("Focused review issues are treated as repair tasks, including directly fixable low-priority issues")
	}

	promptInput := buildAgentSDKImproveVolumePromptInput(input, applyPatches, forceIssueRepair)
	allowlist := composeImproveToolAllowlist(input.Volume, promptInput, applyPatches)
	if crossVolume {
		// Cross-volume mode: allow query/check/patch on every target volume in one session.
		for _, vid := range input.CrossVolumeIDs {
			vid = strings.TrimSpace(vid)
			if vid == "" {
				continue
			}
			for _, vol := range input.Outline.Parts {
				for _, v := range vol.Volumes {
					if strings.EqualFold(strings.TrimSpace(v.ID), vid) {
						allowlist = append(allowlist, composeImproveVolumeToolAllowlist(v, promptInput, applyPatches)...)
					}
				}
			}
		}
		allowlist = append(allowlist,
			"novelgen tool query outline --type refs --entity-type storyline --name",
			"novelgen tool query outline --type refs --entity-type character --name",
			"novelgen tool query outline --type refs --entity-type item --name",
			"novelgen tool query outline --type refs --entity-type location --name",
		)
	}
	allowlist = append(allowlist, composeAdjacentVolumeQueryAllowlist(input.Outline, input.Volume.ID)...)
	params := composeAgentSDKParams("review and improve this outline volume using project query/check/patch tools", "outline-improve-volume-workflow", 28, dedupeComposeToolAllowlist(allowlist))
	params.ToolEvidence = composeImproveToolEvidence(promptInput, applyPatches)
	// An explicit user prompt makes this an explicit task list: clean scoped
	// checks must not close targets or stop the agent before the patch cycle.
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""
	if crossVolume {
		// Cross-volume sessions cover 2-3 volumes (target + adjacent) with up to
		// 45 turns; the default 900s timeout is too tight and caused TimeoutError
		// mid-run. Give them a 30-minute budget.
		params.MaxTurns = 45
		params.Timeout = 1800
	}
	if applyPatches {
		params.ToolEvidence.RequirePatchApplyFollowupCheck = true
	}

	if applyPatches {
		var applyOutput composeAgentSDKApplyOutput
		runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &applyOutput)
		if err != nil {
			if recovered, recoverErr := a.recoverAppliedVolumeAfterAgentSDKError(input.Volume, runtimeResult, err); recoverErr == nil {
				return recovered, nil
			}
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, applyOutput.AppliedPatches, applyOutput.AppliedPatchCount); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		if err := validateAgentSDKApplyOutput(applyOutput); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, err
		}
		reviewResult := applyOutput.ReviewResult.toModelReviewResult()
		reviewResult.NormalizeScoreScale()
		volume := input.Volume
		if applyOutput.AppliedPatches {
			var err error
			volume, err = loadAgentSDKAppliedOutlineVolume(input.Volume, true)
			if err != nil {
				return ComposeAgentSDKImproveVolumeOutput{}, err
			}
		}
		for i, chapter := range volume.Chapters {
			if err := a.validateChapterOutput(&chapter); err != nil {
				return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
			}
		}
		return ComposeAgentSDKImproveVolumeOutput{
			ReviewResult: reviewResult,
			Volume:       volume,
		}, nil
	}

	var patchOutput composeAgentSDKImprovePatchOutput
	runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &patchOutput)
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, false, 0); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := validateAgentSDKImprovePatchOutput(patchOutput, input.Volume.ID); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	if err := utils.ValidateNoSuspiciousPatchText(patchOutput.VolumePatch); err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("agent SDK volume patch rejected: %w", err)
	}
	reviewResult := patchOutput.ReviewResult.toModelReviewResult()
	reviewResult.NormalizeScoreScale()
	volume, err := applyAgentSDKVolumePatch(input.Volume, patchOutput.VolumePatch, len(input.Volume.Chapters))
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, err
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}
	return ComposeAgentSDKImproveVolumeOutput{
		ReviewResult: reviewResult,
		Volume:       volume,
	}, nil
}

// ReviewOutlineWithAgentSDK reviews the outline (whole book or one volume)
// through the read-only Agent SDK workflow and returns a ReviewResult. The
// review is deliberately orthogonal to deterministic checks: it reports
// open-ended issues (pacing, motivation, theme, information warfare) that
// rule-based checks cannot judge, and never patches project state.
func (a *ComposeAgent) ReviewOutlineWithAgentSDK(ctx context.Context, input ComposeOutlineReviewInput) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Outline Review")
	scope := "whole outline"
	if input.CrossVolume && input.FromVolumeIndex > 0 {
		scope = fmt.Sprintf("volumes %d-%d (cross-volume)", input.FromVolumeIndex, input.ToVolumeIndex)
	} else if strings.TrimSpace(input.VolumeID) != "" {
		scope = "volume " + strings.TrimSpace(input.VolumeID)
	}
	logger.Info("Scope: %s", scope)

	if input.CrossVolume {
		return a.reviewOutlineCrossVolumeAgentSDK(ctx, input)
	}

	promptInput := buildAgentSDKOutlineReviewPromptInput(input)
	params := composeOutlineReviewAgentSDKParams("review the story outline and provide focused improvement suggestions", input.Outline, input.VolumeID)
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""
	var output models.ReviewResult
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return models.ReviewResult{}, err
	}
	output.NormalizeScoreScale()
	clipOutlineReviewResult(&output)
	logger.Section("Outline Agent SDK Review Result")
	logger.Info("Overall Score: %.1f/100", output.OverallScore)
	logger.Info("Suggestions: %d", len(output.Suggestions))
	return output, nil
}

// reviewOutlineCrossVolumeAgentSDK 跨卷审查: 一次 session 内按"线"审查多卷之间的连续性问题。
func (a *ComposeAgent) reviewOutlineCrossVolumeAgentSDK(ctx context.Context, input ComposeOutlineReviewInput) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Cross-Volume Outline Review")

	// 收集范围内的卷
	var inScope []models.Volume
	for _, part := range input.Outline.Parts {
		for _, volume := range part.Volumes {
			idx := globalVolumeIndex(input.Outline, volume.ID)
			if idx >= input.FromVolumeIndex && (input.ToVolumeIndex <= 0 || idx <= input.ToVolumeIndex) {
				inScope = append(inScope, volume)
			}
		}
	}
	if len(inScope) == 0 {
		return models.ReviewResult{}, fmt.Errorf("cross-volume review: no volumes in range %d-%d", input.FromVolumeIndex, input.ToVolumeIndex)
	}
	volCount, chapCount := 0, 0
	for _, v := range inScope {
		volCount++
		chapCount += len(v.Chapters)
	}
	logger.Info("Cross-volume scope: %d volumes, %d chapters", volCount, chapCount)

	instructions := []string{
		"Run the required first query exactly as given before anything else. It returns the overall structure of the review scope.",
		"This is a read-only cross-volume review: do not patch, do not run check, do not write files, do not read source/RPG/Claude tool-results.",
		"Focus on CROSS-VOLUME issues only: foreshadowing lifecycle, setup consistency drift, repeated tropes across volumes, macro pacing, volume-end hook handoff, character arc continuity. Single-volume issues are out of scope.",
		"Every suggestion must cite facts from at least TWO different volumes/chapters in scope. Base every claim on facts visible in tool query results; do not assert an unresolved mystery or contradiction unless the queried facts show it.",
		"Use `novelgen tool query outline --type refs --entity-type storyline|character|item --name \"<name>\" --view brief` to trace a line across volumes. Cross-check setup book via story-setup queries for promise-vs-delivery gaps.",
		"Every suggestion target_id must exist in the review scope. For cross-volume issues not tied to one target, point at the most relevant volume/chapter or omit target_id.",
		"Keep review_result compact: summary under 300 Chinese characters, at most 10 suggestions, at most 4 strengths and 4 weaknesses.",
		"Return only the JSON object; no Markdown, no code fences, no extra text.",
	}
	focus := "按跨卷维度自由审查：伏笔生命周期、设定一致性漂移、跨卷重复套路、宏观节奏、卷间钩子衔接、角色跨卷弧线。"
	if strings.TrimSpace(input.UserPrompt) != "" {
		instructions = append(instructions,
			"user_prompt 是本次审查的最高优先级任务清单。围绕它逐条分析，再补充你发现的跨卷问题。",
		)
		focus = "以 user_prompt 为最高优先级；在此基础上补充伏笔生命周期、设定一致性漂移、跨卷重复套路、宏观节奏、卷间钩子衔接、角色跨卷弧线等跨卷维度。"
	}
	if input.SurfaceOnly {
		instructions = append(instructions,
			"SURFACE-ONLY REVIEW: you can ONLY see reader-visible fields (volume summary, chapter title/summary/opening_beat/scenes). Internal structure fields (events, payoff_contract, state_anchor, state_change, conflicts) are NOT available and MUST NOT be assumed or guessed. Judge the outline the way a reader experiences it: pacing, hooks, character motivation, emotional beats, foreshadowing as it reads — not as data-structure panels. Do not report issues that would be invisible to a reader (e.g. a forced data-slot value, a field format, an engine check artifact).",
		)
	}

	promptInput := composeAgentSDKOutlineReviewPromptInput{
		TargetVolumeID:  fmt.Sprintf("%d-%d (cross-volume)", input.FromVolumeIndex, input.ToVolumeIndex),
		VolumeCount:     volCount,
		ChapterCount:    chapCount,
		UserPrompt:      strings.TrimSpace(input.UserPrompt),
		ReviewFocus:     focus,
		RequiredQueries: []string{"novelgen tool query outline --type all --view index"},
		Instructions:    instructions,
	}

	allowlist := []string{
		"novelgen tool query story-setup --type search",
		"novelgen tool query story-setup --type index",
		"novelgen tool query story-setup --type all",
		"novelgen tool query story-setup --type core-cast",
		"novelgen tool query story-setup --type storyline",
		"novelgen tool query story-setup --type premise",
		"novelgen tool query story-setup --type resource",
		"novelgen tool query story-setup --type timeline",
		"novelgen tool query outline --type all --view index",
		"novelgen tool query outline --type all --view brief",
		"novelgen tool query outline --type part --id",
		"novelgen tool query outline --type refs --entity-type storyline --name",
		"novelgen tool query outline --type refs --entity-type character --name",
		"novelgen tool query outline --type refs --entity-type item --name",
		"novelgen tool query outline --type refs --entity-type location --name",
	}
	for _, v := range inScope {
		vid := strings.TrimSpace(v.ID)
		if vid == "" {
			continue
		}
		if input.SurfaceOnly {
			// Reader-visible fields only: title/summary/opening_beat/scenes.
			// No events, no payoff_contract, no state_anchor — the AI-flavor
			// review judges what a reader actually sees, not the author's
			// internal structure panels (which are routinely misread as
			// 'AI flavor' — e.g. the forced opponent-misread slot).
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool query outline --type volume --id %q --fields summary --view brief", vid),
			)
			for _, chapter := range v.Chapters {
				cid := strings.TrimSpace(chapter.ID)
				if cid == "" {
					continue
				}
				allowlist = append(allowlist,
					fmt.Sprintf("novelgen tool query outline --type chapter --id %q --fields title,summary,opening_beat,scenes --view brief", cid),
				)
			}
			continue
		}
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query outline --type volume --id %q --view brief", vid),
			fmt.Sprintf("novelgen tool query outline --type events --volume-id %q --view brief", vid),
		)
		for _, chapter := range v.Chapters {
			cid := strings.TrimSpace(chapter.ID)
			if cid == "" {
				continue
			}
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", cid),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --view brief", cid),
			)
		}
	}

	params := InvokeParams{
		SDKSkills:      []string{"novel-tools-core", "outline-review-workflow", "cross-volume-review-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  dedupeComposeToolAllowlist(allowlist),
		ToolEvidence: ToolEvidenceRequirement{
			MinQueryCalls:        2,
			RequireNoDeniedTools: true,
			RequiredToolCommands: []string{"novelgen tool query outline --type all --view index"},
		},
		MaxTurns: 45,
		Timeout:  1800,
		Command:  "review the cross-volume continuity of the story outline and provide focused improvement suggestions",
	}
	params.UserPromptDriven = strings.TrimSpace(input.UserPrompt) != ""

	var output models.ReviewResult
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return models.ReviewResult{}, err
	}
	output.NormalizeScoreScale()
	clipOutlineReviewResult(&output)
	logger.Section("Cross-Volume Agent SDK Review Result")
	logger.Info("Overall Score: %.1f/100", output.OverallScore)
	logger.Info("Suggestions: %d", len(output.Suggestions))
	return output, nil
}

func globalVolumeIndex(outline models.Outline, volumeID string) int {
	idx := 0
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			idx++
			if strings.EqualFold(strings.TrimSpace(volume.ID), strings.TrimSpace(volumeID)) {
				return idx
			}
		}
	}
	return 0
}

func buildAgentSDKOutlineReviewPromptInput(input ComposeOutlineReviewInput) composeAgentSDKOutlineReviewPromptInput {
	volumeCount, chapterCount := countOutlineScope(input.Outline, input.VolumeID)
	instructions := []string{
		"Run the required first query exactly as given before anything else. It returns the overall structure of the review scope.",
		"This is a read-only review: do not patch, do not run check, do not write files, do not read source/RPG/Claude tool-results.",
		"Base every claim on facts visible in tool query results. Do not assert an unresolved mystery or contradiction unless the queried facts show it.",
		"Every suggestion target_id must exist in the review scope. If a suggestion is not tied to a concrete volume/chapter, omit target_id.",
		"Keep review_result compact: summary under 300 Chinese characters, at most 8 suggestions, at most 4 strengths and 4 weaknesses.",
		"Return only the JSON object; no Markdown, no code fences, no extra text.",
	}
	if strings.TrimSpace(input.VolumeID) != "" && !input.CrossVolume {
		instructions = append(instructions,
			"Cross-volume context available: the immediately adjacent volumes (previous + next) can be queried read-only (volume/events/chapter queries are allowlisted). When an issue involves a volume-end hook, a next-volume payoff, setup planted elsewhere, or continuity drift, query the adjacent volumes and cite the cross-volume evidence instead of guessing. Keep the review focused on the target volume; adjacent volumes are evidence context, not the review target.",
		)
	}
	focus := "按以下维度自由审查：结构、节奏、连贯性、人物动机、情节逻辑、信息差博弈。"
	if strings.TrimSpace(input.UserPrompt) != "" {
		instructions = append(instructions,
			"user_prompt 是本次审查的最高优先级任务清单。围绕它逐条分析，再补充你发现的其他开放性问题。",
		)
		focus = "以 user_prompt 为最高优先级；在此基础上补充结构、节奏、连贯性、人物动机、情节逻辑、信息差博弈等维度的开放性问题。"
	}
	return composeAgentSDKOutlineReviewPromptInput{
		TargetVolumeID:  strings.TrimSpace(input.VolumeID),
		VolumeCount:     volumeCount,
		ChapterCount:    chapterCount,
		UserPrompt:      strings.TrimSpace(input.UserPrompt),
		ReviewFocus:     focus,
		RequiredQueries: composeOutlineReviewRequiredQueries(input.Outline, input.VolumeID),
		Instructions:    instructions,
	}
}

func countOutlineScope(outline models.Outline, volumeID string) (int, int) {
	volumes, chapters := 0, 0
	volumeID = strings.TrimSpace(volumeID)
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if volumeID != "" && !strings.EqualFold(strings.TrimSpace(volume.ID), volumeID) {
				continue
			}
			volumes++
			chapters += len(volume.Chapters)
		}
	}
	return volumes, chapters
}

func composeOutlineReviewRequiredQueries(outline models.Outline, volumeID string) []string {
	volumeID = strings.TrimSpace(volumeID)
	if volumeID != "" {
		return []string{fmt.Sprintf("novelgen tool query context --type outline-volume --id %s --view index", volumeID)}
	}
	return []string{"novelgen tool query outline --type all --view index"}
}

func composeOutlineReviewAgentSDKParams(command string, outline models.Outline, volumeID string) InvokeParams {
	allowlist := composeOutlineReviewToolAllowlist(outline, volumeID)
	return InvokeParams{
		SDKSkills:      []string{"novel-tools-core", "outline-review-workflow"},
		Tools:          []string{"Bash"},
		AllowedTools:   []string{"Bash"},
		PermissionMode: "dontAsk",
		RequireSDK:     true,
		ToolAllowlist:  dedupeComposeToolAllowlist(allowlist),
		ToolEvidence: ToolEvidenceRequirement{
			MinQueryCalls:        1,
			RequireNoDeniedTools: true,
			RequiredToolCommands: composeOutlineReviewRequiredQueries(outline, volumeID),
		},
		MaxTurns: 20,
		Timeout:  600,
		Command:  command,
	}
}

func composeOutlineReviewToolAllowlist(outline models.Outline, volumeID string) []string {
	allowlist := []string{
		"novelgen tool query story-setup --type search",
		"novelgen tool query story-setup --type index",
		"novelgen tool query story-setup --type all",
		"novelgen tool query story-setup --type core-cast",
		"novelgen tool query story-setup --type storyline",
		"novelgen tool query story-setup --type premise",
		"novelgen tool query story-setup --type resource",
		"novelgen tool query story-setup --type timeline",
		"novelgen tool query context --type outline-volume",
		"novelgen tool query outline --type refs --entity-type storyline --name",
		"novelgen tool query outline --type refs --entity-type character --name",
		"novelgen tool query outline --type refs --entity-type item --name",
		"novelgen tool query outline --type refs --entity-type location --name",
	}
	addVolume := func(volume models.Volume) {
		vid := strings.TrimSpace(volume.ID)
		if vid == "" {
			return
		}
		allowlist = append(allowlist,
			fmt.Sprintf("novelgen tool query outline --type volume --id %q --view brief", vid),
			fmt.Sprintf("novelgen tool query outline --type events --volume-id %q --view brief", vid),
		)
		for _, chapter := range volume.Chapters {
			cid := strings.TrimSpace(chapter.ID)
			if cid == "" {
				continue
			}
			allowlist = append(allowlist,
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", cid),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --view brief", cid),
			)
		}
	}
	volumeID = strings.TrimSpace(volumeID)
	if volumeID != "" {
		allowlist = append(allowlist, fmt.Sprintf("novelgen tool query context --type outline-volume --id %s --view index", volumeID))
		for _, part := range outline.Parts {
			for _, volume := range part.Volumes {
				if strings.EqualFold(strings.TrimSpace(volume.ID), volumeID) {
					addVolume(volume)
					// Cross-volume by default: also allow read-only queries of the
					// adjacent volumes (previous + next) so the review can check
					// volume-end hooks, next-volume payoffs, and continuity drift
					// without a separate --cross-volume run.
					for _, adjID := range composeAdjacentVolumeIDs(outline, volume.ID) {
						for _, part2 := range outline.Parts {
							for _, adj := range part2.Volumes {
								if strings.EqualFold(strings.TrimSpace(adj.ID), adjID) {
									addVolume(adj)
								}
							}
						}
					}
				}
			}
		}
		return dedupeComposeToolAllowlist(allowlist)
	}
	allowlist = append(allowlist, "novelgen tool query outline --type all --view index")
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			addVolume(volume)
		}
	}
	return dedupeComposeToolAllowlist(allowlist)
}

func clipOutlineReviewResult(result *models.ReviewResult) {
	if result == nil {
		return
	}
	result.Summary = clipForPrompt(result.Summary, 300)
	result.Strengths = clipReviewStringList(result.Strengths, 4, 180)
	result.Weaknesses = clipReviewStringList(result.Weaknesses, 4, 220)
	suggestions := make([]models.ReviewSuggestion, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		s.Category = strings.TrimSpace(s.Category)
		s.TargetID = strings.TrimSpace(s.TargetID)
		s.TargetName = clipForPrompt(s.TargetName, 120)
		s.Issue = clipForPrompt(s.Issue, 360)
		s.Suggestion = clipForPrompt(s.Suggestion, 420)
		s.Priority = strings.TrimSpace(s.Priority)
		suggestions = append(suggestions, s)
		if len(suggestions) >= 8 {
			break
		}
	}
	result.Suggestions = suggestions
}

func clipReviewStringList(values []string, maxItems int, maxRunes int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = clipForPrompt(value, maxRunes)
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func composeImproveToolEvidence(promptInput composeAgentSDKImproveVolumePromptInput, applyPatches bool) ToolEvidenceRequirement {
	evidence := ToolEvidenceRequirement{
		MinContextQueryCalls: 1,
		MinCheckCalls:        1,
		RequireNoDeniedTools: true,
	}
	evidence.RequiredToolCommands = append(evidence.RequiredToolCommands, promptInputRequiredFocusedChecks(promptInput)...)
	if !composeImproveHasFocusedTasks(promptInput) || composeImproveUsesCleanProbe(promptInput) {
		evidence.MaxQueryCalls = 2
		evidence.MaxContextQueryCalls = 2
		evidence.DisallowQueryFullCalls = true
	}
	if applyPatches {
		evidence.RequirePatchApplyFollowupCheck = true
	}
	return evidence
}

func promptInputRequiredFocusedChecks(promptInput composeAgentSDKImproveVolumePromptInput) []string {
	const prefix = "Required focused checks before final JSON: "
	for _, instruction := range promptInput.Instructions {
		if !strings.HasPrefix(instruction, prefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(instruction, prefix))
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ";")
		checks := make([]string, 0, len(parts))
		for _, part := range parts {
			if check := strings.TrimSpace(part); check != "" {
				checks = append(checks, check)
			}
		}
		return checks
	}
	return nil
}

func (a *ComposeAgent) RepairGlobalIssuesWithAgentSDK(ctx context.Context, review models.ReviewResult, applyPatches bool) (models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Global Outline Repair")
	if applyPatches {
		logger.Info("Agent apply enabled: SDK may write global fixes through validated setup/outline patch tools")
	}
	promptInput := composeAgentSDKGlobalRepairPromptInput{
		ReviewResult:     compactReviewForAgentSDKPrompt(review, ""),
		ApplyPatches:     applyPatches,
		ForceIssueRepair: true,
		RequiredQueries: []string{
			"novelgen tool query context --type outline-global-repair --view index",
		},
		Instructions: []string{
			"Use the required outline-global-repair index query first. It returns check summary, issue_context, patch_task, navigation, workflow, next_actions, and stats without full setup/outline.",
			"If patch_task is present, use patch_task.dry_run_command and patch_task.apply_command exactly. Do not run repair_context_query, outline-volume, outline-repair, chapter-repair, query outline, query chapter, source/RPG reads, or Claude tool-results reads.",
			"Follow patch_task exactly: run dry_run_command once, run apply_command once only if dry-run succeeds, then patch_task.post_patch_check_query exactly.",
			"If patch_task is absent, fall back to issue_context. Patch only entries that have both patch_query and patch_shape; patch_query may target setup or a specific outline volume.",
			"When patch_task is absent, run at most one repair_context_query, and only the exact repair_context_query from the first patchable issue_context item. Do not invent combined category names such as logic,plot,structure.",
			"Do not patch broad global issues that lack patch_shape. Return them as remaining diagnostics or route them to a smaller workflow.",
			"Patch at most one item in this invocation. When patch_task has dry_run_command/apply_command, do not reconstruct the command; run those exact commands. If falling back to issue_context, dry-run exactly once with `printf '%s' '<compact-json>' | <patch_query>` using a concrete compact JSON object; do not run placeholder text such as `<json>`.",
			"For global outline/setup repair, do not use --patch-json. For Chinese/non-ASCII patch JSON, do not run Python/Node/PowerShell/help commands to encode it.",
			"If apply_patches=true and dry-run succeeds, repeat the same stdin-piped patch with --apply, then run the provided post_patch_check_query exactly; do not replace it with a volume-level or medium-only check.",
			"Do not inspect source code, RPG files, full setup, full outline, or all chapters.",
		},
	}
	requiredPostPatchChecks := composeGlobalRepairRequiredPostPatchChecks(review)
	if len(requiredPostPatchChecks) > 0 {
		promptInput.Instructions = append(promptInput.Instructions, "Required post-patch checks before final JSON: "+strings.Join(requiredPostPatchChecks, "; "))
	}
	params := composeAgentSDKParams("repair global outline issues using project query/check/patch tools", "outline-global-repair-workflow", 18, composeGlobalRepairToolAllowlistForReview(review, applyPatches))
	params.ToolEvidence = ToolEvidenceRequirement{
		MinContextQueryCalls:   1,
		MinCheckCalls:          1,
		MaxContextQueryCalls:   2,
		DisallowQueryFullCalls: true,
		RequireNoDeniedTools:   true,
		RequiredToolCommands:   requiredPostPatchChecks,
	}
	if applyPatches {
		params.ToolEvidence.RequirePatchApplyFollowupCheck = true
	}

	var output composeAgentSDKGlobalRepairOutput
	runtimeResult, err := a.base.ExecuteWithRuntimeResult(ctx, params, promptInput, &output)
	if err != nil {
		if recovered, recoverErr := recoverAppliedGlobalAfterAgentSDKError(runtimeResult, err, requiredPostPatchChecks); recoverErr == nil {
			logger.Warn("Agent SDK global repair ended with an error after applying a validated patch; recovered saved project state: %v", err)
			return recovered, nil
		}
		return models.ReviewResult{}, err
	}
	if err := validateComposeImproveAgentSDKPatchApplyEvidence(runtimeResult, output.AppliedPatches, output.AppliedPatchCount); err != nil {
		return models.ReviewResult{}, err
	}
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return models.ReviewResult{}, fmt.Errorf("agent SDK global repair output has empty review_result")
	}
	result := output.ReviewResult.toModelReviewResult()
	result.NormalizeScoreScale()
	return result, nil
}

func recoverAppliedGlobalAfterAgentSDKError(runtimeResult *agentruntime.Result, invokeErr error, requiredPostPatchChecks []string) (models.ReviewResult, error) {
	if runtimeResult == nil || runtimeResult.LiveSummary == nil {
		return models.ReviewResult{}, invokeErr
	}
	summary := runtimeResult.LiveSummary
	if summary.PatchApplies == 0 || summary.CheckCalls == 0 || summary.ApplyWithoutFollowupCheck > 0 {
		return models.ReviewResult{}, invokeErr
	}
	if len(requiredPostPatchChecks) > 0 && !liveSummaryHasAnyToolCommand(summary, requiredPostPatchChecks) {
		return models.ReviewResult{}, invokeErr
	}
	return models.ReviewResult{
		OverallScore: 80,
		Summary:      fmt.Sprintf("Agent SDK applied %d validated global patch(es) and completed follow-up checks, but ended with tool evidence diagnostics; recovered saved project state.", summary.PatchApplies),
		Suggestions: []models.ReviewSuggestion{{
			Category:   "agent_sdk_recovery",
			Issue:      "Agent SDK attempted disallowed or extra tools after a validated global patch.",
			Suggestion: "Recovered the saved project state because patch apply and follow-up check evidence were present.",
			Priority:   models.PriorityLow,
		}},
	}, nil
}

func liveSummaryHasAnyToolCommand(summary *agentruntime.LiveSummary, commands []string) bool {
	for _, command := range commands {
		if liveSummaryHasToolCommand(summary, command) {
			return true
		}
	}
	return false
}

func validateComposeImproveAgentSDKPatchApplyEvidence(result *agentruntime.Result, appliedPatches bool, appliedPatchCount int) error {
	if result == nil || result.LiveSummary == nil {
		return nil
	}
	summary := result.LiveSummary
	if appliedPatches && summary.PatchApplies == 0 {
		return fmt.Errorf("agent SDK reported applied outline patches but live log has no patch apply call")
	}
	if appliedPatchCount > summary.PatchApplies {
		return fmt.Errorf("agent SDK reported %d applied outline patches but live log has %d patch apply call(s)", appliedPatchCount, summary.PatchApplies)
	}
	return nil
}

func (a *ComposeAgent) recoverAppliedVolumeAfterAgentSDKError(original models.Volume, runtimeResult *agentruntime.Result, invokeErr error) (ComposeAgentSDKImproveVolumeOutput, error) {
	if runtimeResult == nil || runtimeResult.LiveSummary == nil {
		return ComposeAgentSDKImproveVolumeOutput{}, invokeErr
	}
	summary := runtimeResult.LiveSummary
	if summary.PatchApplies == 0 || summary.ApplyWithoutFollowupCheck > 0 {
		return ComposeAgentSDKImproveVolumeOutput{}, invokeErr
	}
	volume, err := loadAgentSDKAppliedOutlineVolume(original, true)
	if err != nil {
		return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("recover applied Agent SDK outline patch: %w; original error: %v", err, invokeErr)
	}
	for i, chapter := range volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeAgentSDKImproveVolumeOutput{}, fmt.Errorf("recovered chapter %d invalid: %w; original error: %v", i+1, err, invokeErr)
		}
	}
	review := models.ReviewResult{
		OverallScore: 80,
		Summary:      fmt.Sprintf("Agent SDK applied %d validated outline patch(es) and completed follow-up checks, but ended before final JSON; recovered saved outline.", summary.PatchApplies),
		Suggestions: []models.ReviewSuggestion{{
			Category:   "agent_sdk_recovery",
			Issue:      "Agent SDK ended before returning final JSON.",
			Suggestion: "Recovered the saved outline because patch apply and follow-up check evidence were present.",
			Priority:   models.PriorityLow,
		}},
	}
	return ComposeAgentSDKImproveVolumeOutput{ReviewResult: review, Volume: volume}, nil
}

func validateAgentSDKApplyOutput(output composeAgentSDKApplyOutput) error {
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return fmt.Errorf("agent SDK apply output has empty review_result")
	}
	if output.AppliedPatchCount < 0 {
		return fmt.Errorf("agent SDK apply output has negative applied_patch_count")
	}
	if output.AppliedPatchCount > 1 {
		return fmt.Errorf("agent SDK apply output reports applied_patch_count=%d, want at most 1 per invocation", output.AppliedPatchCount)
	}
	if !output.AppliedPatches && output.AppliedPatchCount > 0 {
		return fmt.Errorf("agent SDK apply output reports applied_patch_count=%d while applied_patches=false", output.AppliedPatchCount)
	}
	return nil
}

func loadAgentSDKAppliedOutlineVolume(original models.Volume, requireApplied bool) (models.Volume, error) {
	root := agentWorkspaceRoot()
	if strings.TrimSpace(root) == "" {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but workspace root is unknown")
		}
		return original, nil
	}
	outlinePath := filepath.Join(root, "story", "compose", "outline.json")
	outline, err := models.LoadOutline(outlinePath)
	if err != nil {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but reloading %s failed: %w", outlinePath, err)
		}
		return original, nil
	}
	volume := outline.GetVolumeByID(original.ID)
	if volume == nil {
		if requireApplied {
			return models.Volume{}, fmt.Errorf("agent SDK applied outline patch but target volume %q is missing from %s", original.ID, outlinePath)
		}
		return original, nil
	}
	loaded, err := cloneVolumeForAgentSDKPatch(*volume)
	if err != nil {
		return models.Volume{}, err
	}
	preserveImprovedVolumeIdentity(&original, &loaded)
	return loaded, nil
}

func validateAgentSDKImprovePatchOutput(output composeAgentSDKImprovePatchOutput, targetVolumeID string) error {
	targetVolumeID = strings.TrimSpace(targetVolumeID)
	if targetVolumeID == "" {
		return fmt.Errorf("target volume id is empty")
	}
	if strings.TrimSpace(output.VolumePatch.ID) == "" {
		return fmt.Errorf("agent SDK improve output missing volume_patch.id for target volume %q", targetVolumeID)
	}
	if strings.TrimSpace(output.VolumePatch.ID) != targetVolumeID {
		return fmt.Errorf("agent SDK improve output volume_patch.id %q does not match target volume %q", output.VolumePatch.ID, targetVolumeID)
	}
	if output.ReviewResult.OverallScore == 0 &&
		strings.TrimSpace(output.ReviewResult.Summary) == "" &&
		len(output.ReviewResult.Suggestions) == 0 {
		return fmt.Errorf("agent SDK improve output has empty review_result")
	}
	return nil
}

func (r composeAgentSDKReviewResult) toModelReviewResult() models.ReviewResult {
	result := models.ReviewResult{
		OverallScore: r.OverallScore,
		Summary:      clipForPrompt(r.Summary, 500),
	}
	for _, suggestion := range r.Suggestions {
		result.Suggestions = append(result.Suggestions, models.ReviewSuggestion{
			Category:   strings.TrimSpace(suggestion.Category),
			TargetID:   strings.TrimSpace(suggestion.TargetID),
			TargetName: clipForPrompt(suggestion.TargetName, 120),
			Issue:      clipForPrompt(suggestion.Issue, 360),
			Suggestion: clipForPrompt(suggestion.Suggestion, 420),
			Priority:   strings.TrimSpace(suggestion.Priority),
		})
		if len(result.Suggestions) >= 8 {
			break
		}
	}
	return result
}

func applyAgentSDKVolumePatch(original models.Volume, patch composeVolumePatch, expectedChapters int) (models.Volume, error) {
	if err := utils.ValidateNoSuspiciousPatchText(patch); err != nil {
		return models.Volume{}, fmt.Errorf("agent SDK volume patch rejected: %w", err)
	}
	if id := strings.TrimSpace(patch.ID); id != "" && id != strings.TrimSpace(original.ID) {
		return models.Volume{}, fmt.Errorf("agent SDK volume patch id %q does not match target volume %q", id, original.ID)
	}
	if expectedChapters > 0 && len(original.Chapters) != expectedChapters {
		return models.Volume{}, fmt.Errorf("target volume has %d chapter(s), expected %d", len(original.Chapters), expectedChapters)
	}

	merged, err := cloneVolumeForAgentSDKPatch(original)
	if err != nil {
		return models.Volume{}, err
	}
	if strings.TrimSpace(patch.Title) != "" {
		merged.Title = patch.Title
	}
	if strings.TrimSpace(patch.Summary) != "" {
		merged.Summary = patch.Summary
	}
	if !isEmptyPayoffContract(patch.PayoffContract) {
		merged.PayoffContract = models.MergeVolumePayoffContract(merged.PayoffContract, patch.PayoffContract)
	}

	chapterByID := make(map[string]int, len(merged.Chapters))
	for i, chapter := range merged.Chapters {
		chapterByID[strings.TrimSpace(chapter.ID)] = i
	}
	for _, changed := range patch.Chapters {
		id := strings.TrimSpace(changed.ID)
		if id == "" {
			return models.Volume{}, fmt.Errorf("agent SDK volume patch includes a changed chapter without id")
		}
		idx, ok := chapterByID[id]
		if !ok {
			return models.Volume{}, fmt.Errorf("agent SDK volume patch references unknown chapter id %q", id)
		}
		merged.Chapters[idx] = mergeAgentSDKChapterPatch(merged.Chapters[idx], changed)
	}
	if err := applyAgentSDKEventPatches(&merged, patch.Events); err != nil {
		return models.Volume{}, err
	}
	if expectedChapters > 0 && len(merged.Chapters) != expectedChapters {
		return models.Volume{}, fmt.Errorf("merged volume has %d chapter(s), expected %d", len(merged.Chapters), expectedChapters)
	}
	preserveImprovedVolumeIdentity(&original, &merged)
	return merged, nil
}

func applyAgentSDKEventPatches(volume *models.Volume, patches []composeChangedEventPatch) error {
	if len(patches) == 0 {
		return nil
	}
	chapterByID := make(map[string]int, len(volume.Chapters))
	for i, chapter := range volume.Chapters {
		chapterByID[strings.TrimSpace(chapter.ID)] = i
	}
	for _, patch := range patches {
		chapterID := strings.TrimSpace(patch.ChapterID)
		if chapterID == "" {
			return fmt.Errorf("agent SDK changed_events includes an event without chapter_id")
		}
		chapterIdx, ok := chapterByID[chapterID]
		if !ok {
			return fmt.Errorf("agent SDK changed_events references unknown chapter id %q", chapterID)
		}
		if patch.EventIndex < 0 || patch.EventIndex >= len(volume.Chapters[chapterIdx].Events) {
			return fmt.Errorf("agent SDK changed_events index %d out of range for chapter %q", patch.EventIndex, chapterID)
		}
		volume.Chapters[chapterIdx].Events[patch.EventIndex] = mergeAgentSDKEventPatch(volume.Chapters[chapterIdx].Events[patch.EventIndex], patch)
	}
	return nil
}

func mergeAgentSDKEventPatch(original models.Event, patch composeChangedEventPatch) models.Event {
	merged := original
	if strings.TrimSpace(patch.Type) != "" {
		merged.Type = patch.Type
	}
	if len(patch.Characters) > 0 {
		merged.Characters = patch.Characters
	}
	if strings.TrimSpace(patch.Subject) != "" {
		merged.Subject = patch.Subject
	}
	if strings.TrimSpace(patch.Change) != "" {
		merged.Change = patch.Change
	}
	if strings.TrimSpace(patch.Details) != "" {
		merged.Details = patch.Details
	}
	if strings.TrimSpace(patch.Actor) != "" {
		merged.Actor = patch.Actor
	}
	if strings.TrimSpace(patch.Action) != "" {
		merged.Action = patch.Action
	}
	if strings.TrimSpace(patch.Target) != "" {
		merged.Target = patch.Target
	}
	if strings.TrimSpace(patch.TargetType) != "" {
		merged.TargetType = patch.TargetType
	}
	if strings.TrimSpace(patch.Context) != "" {
		merged.Context = patch.Context
	}
	if strings.TrimSpace(patch.Result) != "" {
		merged.Result = patch.Result
	}
	return merged
}

func mergeAgentSDKChapterPatch(original models.Chapter, patch composeChapterPatch) models.Chapter {
	merged := original
	merged.ID = original.ID
	if strings.TrimSpace(patch.Title) != "" {
		merged.Title = patch.Title
	}
	if strings.TrimSpace(patch.Summary) != "" {
		merged.Summary = patch.Summary
	}
	if len(patch.Characters) > 0 {
		merged.Characters = patch.Characters
	}
	if strings.TrimSpace(patch.Location) != "" {
		merged.Location = patch.Location
	}
	if len(patch.Events) > 0 {
		merged.Events = composeEventPatchesToModelEvents(patch.Events)
	}
	if len(patch.Scenes) > 0 {
		merged.Scenes = mergeOutlineScenesByOrder(merged.Scenes, patch.Scenes)
	}
	if strings.TrimSpace(patch.OpeningBeat) != "" {
		merged.OpeningBeat = patch.OpeningBeat
	}
	if strings.TrimSpace(patch.ClosingBeat) != "" {
		merged.ClosingBeat = patch.ClosingBeat
	}
	if strings.TrimSpace(patch.StateChange) != "" {
		merged.StateChange = patch.StateChange
	}
	if strings.TrimSpace(patch.Conflict) != "" {
		merged.Conflict = patch.Conflict
	}
	if strings.TrimSpace(patch.Pacing) != "" {
		merged.Pacing = patch.Pacing
	}
	if !reflect.DeepEqual(patch.Timeline, models.ChapterTimeline{}) {
		merged.Timeline = patch.Timeline
	}
	if !reflect.DeepEqual(patch.StateAnchor, models.StateAnchor{}) {
		merged.StateAnchor = patch.StateAnchor
	}
	if len(patch.Enemies) > 0 {
		merged.Enemies = patch.Enemies
	}
	if len(patch.ResourceLedger) > 0 {
		merged.ResourceLedger = patch.ResourceLedger
	}
	if !reflect.DeepEqual(patch.Mysteries, models.ChapterMysteries{}) {
		merged.Mysteries = patch.Mysteries
	}
	if len(patch.StorylineAdvances) > 0 {
		merged.StorylineAdvances = patch.StorylineAdvances
	}
	if patch.ChapterPayoff != nil && !patch.ChapterPayoff.IsZero() {
		merged.ChapterPayoff = patch.ChapterPayoff
	}
	return merged
}

// mergeOutlineScenesByOrder overlays patch scenes onto the original list by
// scene order: an existing scene with the same order is replaced, and new
// orders are appended. Partial scene lists therefore never truncate chapters
// whose other scenes the agent did not touch.
func mergeOutlineScenesByOrder(original, patch []models.OutlineScene) []models.OutlineScene {
	if len(patch) == 0 {
		return original
	}
	out := append([]models.OutlineScene(nil), original...)
	indexByOrder := map[int]int{}
	for i, scene := range out {
		indexByOrder[scene.Order] = i
	}
	for _, scene := range patch {
		if scene.Order <= 0 {
			continue
		}
		if idx, ok := indexByOrder[scene.Order]; ok {
			out[idx] = scene
			continue
		}
		indexByOrder[scene.Order] = len(out)
		out = append(out, scene)
	}
	return out
}

func composeEventPatchesToModelEvents(events []composeEventPatch) []models.Event {
	result := make([]models.Event, 0, len(events))
	for _, event := range events {
		result = append(result, models.Event{
			Type:       event.Type,
			Characters: event.Characters,
			Subject:    event.Subject,
			Change:     event.Change,
			Details:    event.Details,
			Actor:      event.Actor,
			Action:     event.Action,
			Target:     event.Target,
			TargetType: event.TargetType,
			Context:    event.Context,
			Result:     event.Result,
		})
	}
	return result
}

func prepareAgentSDKReturnedVolume(original models.Volume, returned models.Volume, expectedChapters int) (models.Volume, error) {
	if expectedChapters > 0 {
		if len(returned.Chapters) > expectedChapters {
			returned.Chapters = returned.Chapters[:expectedChapters]
		}
		if len(returned.Chapters) != expectedChapters {
			return models.Volume{}, fmt.Errorf("agent SDK returned %d chapter(s), expected %d", len(returned.Chapters), expectedChapters)
		}
	}
	merged, err := cloneVolumeForAgentSDKPatch(original)
	if err != nil {
		return models.Volume{}, err
	}
	if strings.TrimSpace(returned.Title) != "" {
		merged.Title = returned.Title
	}
	if strings.TrimSpace(returned.Summary) != "" {
		merged.Summary = returned.Summary
	}
	if !isEmptyPayoffContract(returned.PayoffContract) {
		merged.PayoffContract = returned.PayoffContract
	}
	merged.Chapters = returned.Chapters
	preserveImprovedVolumeIdentity(&original, &merged)
	return merged, nil
}

func cloneVolumeForAgentSDKPatch(volume models.Volume) (models.Volume, error) {
	data, err := json.Marshal(volume)
	if err != nil {
		return models.Volume{}, fmt.Errorf("clone agent SDK volume: %w", err)
	}
	var cloned models.Volume
	if err := json.Unmarshal(data, &cloned); err != nil {
		return models.Volume{}, fmt.Errorf("clone agent SDK volume: %w", err)
	}
	return cloned, nil
}

func isEmptyPayoffContract(payoff *models.VolumePayoffContract) bool {
	return payoff == nil || payoff.IsZero()
}

func buildAgentSDKChaptersPromptInput(input ComposeChaptersInput) composeAgentSDKChaptersPromptInput {
	required := []string{
		"novelgen tool query context --type outline-volume --id " + input.Volume.ID + " --view brief",
	}
	previousID := ""
	if input.PreviousVolume != nil && strings.TrimSpace(input.PreviousVolume.ID) != "" {
		previousID = input.PreviousVolume.ID
	}
	return composeAgentSDKChaptersPromptInput{
		TargetPartID:     input.Part.ID,
		TargetVolumeID:   input.Volume.ID,
		TargetVolumeName: input.Volume.Title,
		VolumeIndex:      input.VolumeIndex,
		TotalVolumes:     input.TotalVolumes,
		ChaptersPerVol:   input.ChaptersPerVol,
		PreviousVolumeID: previousID,
		RequiredQueries:  required,
		Instructions: []string{
			"Use the required context query to fetch compact story setup, target volume, neighboring volumes, entity index, events, and navigation.",
			"After the brief queries, use targeted detail queries only when needed: chapter by id, events by chapter id, craft by exact name.",
			"Do not ask for, inspect, or modify source files.",
			"Return exactly one volume object with exactly chapters_per_volume chapters.",
			"Do not invent or change project-wide facts that are absent from tool query results.",
		},
	}
}

func buildAgentSDKImproveVolumePromptInput(input ComposeImproveVolumeInput, applyPatches bool, forceIssueRepair bool) composeAgentSDKImproveVolumePromptInput {
	required := []string{
		"novelgen tool query context --type outline-volume --id " + input.Volume.ID + " --view index",
	}
	crossVolume := len(input.CrossVolumeIDs) > 0
	hasFocusedTasks := len(input.ReviewResult.Suggestions) > 0 || strings.TrimSpace(input.UserPrompt) != ""
	instructions := []string{
		"Use the required context query before reviewing. It returns an index-sized target volume route, check summary, navigation, workflow, next_actions, and stats.",
		"If review_result.suggestions contain navigation, execute repair_route_query first when present. It returns an index-sized route and next_actions. Execute repair_context_query only when the route indicates detailed facts are needed. If route/context are absent, execute detail_queries or focused_check_query before broadening context; treat these focused commands as the primary task list.",
		"If review_result contains suggestions without navigation, query only the referenced target IDs unless more context is necessary.",
	}
	if crossVolume {
		instructions = append(instructions,
			fmt.Sprintf("CROSS-VOLUME MODE: this session may query, check, and patch the target volume %s AND all listed cross-volume volumes: %v. Cross-volume issues (e.g. a volume-end hook that must be resolved at the start of the next volume, a setup planted in one volume and paid off in another, consistency drift across volumes) should be fixed consistently across the involved volumes IN THIS SAME SESSION, not deferred. Use `novelgen tool query outline --type refs --entity-type storyline|character|item|location --name \"<name>\" --view brief` to trace lines across volumes when needed. Every patched volume still needs a check command run for it (volume or chapter scope) before finishing.", input.Volume.ID, strings.Join(input.CrossVolumeIDs, ",")),
		)
	} else {
		instructions = append(instructions,
			"Single-volume mode: only the target_volume_id may be queried, checked, and patched. Do not patch other volumes.",
		)
	}
	if strings.TrimSpace(input.UserPrompt) != "" {
		userPromptInstruction := "Treat user_prompt as already scoped to target_volume_id. If it refers to a multi-volume user request, this invocation must execute only the target_volume_id portion. Do not query, check, or patch any other volume."
		if crossVolume {
			userPromptInstruction = "Treat user_prompt as scoped to the target volume plus the listed cross-volume volumes. Multi-volume user requests should be executed across the involved volumes in this session."
		}
		instructions = append(instructions,
			userPromptInstruction,
			"user_prompt requests are explicit tasks. If they describe concrete narrative changes (character motivation, scene beats, foreshadowing, wording, mystery clues), implement them in volume_patch fields (summary/opening_beat/scenes/events/mysteries) even when scoped checks are clean. Do not decline creative changes as 'not check-verifiable'; the user request is the task list.",
		)
		if composePromptRequestsCheckFirstNoop(input.UserPrompt) {
			instructions = append(instructions,
				"The user_prompt asks for a check-first/no-op-if-clean run. After the required index query, run the target_volume_id check before any brief/full detail query. If that check returns zero issues, return final JSON immediately; do not run extra context just to keep reviewing.",
			)
		}
	}
	if !hasFocusedTasks {
		noTaskInstruction := "No focused review suggestions or user repair request were supplied. In this case, run the required context query, at most one same-volume brief context query if needed, and the required medium+ volume check; do not run low-priority broad checks, do not run full context, and do not sweep chapter-by-chapter."
		if applyPatches {
			noTaskInstruction += " If the medium+ check is clean, return applied_patches=false and applied_patch_count=0."
		} else {
			noTaskInstruction += " If the medium+ check is clean, return an empty patch."
		}
		instructions = append(instructions,
			noTaskInstruction,
		)
	}
	if forceIssueRepair {
		instructions = append(instructions,
			"Treat review_result.suggestions as an explicit repair task list even when they are low priority. Build and validate a minimal patch for the directly fixable issues you can verify from focused tool results.",
			"Do not dismiss low-priority focused issues only because the medium+ scoped check passes. If you decide a focused issue is a false positive, cite the exact queried facts in review_result.suggestions and leave it as a remaining false-positive note instead of saying it was fixed.",
			"When apply_patches=true and at least one focused issue is directly fixable, prefer a small dry-run/apply/check cycle over returning an empty patch.",
		)
	}
	detailInstruction := "If a focused issue, user prompt, or failed medium+ check requires more facts, use targeted detail queries only when needed: repair route/context first, then chapter by id, events by chapter id, or craft by exact name."
	fieldSyncInstruction := "FIELD-SYNC RULE: when a change affects a chapter's narrative (a character's fate, a timeline/date, a fight outcome, an object's species/name, a death, a mystery payoff), apply it consistently to EVERY reader-visible field of that chapter and its volume: summary, opening_beat, scenes[*].beats, events[*].details/result, chapter_payoff (clever_move/reward/hook), state_change, state_anchor notes, AND the volume-level summary/payoff_contract if it references the same fact. A chapter whose summary says one thing while its events say another is a continuity bug, not a patch. After patching, re-check the patched chapters' events/beats for stale references to the old fact (e.g. a character who died in the summary but is still alive in events) and fix them in the same session."
	instructions = append(instructions, fieldSyncInstruction)
	if hasFocusedTasks {
		detailInstruction = "After the index query, use targeted detail queries only when needed: same target volume via `novelgen tool query context --type outline-volume --id <target_volume_id> --view brief`, repair context, chapter by id, events by chapter id, craft by exact name."
	}
	if !crossVolume {
		if boundaryIDs := composeImproveBoundaryChapterIDs(input.Volume, input.UserPrompt); len(boundaryIDs) > 0 {
			instructions = append(instructions,
				fmt.Sprintf("The user prompt is boundary-scoped. Prefer direct chapter/event detail queries only for these target chapter IDs: %s. Query other chapters only when a check result or next_actions explicitly names them.", strings.Join(boundaryIDs, ", ")),
				"For a boundary-scoped chapter summary/title/beat patch, do not query story-setup, character refs, cross-volume context, or unrelated chapters. Use the target volume index, target volume brief, target chapter detail, and required focused check as sufficient evidence unless next_actions names a narrower repair context.",
				"If the user prompt names a character, item, location, skill, or proper noun that is absent from the target chapter/volume facts, verify it once with an exact refs or story-setup search. If both focused facts and the verification query show no match, do not patch; return a compact review_result note that the user prompt conflicts with project facts.",
			)
			if checks := composeImproveRequiredFocusedChecks(input.Volume, input.UserPrompt); len(checks) > 0 {
				instructions = append(instructions,
					"Required focused checks before final JSON: "+strings.Join(checks, "; "),
				)
			}
		}
	}
	instructions = append(instructions,
		detailInstruction,
		"If continuity facts require cross-volume context, you may read the payoff_contract/summary of the immediately adjacent volumes only (the allowed query list includes them). Do not query non-adjacent volumes and never patch an adjacent volume.",
		"Review and, only when needed, improve target_volume_id. Do not change any other volume.",
		composeAgentSDKPatchInstruction(applyPatches),
		composeAgentSDKCheckInstruction(applyPatches),
		composeAgentSDKReturnInstruction(applyPatches),
		"Keep review_result compact: summary under 500 Chinese characters and at most 8 suggestions, focused on remaining issues or applied patch targets.",
		composeAgentSDKWriteInstruction(applyPatches),
	)
	return composeAgentSDKImproveVolumePromptInput{
		TargetPartID:     input.Part.ID,
		TargetVolumeID:   input.Volume.ID,
		TargetVolumeName: input.Volume.Title,
		ChapterCount:     len(input.Volume.Chapters),
		ReviewResult:     compactReviewForAgentSDKPrompt(input.ReviewResult, input.Volume.ID),
		UserPrompt:       input.UserPrompt,
		ApplyPatches:     applyPatches,
		ForceIssueRepair: forceIssueRepair,
		RequiredQueries:  required,
		Instructions:     instructions,
	}
}

func composeAgentSDKPatchInstruction(applyPatches bool) string {
	const jsonInput = "For Chinese/non-ASCII patch JSON, do not use --patch-json and do not run Python/Node/PowerShell/help commands to encode it. Pipe compact literal JSON on stdin instead: `printf '%s' '<compact-json>' | novelgen tool patch outline --target volume --id <target_volume_id>`. Use --patch-json only for small ASCII-only patches."
	if applyPatches {
		return "If you build a non-empty candidate patch JSON, first dry-run it with `printf '%s' '<compact-json>' | novelgen tool patch outline --target volume --id <target_volume_id>`; only after a successful dry-run, repeat the same stdin-piped patch command with `--apply`. Apply at most one successful outline volume patch in this invocation. " + jsonInput
	}
	return "If you build a non-empty volume_patch, dry-run it with `printf '%s' '<compact-json>' | novelgen tool patch outline --target volume --id <target_volume_id>` and do not use --apply. " + jsonInput
}

func composeAgentSDKCheckInstruction(applyPatches bool) string {
	if applyPatches {
		return "After a successful --apply, run `novelgen tool check all --target outline --scope volume --id <target_volume_id> --min-priority medium --max-issues 12`, summarize remaining issues, and return final JSON. Do not start another dry-run/apply cycle in the same invocation."
	}
	return "Use the quality/simulation result embedded in patch dry-run output as the validation signal."
}

func composeAgentSDKReturnInstruction(applyPatches bool) string {
	if applyPatches {
		return "Return only review_result, applied_patches, applied_patch_count, and final_check. Do not return volume_patch; Go reloads the saved outline after validated tool apply."
	}
	return "Return review_result and volume_patch. volume_patch.changed_chapters must contain only chapters you actually changed, and may include only changed fields plus the chapter id. Use volume_patch.changed_events for individual event field changes with chapter_id and 0-based event_index. If no changes are needed, return empty changed_chapters/changed_events arrays instead of echoing the whole volume."
}

func composeAgentSDKWriteInstruction(applyPatches bool) string {
	if applyPatches {
		return "Do not inspect source code or write files directly; do not read Claude tool-results temporary files. If you need more detail, rerun a narrower `novelgen tool query/check`. The only allowed write is `novelgen tool patch outline --target volume ... --apply` after dry-run validation."
	}
	return "Do not ask for, inspect, or modify source files. Go will merge and save the returned patch."
}

// Generate creates a story outline from setup and structure
// This is the type-safe wrapper around BaseAgent.Execute
func (a *ComposeAgent) Generate(ctx context.Context, input ComposeGenInput) (ComposeGenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Generation")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes × %d chapters",
		input.Structure.TargetParts, input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Language: %s", a.base.language)

	var output ComposeGenOutput
	params := InvokeParams{
		Skills:  []string{"compose-gen"},
		Command: "generate a story outline with the specified structure",
	}

	promptInput := composeGenPromptInput{
		SetupBrief: a.buildSetupBrief(&input.Setup),
		Structure:  input.Structure,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate the outline structure
	if err := a.validateOutlineStructure(&output.Outline, input.Structure); err != nil {
		return ComposeGenOutput{}, err
	}

	// Validate chapter anchors and state change mapping
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeGenOutput{}, err
	}

	// Assign IDs to all elements using IDManager
	idManager := logic.NewIDManager(&output.Outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := input.Structure.TotalChapters()
	logger.Info("Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume",
		len(output.Outline.Parts), input.Structure.TargetVolumes, input.Structure.TargetChapters)
	logger.Info("Total: %d chapters", totalChapters)

	return output, nil
}

// Regenerate regenerates a story outline element (part, volume, or chapter)
func (a *ComposeAgent) Regenerate(ctx context.Context, input ComposeRegenInput) (ComposeRegenOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Regeneration")
	logger.Info("Element Type: %s", input.ElementType)
	logger.Info("Element ID: %s", input.ElementID)
	logger.Info("Language: %s", a.base.language)

	params := InvokeParams{
		Skills:  []string{"compose-regen"},
		Command: fmt.Sprintf("regenerate a %s while maintaining continuity", input.ElementType),
	}

	var output ComposeRegenOutput

	switch input.ElementType {
	case "part":
		var part models.Part
		if err := a.base.Execute(ctx, params, input, &part); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Part = &part
		logger.Info("✓ Part regenerated: %s", part.Title)

	case "volume":
		var volume models.Volume
		if err := a.base.Execute(ctx, params, input, &volume); err != nil {
			return ComposeRegenOutput{}, err
		}
		output.Volume = &volume
		logger.Info("✓ Volume regenerated: %s (%d chapters)", volume.Title, len(volume.Chapters))

	case "chapter":
		var chapter models.Chapter
		if err := a.base.Execute(ctx, params, input, &chapter); err != nil {
			return ComposeRegenOutput{}, err
		}
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeRegenOutput{}, fmt.Errorf("validation failed: %w", err)
		}
		output.Chapter = &chapter
		logger.Info("✓ Chapter regenerated: %s", chapter.Title)

	default:
		return ComposeRegenOutput{}, fmt.Errorf("invalid element type: %s", input.ElementType)
	}

	return output, nil
}

// Review reviews an existing outline and provides improvement suggestions
func (a *ComposeAgent) Review(ctx context.Context, input ComposeReviewInput) (ComposeReviewOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Review")
	logger.Info("Language: %s", a.base.language)

	var output ComposeReviewOutput
	params := InvokeParams{
		Skills:  []string{"compose-review"},
		Command: "review the story outline and provide improvement suggestions",
	}

	promptInput := composeReviewPromptInput{
		ExistingOutline: input.ExistingOutline,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		UserPrompt:      input.UserPrompt,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Result); err != nil {
		return ComposeReviewOutput{}, err
	}
	output.Result.NormalizeScoreScale()

	// Log result
	logger.Section("Outline Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// Improve improves an existing outline
func (a *ComposeAgent) Improve(ctx context.Context, input ComposeImproveInput) (ComposeImproveOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Improvement")
	logger.Info("Language: %s", a.base.language)

	var output ComposeImproveOutput
	params := InvokeParams{
		Skills:  []string{"compose-improve"},
		Command: "improve the story outline",
	}

	promptInput := composeImprovePromptInput{
		ExistingOutline: input.ExistingOutline,
		ReviewResult:    compactReviewForPrompt(input.ReviewResult),
		UserPrompt:      input.UserPrompt,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		RevisionContext: input.RevisionContext,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	// Validate the improved outline
	if err := a.validateOutlineChapters(&output.Outline); err != nil {
		return ComposeImproveOutput{}, err
	}

	return output, nil
}

// ReviewSkeleton reviews an outline skeleton without requiring generated chapters.
func (a *ComposeAgent) ReviewSkeleton(ctx context.Context, input ComposeSkeletonReviewInput) (ComposeSkeletonReviewOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Skeleton Review")
	logger.Info("Language: %s", a.base.language)

	var output ComposeSkeletonReviewOutput
	params := InvokeParams{
		Skills:  []string{"compose-skeleton-review"},
		Command: "review the story outline skeleton and provide improvement suggestions",
	}

	promptInput := composeSkeletonReviewPromptInput{
		ExistingOutline: input.ExistingOutline,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		Structure:       input.Structure,
		UserPrompt:      input.UserPrompt,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Result); err != nil {
		return ComposeSkeletonReviewOutput{}, err
	}
	output.Result.NormalizeScoreScale()

	logger.Section("Outline Skeleton Review Result")
	logger.Info("Overall Score: %.1f/100", output.Result.OverallScore)
	for _, dim := range output.Result.Dimensions {
		logger.Info("%s: %.1f/%.0f", dim.Name, dim.Score, dim.Max)
	}
	logger.Info("Summary: %s", output.Result.Summary)
	logger.Info("Strengths: %d items", len(output.Result.Strengths))
	logger.Info("Suggestions: %d items", len(output.Result.Suggestions))

	return output, nil
}

// ImproveSkeleton improves only the part/volume skeleton. Existing chapters are
// preserved, so this is safe to run before or after some volumes have chapters.
func (a *ComposeAgent) ImproveSkeleton(ctx context.Context, input ComposeSkeletonImproveInput) (ComposeSkeletonImproveOutput, error) {
	logger.Section("COMPOSE AGENT - Outline Skeleton Improvement")
	logger.Info("Language: %s", a.base.language)

	var output ComposeSkeletonImproveOutput
	params := InvokeParams{
		Skills:  []string{"compose-skeleton-improve"},
		Command: "improve the story outline skeleton",
	}

	promptInput := composeSkeletonImprovePromptInput{
		ExistingOutline: input.ExistingOutline,
		ReviewResult:    compactReviewForPrompt(input.ReviewResult),
		UserPrompt:      input.UserPrompt,
		SetupBrief:      a.buildSetupBrief(&input.Setup),
		Structure:       input.Structure,
		RevisionContext: input.RevisionContext,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output.Outline); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}

	if err := preserveSkeletonIdentityAndChapters(&input.ExistingOutline, &output.Outline); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}
	models.EnsureVolumeTitleOrdinals(&output.Outline)
	if err := validateOutlineSkeleton(&output.Outline, input.Structure); err != nil {
		return ComposeSkeletonImproveOutput{}, err
	}

	return output, nil
}

// IterateSkeleton runs review/improve loops for a parts/volumes skeleton.
func (a *ComposeAgent) IterateSkeleton(ctx context.Context, outline *models.Outline, setup *models.StorySetup, structure models.StoryStructure, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Skeleton Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	if err := validateOutlineSkeleton(outline, structure); err != nil {
		return nil, nil, err
	}

	currentOutline := *outline
	var finalReview *models.ReviewResult
	session := NewRevisionSession("compose-skeleton", "Improve only outline parts/volumes/payoff contracts before chapter generation.")
	if strings.TrimSpace(userPrompt) != "" {
		session.AddUserGuidance(0, userPrompt)
	}

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Skeleton iteration %d/%d ===", i, maxIterations)
		reviewOutput, err := a.ReviewSkeleton(ctx, ComposeSkeletonReviewInput{
			ExistingOutline: currentOutline,
			Setup:           *setup,
			Structure:       structure,
			UserPrompt:      userPrompt,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("skeleton review failed at iteration %d: %w", i, err)
		}
		reviewOutput.Result.Iteration = i
		session.AddReview(i, reviewOutput.Result)
		finalReview = &reviewOutput.Result

		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing skeleton improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		improveOutput, err := a.ImproveSkeleton(ctx, ComposeSkeletonImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    reviewOutput.Result,
			UserPrompt:      userPrompt,
			Setup:           *setup,
			Structure:       structure,
			RevisionContext: session.Prompt(),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("skeleton improvement failed at iteration %d: %w", i, err)
		}
		currentOutline = improveOutput.Outline
		session.AddImprove(i, "Applied skeleton review feedback while preserving part/volume IDs and existing chapters.")

		if i == maxIterations {
			logger.Warn("Max skeleton iterations reached, stopping iteration loop")
			break
		}
	}

	models.EnsureVolumeTitleOrdinals(&currentOutline)
	return &currentOutline, finalReview, nil
}

// Iterate runs the review-improvement loop for outline
func (a *ComposeAgent) Iterate(ctx context.Context, outline *models.Outline, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}

	currentOutline := *outline
	var finalReview *models.ReviewResult
	session := NewRevisionSession("compose", "Improve the outline through review feedback while preserving existing structure and continuity.")
	if strings.TrimSpace(userPrompt) != "" {
		session.AddUserGuidance(0, userPrompt)
	}

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Review current outline
		reviewInput := ComposeReviewInput{ExistingOutline: currentOutline}
		if setup != nil {
			reviewInput.Setup = *setup
		}
		reviewOutput, err := a.Review(ctx, reviewInput)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}

		reviewOutput.Result.Iteration = i
		session.AddReview(i, reviewOutput.Result)
		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}

		// Determine if we should improve. Blocking suggestions override the
		// numeric score because they represent issues that must still be fixed.
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Improve the outline with review feedback
		improveInput := ComposeImproveInput{
			ExistingOutline: currentOutline,
			ReviewResult:    compactReviewForPrompt(reviewOutput.Result),
			UserPrompt:      userPrompt,
			RevisionContext: session.Prompt(),
		}
		if setup != nil {
			improveInput.Setup = *setup
		}
		improveOutput, err := a.Improve(ctx, improveInput)
		if err != nil {
			return nil, nil, fmt.Errorf("improvement failed at iteration %d: %w", i, err)
		}

		currentOutline = improveOutput.Outline
		session.AddImprove(i, "Applied the previous outline review feedback; next review should verify the repaired chapters and avoid undoing preserved structure.")
		logger.Info("✓ Outline improved based on review suggestions")

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
}

// BuildPartContext builds context for part regeneration
func (a *ComposeAgent) BuildPartContext(part *models.Part, outline *models.Outline) string {
	var context strings.Builder

	// Find part index
	partIdx := -1
	for i, p := range outline.Parts {
		if p.ID == part.ID {
			partIdx = i
			break
		}
	}

	if partIdx > 0 {
		prevPart := outline.Parts[partIdx-1]
		context.WriteString(fmt.Sprintf("Previous Part (%s): %s\nSummary: %s\n\n",
			prevPart.ID, prevPart.Title, prevPart.Summary))
	}

	if partIdx < len(outline.Parts)-1 {
		nextPart := outline.Parts[partIdx+1]
		context.WriteString(fmt.Sprintf("Next Part (%s): %s\nSummary: %s\n\n",
			nextPart.ID, nextPart.Title, nextPart.Summary))
	}

	return context.String()
}

// ImproveVolume improves a specific volume based on review feedback
func (a *ComposeAgent) ImproveVolume(ctx context.Context, input ComposeImproveVolumeInput) (ComposeImproveVolumeOutput, error) {
	logger.Section("COMPOSE AGENT - Volume Improvement")
	logger.Info("Volume: %s", input.Volume.Title)
	logger.Info("Language: %s", a.base.language)

	var output ComposeImproveVolumeOutput
	params := InvokeParams{
		Skills:  []string{"compose-improve-volume"},
		Command: "improve the chapters in this volume based on review feedback",
	}

	promptInput := a.buildImproveVolumePromptInput(input)
	if err := a.base.Execute(ctx, params, promptInput, &output.Volume); err != nil {
		return ComposeImproveVolumeOutput{}, err
	}

	// Validate the improved volume
	if len(output.Volume.Chapters) == 0 {
		return ComposeImproveVolumeOutput{}, fmt.Errorf("improved volume has no chapters")
	}
	if expected := len(input.Volume.Chapters); expected > 0 && len(output.Volume.Chapters) != expected {
		logger.Warn("Improved volume returned %d chapters, expected %d; preserving original chapter slots", len(output.Volume.Chapters), expected)
		if len(output.Volume.Chapters) > expected {
			output.Volume.Chapters = output.Volume.Chapters[:expected]
		}
		for len(output.Volume.Chapters) < expected {
			idx := len(output.Volume.Chapters)
			output.Volume.Chapters = append(output.Volume.Chapters, input.Volume.Chapters[idx])
		}
	}

	// Validate each chapter
	for i, chapter := range output.Volume.Chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeImproveVolumeOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	// Preserve original volume ID and chapter IDs to maintain consistency
	originalVolumeID := input.Volume.ID
	output.Volume.ID = originalVolumeID

	// Preserve original chapter IDs
	for i := range output.Volume.Chapters {
		if i < len(input.Volume.Chapters) {
			output.Volume.Chapters[i].ID = input.Volume.Chapters[i].ID
		}
	}

	logger.Info("✓ Volume improved: %s (%d chapters)", output.Volume.Title, len(output.Volume.Chapters))

	return output, nil
}

func (a *ComposeAgent) buildImproveVolumePromptInput(input ComposeImproveVolumeInput) composeImproveVolumePromptInput {
	return composeImproveVolumePromptInput{
		SetupBrief:     a.buildSetupBrief(&input.Setup),
		OutlineContext: a.buildVolumeImproveContext(&input.Outline, input.Part.ID, input.Volume.ID),
		TargetVolume:   input.Volume,
		ReviewResult:   compactReviewForPrompt(input.ReviewResult),
		UserPrompt:     input.UserPrompt,
	}
}

func (a *ComposeAgent) buildSetupBrief(setup *models.StorySetup) string {
	if setup == nil || setup.ProjectName == "" {
		return "No setup provided."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Project: %s\n", setup.ProjectName))
	if len(setup.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(setup.Genres, ", ")))
	}
	b.WriteString(fmt.Sprintf("Premise: %s\n", clipForPrompt(setup.Premise, 700)))
	b.WriteString(fmt.Sprintf("Theme: %s\n", clipForPrompt(setup.Theme, 350)))
	if setup.Tone != "" {
		b.WriteString(fmt.Sprintf("Tone: %s\n", clipForPrompt(setup.Tone, 160)))
	}

	if setup.LongFormPlan != nil && !setup.LongFormPlan.IsZero() {
		plan := setup.LongFormPlan
		b.WriteString("\nLong Form Plan:\n")
		if plan.TargetChapters > 0 || plan.TargetVolumes > 0 {
			b.WriteString(fmt.Sprintf("- Target scale: %d chapter(s), %d volume(s)\n", plan.TargetChapters, plan.TargetVolumes))
		}
		if plan.MainLoop != "" {
			b.WriteString(fmt.Sprintf("- Main loop: %s\n", clipForPrompt(plan.MainLoop, 260)))
		}
		if len(plan.EscalationLadder) > 0 {
			b.WriteString(fmt.Sprintf("- Escalation ladder: %s\n", joinLimited(plan.EscalationLadder, 8)))
		}
		if len(plan.ReaderPromises) > 0 {
			b.WriteString(fmt.Sprintf("- Reader promises: %s\n", joinLimited(plan.ReaderPromises, 8)))
		}
		if plan.PayoffCadence != "" {
			b.WriteString(fmt.Sprintf("- Payoff cadence: %s\n", clipForPrompt(plan.PayoffCadence, 220)))
		}
		if len(plan.VolumePattern) > 0 {
			b.WriteString(fmt.Sprintf("- Volume pattern: %s\n", joinLimited(plan.VolumePattern, 8)))
		}
		if plan.MidpointMutation != "" || plan.EndgamePromise != "" {
			b.WriteString(fmt.Sprintf("- Mutation/endgame: %s | %s\n", clipForPrompt(plan.MidpointMutation, 180), clipForPrompt(plan.EndgamePromise, 180)))
		}
	}

	if len(setup.Rules) > 0 {
		b.WriteString("\nCore Rules:\n")
		for i, rule := range setup.Rules {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("- ... %d more rule(s)\n", len(setup.Rules)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s\n", clipForPrompt(rule, 360)))
		}
	}

	if len(setup.CoreCast) > 0 {
		b.WriteString("\nCore Cast Seeds:\n")
		for i, seed := range setup.CoreCast {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more cast seed(s)\n", len(setup.CoreCast)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s, importance %d, entry %s): %s\n",
				seed.Name, seed.Role, seed.Importance, seed.EntryPhase, clipForPrompt(seed.StoryFunction, 220)))
			if seed.RelationshipArc != "" || seed.Payoff != "" {
				b.WriteString(fmt.Sprintf("  Arc: %s | Payoff: %s\n",
					clipForPrompt(seed.RelationshipArc, 160), clipForPrompt(seed.Payoff, 180)))
			}
			if len(seed.StorylineRefs) > 0 {
				b.WriteString(fmt.Sprintf("  Storylines: %s\n", strings.Join(seed.StorylineRefs, ", ")))
			}
		}
	}

	if len(setup.Storylines) > 0 {
		b.WriteString("\nStorylines:\n")
		for i, sl := range setup.Storylines {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more storyline(s)\n", len(setup.Storylines)-i))
				break
			}
			scopeNote := compactStorylineScope(sl)
			b.WriteString(fmt.Sprintf("- %s (%s%s, importance %d): %s\n",
				sl.Name, sl.Type, scopeNote, sl.Importance, clipForPrompt(sl.Description, 260)))
			if sl.SetupRole != "" {
				b.WriteString(fmt.Sprintf("  Role: %s\n", clipForPrompt(sl.SetupRole, 160)))
			}
			if sl.RepeatablePressure != "" || sl.PayoffCadence != "" || sl.Mutation != "" {
				b.WriteString(fmt.Sprintf("  Serial engine: pressure=%s | cadence=%s | mutation=%s\n",
					clipForPrompt(sl.RepeatablePressure, 150), clipForPrompt(sl.PayoffCadence, 130), clipForPrompt(sl.Mutation, 150)))
			}
			if sl.FailureMode != "" {
				b.WriteString(fmt.Sprintf("  Avoid failure mode: %s\n", clipForPrompt(sl.FailureMode, 140)))
			}
			if sl.Desire != "" || sl.Opposition != "" || sl.Payoff != "" {
				b.WriteString(fmt.Sprintf("  Desire: %s | Opposition: %s | Payoff%s: %s\n",
					clipForPrompt(sl.Desire, 160), clipForPrompt(sl.Opposition, 160), compactPayoffStyle(sl), clipForPrompt(sl.Payoff, 160)))
			}
			if sl.AppealEngine != nil {
				b.WriteString(formatAppealEngineBrief("  Appeal", sl.AppealEngine))
			}
		}
	}

	if len(setup.Premises) > 0 {
		b.WriteString("\nProgression Systems:\n")
		for i, premise := range setup.Premises {
			if i >= 8 {
				b.WriteString(fmt.Sprintf("- ... %d more progression system(s)\n", len(setup.Premises)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", premise.Name, premise.Category, clipForPrompt(premise.Description, 280)))
			if premise.AppealEngine != nil {
				b.WriteString(formatAppealEngineBrief("  Appeal", premise.AppealEngine))
			}
			for j, stage := range premise.Progression {
				if j >= 8 {
					b.WriteString(fmt.Sprintf("  ... %d more progression stage(s)\n", len(premise.Progression)-j))
					break
				}
				b.WriteString(fmt.Sprintf("  L%d %s: %s Requirement: %s\n",
					stage.Level, stage.Name, clipForPrompt(stage.Description, 130), clipForPrompt(stage.Requirements, 130)))
			}
		}
	}

	if len(setup.WorldResources) > 0 {
		b.WriteString("\nResources:\n")
		for i, res := range setup.WorldResources {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ... %d more resource(s)\n", len(setup.WorldResources)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- %s (%s, scarcity: %s): %s\n",
				res.Name, res.Category, res.Scarcity, clipForPrompt(res.Description, 180)))
		}
	}

	return b.String()
}

func formatAppealEngineBrief(prefix string, engine *models.AppealEngine) string {
	if engine == nil {
		return ""
	}
	var parts []string
	if strings.TrimSpace(engine.Appeal) != "" {
		parts = append(parts, "appeal="+clipForPrompt(engine.Appeal, 120))
	}
	if strings.TrimSpace(engine.SurfaceLimit) != "" {
		parts = append(parts, "surface_limit="+clipForPrompt(engine.SurfaceLimit, 120))
	}
	if strings.TrimSpace(engine.Exploit) != "" {
		parts = append(parts, "exploit="+clipForPrompt(engine.Exploit, 120))
	}
	if strings.TrimSpace(engine.SignatureWin) != "" {
		parts = append(parts, "signature_win="+clipForPrompt(engine.SignatureWin, 120))
	}
	if strings.TrimSpace(engine.UpgradePath) != "" {
		parts = append(parts, "upgrade_path="+clipForPrompt(engine.UpgradePath, 120))
	}
	if strings.TrimSpace(engine.OpponentMisread) != "" {
		parts = append(parts, "opponent_misread="+clipForPrompt(engine.OpponentMisread, 120))
	}
	if strings.TrimSpace(engine.RewardType) != "" {
		parts = append(parts, "reward_type="+clipForPrompt(engine.RewardType, 80))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s\n", prefix, strings.Join(parts, " | "))
}

func compactStorylineScope(sl models.Storyline) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(sl.Scope) != "" {
		parts = append(parts, "scope "+strings.TrimSpace(sl.Scope))
	}
	if strings.TrimSpace(sl.PayoffStyle) != "" {
		parts = append(parts, "payoff "+strings.TrimSpace(sl.PayoffStyle))
	}
	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

func compactPayoffStyle(sl models.Storyline) string {
	if strings.TrimSpace(sl.PayoffStyle) == "" {
		return ""
	}
	return " [" + strings.TrimSpace(sl.PayoffStyle) + "]"
}

func (a *ComposeAgent) buildVolumeImproveContext(outline *models.Outline, partID, volumeID string) string {
	if outline == nil {
		return "No outline context provided."
	}

	var b strings.Builder
	for partIdx, part := range outline.Parts {
		for volIdx, volume := range part.Volumes {
			if volume.ID != volumeID {
				continue
			}

			b.WriteString(fmt.Sprintf("Current Part: %s (%s)\nSummary: %s\n", part.Title, nonEmpty(part.ID, partID), clipForPrompt(part.Summary, 650)))
			b.WriteString(fmt.Sprintf("Current Volume: %s (%s), volume %d/%d\nSummary: %s\n",
				volume.Title, volume.ID, volIdx+1, len(part.Volumes), clipForPrompt(volume.Summary, 650)))

			if volIdx > 0 {
				prevVol := part.Volumes[volIdx-1]
				b.WriteString("\nPrevious Volume:\n")
				b.WriteString(formatVolumeBrief(prevVol))
			}
			if volIdx < len(part.Volumes)-1 {
				nextVol := part.Volumes[volIdx+1]
				b.WriteString("\nNext Volume:\n")
				b.WriteString(formatVolumeBrief(nextVol))
			}

			if len(volume.Chapters) > 0 {
				b.WriteString("\nTarget Volume Chapter Map:\n")
				for _, ch := range volume.Chapters {
					b.WriteString(formatChapterBrief(ch))
				}
			}

			if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				b.WriteString("\nPrevious Part Summary:\n")
				b.WriteString(fmt.Sprintf("- %s: %s\n", prevPart.Title, clipForPrompt(prevPart.Summary, 350)))
			}
			if partIdx < len(outline.Parts)-1 {
				nextPart := outline.Parts[partIdx+1]
				b.WriteString("\nNext Part Summary:\n")
				b.WriteString(fmt.Sprintf("- %s: %s\n", nextPart.Title, clipForPrompt(nextPart.Summary, 350)))
			}

			return b.String()
		}
	}

	return "Target volume was not found in the outline context."
}

func formatVolumeBrief(volume models.Volume) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("- %s (%s): %s\n", volume.Title, volume.ID, clipForPrompt(volume.Summary, 500)))
	if payoff := formatVolumePayoffBrief(volume.PayoffContract); payoff != "" {
		b.WriteString("  Payoff: " + payoff + "\n")
	}
	if len(volume.Chapters) > 0 {
		first := volume.Chapters[0]
		last := volume.Chapters[len(volume.Chapters)-1]
		b.WriteString(fmt.Sprintf("  Opens with: %s - %s\n", first.Title, clipForPrompt(first.Summary, 180)))
		b.WriteString(fmt.Sprintf("  Ends with: %s - %s\n", last.Title, clipForPrompt(last.Summary, 180)))
	}
	return b.String()
}

func formatVolumePayoffBrief(payoff *models.VolumePayoffContract) string {
	if payoff.IsZero() {
		return ""
	}
	var parts []string
	if strings.TrimSpace(payoff.VolumeQuestion) != "" {
		parts = append(parts, "question="+clipForPrompt(payoff.VolumeQuestion, 120))
	}
	if strings.TrimSpace(payoff.PowerPromise) != "" {
		parts = append(parts, "promise="+clipForPrompt(payoff.PowerPromise, 120))
	}
	if strings.TrimSpace(payoff.MainOpponentMisread) != "" {
		parts = append(parts, "misread="+clipForPrompt(payoff.MainOpponentMisread, 120))
	}
	if strings.TrimSpace(payoff.BigWin) != "" {
		parts = append(parts, "big_win="+clipForPrompt(payoff.BigWin, 140))
	}
	if strings.TrimSpace(payoff.VisibleReward) != "" {
		parts = append(parts, "reward="+clipForPrompt(payoff.VisibleReward, 100))
	}
	if strings.TrimSpace(payoff.ReputationShift) != "" {
		parts = append(parts, "reputation="+clipForPrompt(payoff.ReputationShift, 100))
	}
	if strings.TrimSpace(payoff.NextBiggerGame) != "" {
		parts = append(parts, "next="+clipForPrompt(payoff.NextBiggerGame, 120))
	}
	return strings.Join(parts, " | ")
}

func formatChapterBrief(ch models.Chapter) string {
	beats := ch.GetBeats()
	firstBeat := ""
	lastBeat := ""
	if len(beats) > 0 {
		firstBeat = beats[0]
		lastBeat = beats[len(beats)-1]
	}
	var states []string
	if ch.StateChange != "" {
		states = append(states, "state_change="+clipForPrompt(ch.StateChange, 100))
	}
	if ch.StateAnchor.Cultivation != "" {
		states = append(states, "cultivation="+clipForPrompt(ch.StateAnchor.Cultivation, 80))
	}
	if len(ch.StateAnchor.Allies) > 0 {
		states = append(states, "allies="+strings.Join(ch.StateAnchor.Allies, ", "))
	}
	stateLine := strings.Join(states, "; ")
	if stateLine == "" {
		stateLine = "no explicit state anchor"
	}

	payoffLine := ""
	if payoff := formatChapterPayoffBrief(ch.ChapterPayoff); payoff != "" {
		payoffLine = " | payoff: " + payoff
	}

	return fmt.Sprintf("- %s %s: %s | %s%s | first beat: %s | last beat: %s\n",
		ch.ID, ch.Title, clipForPrompt(ch.Summary, 240), stateLine, payoffLine, clipForPrompt(firstBeat, 120), clipForPrompt(lastBeat, 120))
}

func formatChapterPayoffBrief(payoff *models.ChapterPayoff) string {
	if payoff.IsZero() {
		return ""
	}
	var parts []string
	if strings.TrimSpace(payoff.Desire) != "" {
		parts = append(parts, "desire="+clipForPrompt(payoff.Desire, 90))
	}
	if strings.TrimSpace(payoff.Pressure) != "" {
		parts = append(parts, "pressure="+clipForPrompt(payoff.Pressure, 90))
	}
	if strings.TrimSpace(payoff.CleverMove) != "" {
		parts = append(parts, "move="+clipForPrompt(payoff.CleverMove, 100))
	}
	if strings.TrimSpace(payoff.PayoffMoment) != "" {
		parts = append(parts, "moment="+clipForPrompt(payoff.PayoffMoment, 120))
	}
	if strings.TrimSpace(payoff.Reward) != "" {
		parts = append(parts, "reward="+clipForPrompt(payoff.Reward, 90))
	}
	if strings.TrimSpace(payoff.SocialProof) != "" {
		parts = append(parts, "proof="+clipForPrompt(payoff.SocialProof, 90))
	}
	if strings.TrimSpace(payoff.Hook) != "" {
		parts = append(parts, "hook="+clipForPrompt(payoff.Hook, 100))
	}
	return strings.Join(parts, " | ")
}

func compactReviewForPrompt(review models.ReviewResult) models.ReviewResult {
	compact := models.ReviewResult{
		OverallScore: review.OverallScore,
		Dimensions:   review.Dimensions,
		Summary:      clipForPrompt(review.Summary, 700),
		Iteration:    review.Iteration,
	}

	for _, strength := range review.Strengths {
		compact.Strengths = append(compact.Strengths, clipForPrompt(strength, 220))
	}
	for _, weakness := range review.Weaknesses {
		compact.Weaknesses = append(compact.Weaknesses, clipForPrompt(weakness, 260))
	}
	for _, suggestion := range review.Suggestions {
		compact.Suggestions = append(compact.Suggestions, models.ReviewSuggestion{
			Category:   suggestion.Category,
			TargetID:   suggestion.TargetID,
			TargetName: clipForPrompt(suggestion.TargetName, 120),
			Issue:      clipForPrompt(suggestion.Issue, 360),
			Suggestion: clipForPrompt(suggestion.Suggestion, 420),
			Priority:   suggestion.Priority,
		})
	}
	for _, issue := range review.ContinuityIssues {
		compact.ContinuityIssues = append(compact.ContinuityIssues, models.ContinuityIssue{
			Type:        issue.Type,
			Location:    clipForPrompt(issue.Location, 160),
			Description: clipForPrompt(issue.Description, 320),
			Reason:      clipForPrompt(issue.Reason, 260),
			Suggestion:  clipForPrompt(issue.Suggestion, 360),
			Severity:    issue.Severity,
		})
	}

	return compact
}

func compactReviewForAgentSDKPrompt(review models.ReviewResult, volumeID string) composeAgentSDKPromptReviewResult {
	compact := composeAgentSDKPromptReviewResult{
		OverallScore: review.OverallScore,
		Summary:      clipForPrompt(review.Summary, 700),
	}
	for _, suggestion := range review.Suggestions {
		brief := composeAgentSDKPromptReviewSuggestion{
			Category:   suggestion.Category,
			TargetID:   suggestion.TargetID,
			TargetName: clipForPrompt(suggestion.TargetName, 120),
			Issue:      clipForPrompt(suggestion.Issue, 360),
			Suggestion: clipForPrompt(suggestion.Suggestion, 420),
			Priority:   suggestion.Priority,
			Navigation: buildComposeAgentSDKSuggestionNavigation(volumeID, suggestion),
		}
		compact.Suggestions = append(compact.Suggestions, brief)
	}
	return compact
}

func buildComposeAgentSDKSuggestionNavigation(volumeID string, suggestion models.ReviewSuggestion) *composeAgentSDKSuggestionNavigation {
	volumeID = strings.TrimSpace(volumeID)
	targetID := strings.TrimSpace(suggestion.TargetID)
	category := composeAgentSDKCheckCategory(suggestion.Category)
	if volumeID == "" {
		nav := &composeAgentSDKSuggestionNavigation{
			TargetKind:         "outline.global",
			FocusedCheckQuery:  fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", category),
			RepairRouteQuery:   fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view index", category),
			RepairContextQuery: fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view brief", category),
		}
		if category == "faction_tier" && targetID != "" && !strings.EqualFold(targetID, "global") {
			nav.TargetKind = "setup.faction_tier"
			nav.DetailQueries = []string{
				fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view index", targetID),
				fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view brief", targetID),
			}
			nav.RepairRouteQuery = fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view index", targetID)
			nav.RepairContextQuery = fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view brief", targetID)
			nav.PatchQuery = "novelgen tool patch setup"
			nav.PatchShape = map[string]interface{}{
				"premises": []map[string]string{{
					"name":        targetID + " faction tier ladder",
					"category":    targetID,
					"description": "<define stable tier ladder for " + targetID + ">",
				}},
			}
		}
		return nav
	}
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return nil
	}
	if targetID == volumeID {
		return &composeAgentSDKSuggestionNavigation{
			TargetKind: "volume",
			VolumeID:   volumeID,
			DetailQueries: []string{
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", volumeID),
			},
			FocusedCheckQuery:  fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --category %s --min-priority low --max-issues 12", volumeID, category),
			RepairRouteQuery:   fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", volumeID, category),
			RepairContextQuery: fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", volumeID, category),
			PatchQuery:         fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
		}
	}
	if strings.HasPrefix(targetID, volumeID+"-") {
		return &composeAgentSDKSuggestionNavigation{
			TargetKind: "chapter",
			VolumeID:   volumeID,
			DetailQueries: []string{
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", targetID),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --fields result,details,target,target_type,actor,action --view brief", targetID),
			},
			FocusedCheckQuery:  fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --category %s --min-priority low --max-issues 8", targetID, category),
			RepairRouteQuery:   fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", targetID, category),
			RepairContextQuery: fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", targetID, category),
			PatchQuery:         fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
			PatchShape:         composeAgentSDKOutlineChapterPatchShape(targetID, suggestion),
		}
	}
	return nil
}

func composeAgentSDKOutlineChapterPatchShape(targetID string, suggestion models.ReviewSuggestion) map[string]interface{} {
	chapterPatch := map[string]string{"id": targetID}
	if composeAgentSDKIssueNeedsOpeningBeat(suggestion) {
		chapterPatch["opening_beat"] = "<rewrite the chapter opening beat; for continuity include 继续/随后/紧接着, and for location moves include 来到/前往/到达>"
	}
	return map[string]interface{}{
		"changed_chapters": []map[string]string{chapterPatch},
	}
}

func composeAgentSDKIssueNeedsOpeningBeat(suggestion models.ReviewSuggestion) bool {
	category := strings.TrimSpace(strings.ToLower(suggestion.Category))
	text := strings.Join([]string{suggestion.Issue, suggestion.Suggestion}, " ")
	return category == "transition" ||
		strings.Contains(text, "章节开场") ||
		strings.Contains(text, "开场节拍") ||
		strings.Contains(text, "缺少与上一章的过渡") ||
		strings.Contains(text, "缺少过渡描述") ||
		strings.Contains(text, "地点从")
}

func composeAgentSDKCheckCategory(category string) string {
	category = strings.TrimSpace(strings.ToLower(category))
	if category == "" {
		return "all"
	}
	return category
}

func clipForPrompt(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// IterateHierarchical runs the hierarchical review-improvement loop with checkpoint support
// 1. Review entire outline
// 2. Identify volumes that need improvement
// 3. Improve each volume individually
// Supports resuming from checkpoint if interrupted
func (a *ComposeAgent) IterateHierarchical(ctx context.Context, outline *models.Outline, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT - Hierarchical Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: will improve based on suggestions even if score meets threshold")
	}
	logger.Info("This will review the entire outline, then improve volumes individually")

	currentOutline := *outline
	var finalReview *models.ReviewResult

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Iteration %d/%d ===", i, maxIterations)

		// Step 1: Review entire outline
		logger.Section("Step 1: Reviewing Entire Outline")
		reviewInput := ComposeReviewInput{
			ExistingOutline: currentOutline,
			UserPrompt:      userPrompt,
		}
		if setup != nil {
			reviewInput.Setup = *setup
		}
		reviewOutput, err := a.Review(ctx, reviewInput)
		if err != nil {
			return nil, nil, fmt.Errorf("review failed at iteration %d: %w", i, err)
		}

		finalReview = &reviewOutput.Result

		// Check if quality meets threshold
		scoreMeetsThreshold := reviewOutput.Result.OverallScore >= qualityThreshold
		if scoreMeetsThreshold {
			logger.Info("✓ Quality threshold met (%.1f >= %.1f)", reviewOutput.Result.OverallScore, qualityThreshold)
		}

		// Determine if we should improve. Blocking suggestions override the
		// numeric score because they represent issues that must still be fixed.
		hasBlockingSuggestions := reviewOutput.Result.HasBlockingSuggestions()
		if scoreMeetsThreshold && hasBlockingSuggestions {
			logger.Info("Quality threshold met, but blocking suggestions exist; continuing improvement")
		}
		shouldImprove := !scoreMeetsThreshold || hasBlockingSuggestions || forceImprove
		if !shouldImprove {
			break
		}

		// Step 2: Identify volumes that need improvement from suggestions
		logger.Section("Step 2: Improving Volumes")
		volumesToImprove := a.identifyVolumesToImprove(&currentOutline, &reviewOutput.Result)

		if len(volumesToImprove) == 0 {
			logger.Info("No specific volumes identified for improvement, improving all volumes")
			// Improve all volumes
			for partIdx := range currentOutline.Parts {
				for volIdx := range currentOutline.Parts[partIdx].Volumes {
					volumesToImprove = append(volumesToImprove, [2]int{partIdx, volIdx})
				}
			}
		}

		// Step 3: Improve each identified volume with checkpoint support
		// Note: userPrompt is passed to review, not to improve (review generates suggestions based on userPrompt)
		improvedOutline, err := a.improveVolumesWithCheckpoint(ctx, &currentOutline, volumesToImprove, &reviewOutput.Result, i, maxIterations, setup)
		if err != nil {
			return nil, nil, fmt.Errorf("iteration %d failed: %w", i, err)
		}
		currentOutline = *improvedOutline

		logger.Info("✓ All volumes improved, continuing to next iteration")

		// Check if this is the last iteration
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
}

// IterateHierarchicalAgentSDK runs the per-volume review/improve loop through
// Claude Agent SDK workflows. The SDK agent can query project facts, while Go
// keeps validation, merge, checkpoint, and writes.
// crossOutline, when non-nil, is the full outline used for cross-volume
// adjacency in apply mode. It can differ from outline when the caller scoped
// the run to a subset of volumes (outline only contains the selected volumes,
// while crossOutline still contains their neighbors).
func (a *ComposeAgent) IterateHierarchicalAgentSDK(ctx context.Context, outline *models.Outline, crossOutline *models.Outline, initialReview *models.ReviewResult, maxIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup, applyPatches bool, allVolumes bool) (*models.Outline, *models.ReviewResult, error) {
	logger.Section("COMPOSE AGENT SDK - Hierarchical Iteration Loop")
	logger.Info("Max iterations: %d", maxIterations)
	logger.Info("Quality threshold: %.1f", qualityThreshold)
	if forceImprove {
		logger.Info("Force improve enabled: SDK workflow will apply targeted fixes even when score passes")
	}
	if applyPatches {
		logger.Info("Agent apply enabled: SDK workflow may write through validated outline patch tools")
	}

	if outline == nil {
		return nil, nil, fmt.Errorf("outline is nil")
	}
	currentOutline := *outline
	finalReview := initialReview

	for i := 1; i <= maxIterations; i++ {
		logger.Info("=== Agent SDK iteration %d/%d ===", i, maxIterations)

		volumesToImprove := generatedVolumeIndices(&currentOutline)
		if len(volumesToImprove) == 0 {
			logger.Info("No generated volumes to improve")
			break
		}

		improvedOutline, review, improvedCount, err := a.improveVolumesWithCheckpointAgentSDK(ctx, &currentOutline, volumesToImprove, finalReview, i, maxIterations, qualityThreshold, forceImprove, userPrompt, setup, applyPatches, crossOutline, allVolumes)
		if err != nil {
			return nil, finalReview, fmt.Errorf("iteration %d failed: %w", i, err)
		}
		currentOutline = *improvedOutline
		if review != nil {
			finalReview = review
		}
		if improvedCount == 0 && !forceImprove {
			logger.Info("Agent SDK review found no blocking fixes; stopping iteration loop")
			break
		}
		if i == maxIterations {
			logger.Warn("Max iterations reached, stopping iteration loop")
			break
		}
	}

	return &currentOutline, finalReview, nil
}

// RepairByReview applies an existing review result through bounded volume-level
// repairs. It is intended for validator/DSL feedback after the main review pass,
// where re-running a full-outline rewrite would be unnecessarily fragile.
func (a *ComposeAgent) RepairByReview(ctx context.Context, outline *models.Outline, reviewResult *models.ReviewResult, setup *models.StorySetup) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if reviewResult == nil {
		return outline, nil
	}

	volumesToImprove := a.identifyVolumesToImprove(outline, reviewResult)
	if len(volumesToImprove) == 0 {
		logger.Info("No specific volume targets in review; repairing generated volumes only")
		volumesToImprove = generatedVolumeIndices(outline)
	} else {
		volumesToImprove = filterGeneratedVolumeIndices(outline, volumesToImprove)
	}
	if len(volumesToImprove) == 0 {
		logger.Info("No generated volumes to repair")
		return outline, nil
	}

	logger.Info("Repairing %d volume(s) from enriched review feedback", len(volumesToImprove))
	return a.improveVolumesWithCheckpoint(ctx, outline, volumesToImprove, reviewResult, 0, 1, setup)
}

// RepairByReviewAgentSDK applies an existing quality-gate review through the
// SDK volume workflow.
func (a *ComposeAgent) RepairByReviewAgentSDK(ctx context.Context, outline *models.Outline, reviewResult *models.ReviewResult, setup *models.StorySetup, applyPatches bool, crossOutline *models.Outline, allVolumes bool) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if reviewResult == nil {
		return outline, nil
	}

	volumesToImprove := a.identifyVolumesToImprove(outline, reviewResult)
	if len(volumesToImprove) == 0 {
		logger.Info("No specific volume targets in review; repairing generated volumes only")
		volumesToImprove = generatedVolumeIndices(outline)
	} else {
		volumesToImprove = filterGeneratedVolumeIndices(outline, volumesToImprove)
	}
	if len(volumesToImprove) == 0 {
		logger.Info("No generated volumes to repair")
		return outline, nil
	}

	logger.Info("Repairing %d volume(s) with Agent SDK workflow", len(volumesToImprove))
	improved, _, _, err := a.improveVolumesWithCheckpointAgentSDK(ctx, outline, volumesToImprove, reviewResult, 0, 1, 80.0, true, "", setup, applyPatches, crossOutline, allVolumes)
	return improved, err
}

// improveVolumesWithCheckpoint improves volumes with checkpoint/resume support
func (a *ComposeAgent) improveVolumesWithCheckpoint(ctx context.Context, outline *models.Outline, volumesToImprove [][2]int, reviewResult *models.ReviewResult, currentIteration int, totalIterations int, setup *models.StorySetup) (*models.Outline, error) {
	currentOutline := *outline
	progressPath := "story/compose/outline_improve_progress.json"
	targetVolumes := improveTargetVolumeIDs(outline, volumesToImprove)

	// Try to load existing progress
	var progress *ImproveProgress
	if _, err := os.Stat(progressPath); err == nil {
		loadedProgress, err := a.loadImproveProgress(progressPath)
		if err == nil && loadedProgress.Iteration == currentIteration && equalStringSlices(loadedProgress.TargetVolumes, targetVolumes) {
			logger.Info("📂 Found existing progress for iteration %d, resuming...", currentIteration)
			progress = loadedProgress
			currentOutline = progress.Outline
			logger.Info("✓ Resumed from checkpoint: %d/%d volumes completed", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	// Initialize progress if not resuming
	if progress == nil {
		progress = &ImproveProgress{
			Iteration:        currentIteration,
			TotalIterations:  totalIterations,
			CurrentVolumeIdx: 0,
			TotalVolumes:     len(volumesToImprove),
			TargetVolumes:    targetVolumes,
			CompletedVolumes: []string{},
			Outline:          currentOutline,
			ReviewResult:     *reviewResult,
		}
		// Save initial progress
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save initial progress: %v", err)
		}
	}

	// Improve remaining volumes
	for idx := progress.CurrentVolumeIdx; idx < len(volumesToImprove); idx++ {
		indices := volumesToImprove[idx]
		partIdx, volIdx := indices[0], indices[1]
		if partIdx < 0 || partIdx >= len(currentOutline.Parts) || volIdx < 0 || volIdx >= len(currentOutline.Parts[partIdx].Volumes) {
			logger.Warn("Skipping invalid volume repair target part=%d volume=%d", partIdx+1, volIdx+1)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}
		part := &currentOutline.Parts[partIdx]
		volume := &part.Volumes[volIdx]
		if len(volume.Chapters) == 0 {
			logger.Warn("Skipping empty future volume repair target %s", volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}

		logger.Info("Improving Volume %d.%d: %s (%d/%d)", partIdx+1, volIdx+1, volume.Title, idx+1, len(volumesToImprove))

		// Filter suggestions for this volume
		volumeReview := a.filterReviewForVolume(reviewResult, volume.ID)

		improveInput := ComposeImproveVolumeInput{
			Outline:      currentOutline,
			Part:         *part,
			Volume:       *volume,
			ReviewResult: volumeReview,
			// Note: UserPrompt is not passed here - it's used in review to generate suggestions
		}
		if setup != nil {
			improveInput.Setup = *setup
		}

		improveOutput, err := a.ImproveVolume(ctx, improveInput)
		if err != nil {
			// Save progress before returning error
			progress.CurrentVolumeIdx = idx
			progress.Outline = currentOutline
			if saveErr := a.saveImproveProgress(progress, progressPath); saveErr != nil {
				logger.Warn("Failed to save progress on error: %v", saveErr)
			}
			return nil, fmt.Errorf("failed to improve volume %d.%d: %w", partIdx+1, volIdx+1, err)
		}

		// Update the volume in the outline, preserving stable identity fields.
		improvedVolume := improveOutput.Volume
		preserveImprovedVolumeIdentity(volume, &improvedVolume)
		part.Volumes[volIdx] = improvedVolume
		progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
		progress.CurrentVolumeIdx = idx + 1
		progress.Outline = currentOutline

		// Save progress after each volume
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save progress: %v", err)
		} else {
			logger.Info("[save] Progress saved (%d/%d volumes completed)", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	// Remove progress file on successful completion of this iteration
	os.Remove(progressPath)
	logger.Info("[ok] Iteration %d complete. Progress file removed.", currentIteration)

	return &currentOutline, nil
}

// ComposeImproveReportEntry is a per-volume improvement summary recorded during
// Agent SDK outline improvement. The CLI aggregates these into an end-of-run
// Markdown report.
type ComposeImproveReportEntry struct {
	Iteration           int     `json:"iteration"`
	VolumeID            string  `json:"volume_id"`
	VolumeTitle         string  `json:"volume_title"`
	Score               float64 `json:"score"`
	Changed             bool    `json:"changed"`
	Skipped             bool    `json:"skipped"`
	Summary             string  `json:"summary"`
	RemainingMediumPlus int     `json:"remaining_medium_plus"`
}

const composeImproveReportPath = "story/compose/outline_improve_report.jsonl"

func appendComposeImproveReport(entry ComposeImproveReportEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(composeImproveReportPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func countMediumOrHigherReviewSuggestions(review models.ReviewResult) int {
	count := 0
	for _, suggestion := range review.Suggestions {
		switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
		case models.PriorityCritical, models.PriorityHigh, models.PriorityMedium:
			count++
		}
	}
	return count
}

func (a *ComposeAgent) improveVolumesWithCheckpointAgentSDK(ctx context.Context, outline *models.Outline, volumesToImprove [][2]int, reviewResult *models.ReviewResult, currentIteration int, totalIterations int, qualityThreshold float64, forceImprove bool, userPrompt string, setup *models.StorySetup, applyPatches bool, crossOutline *models.Outline, allVolumes bool) (*models.Outline, *models.ReviewResult, int, error) {
	currentOutline := *outline
	progressPath := "story/compose/outline_improve_progress.json"
	targetVolumes := improveTargetVolumeIDs(outline, volumesToImprove)
	var finalReview *models.ReviewResult

	var progress *ImproveProgress
	if _, err := os.Stat(progressPath); err == nil {
		loadedProgress, err := a.loadImproveProgress(progressPath)
		if err == nil && loadedProgress.Iteration == currentIteration && equalStringSlices(loadedProgress.TargetVolumes, targetVolumes) {
			logger.Info("[resume] Found existing Agent SDK progress for iteration %d, resuming...", currentIteration)
			progress = loadedProgress
			currentOutline = progress.Outline
			if !isEmptyReviewResult(&progress.ReviewResult) {
				finalReview = &progress.ReviewResult
			}
			logger.Info("[ok] Resumed from checkpoint: %d/%d volumes completed", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
	}

	if progress == nil {
		progress = &ImproveProgress{
			Iteration:        currentIteration,
			TotalIterations:  totalIterations,
			CurrentVolumeIdx: 0,
			TotalVolumes:     len(volumesToImprove),
			TargetVolumes:    targetVolumes,
			CompletedVolumes: []string{},
			Outline:          currentOutline,
		}
		if reviewResult != nil {
			progress.ReviewResult = *reviewResult
		}
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save initial Agent SDK progress: %v", err)
		}
	}

	improvedCount := 0
	for idx := progress.CurrentVolumeIdx; idx < len(volumesToImprove); idx++ {
		// Full-book mode hands the whole book-level task to the FIRST volume's
		// session (it may patch every volume); the remaining volumes then run
		// as ordinary per-volume sessions to avoid every session re-fixing the
		// same book-level issues.
		fullBookSession := allVolumes && idx == 0
		indices := volumesToImprove[idx]
		partIdx, volIdx := indices[0], indices[1]
		if partIdx < 0 || partIdx >= len(currentOutline.Parts) || volIdx < 0 || volIdx >= len(currentOutline.Parts[partIdx].Volumes) {
			logger.Warn("Skipping invalid Agent SDK volume target part=%d volume=%d", partIdx+1, volIdx+1)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}
		part := &currentOutline.Parts[partIdx]
		volume := &part.Volumes[volIdx]
		if len(volume.Chapters) == 0 {
			logger.Warn("Skipping empty future volume target %s", volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			continue
		}

		logger.Info("Agent SDK reviewing Volume %d.%d: %s (%d/%d)", partIdx+1, volIdx+1, volume.Title, idx+1, len(volumesToImprove))
		var volumeReview models.ReviewResult
		if reviewResult != nil {
			volumeReview = a.filterReviewForVolume(reviewResult, volume.ID)
			logger.Info("Agent SDK volume %s received %d filtered review issue(s)", volume.ID, len(volumeReview.Suggestions))
		}
		// Skip volumes with no medium/high/critical suggestions: there is
		// nothing for the agent to do. forceImprove (--force or selected
		// volume scope) no longer disables this skip - a volume without
		// suggestions has no tasks regardless of force, so running a full
		// agent session on it only burns tokens.
		if reviewResult != nil && !hasMediumOrHigherReviewSuggestion(volumeReview) {
			logger.Info("[skip] Agent SDK skipping %s: no medium/high/critical volume issues after pre-check", volume.Title)
			progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
			progress.CurrentVolumeIdx = idx + 1
			progress.Outline = currentOutline
			if err := a.saveImproveProgress(progress, progressPath); err != nil {
				logger.Warn("Failed to save Agent SDK progress after skip: %v", err)
			}
			if err := appendComposeImproveReport(ComposeImproveReportEntry{
				Iteration:   currentIteration,
				VolumeID:    volume.ID,
				VolumeTitle: volume.Title,
				Skipped:     true,
				Summary:     "Pre-check found no medium/high/critical volume issues; skipped.",
			}); err != nil {
				logger.Warn("Failed to append improve report entry: %v", err)
			}
			continue
		}
		improveInput := ComposeImproveVolumeInput{
			Outline:      currentOutline,
			Part:         *part,
			Volume:       *volume,
			ReviewResult: volumeReview,
		}
		if fullBookSession {
			// Full-book mode: keep the user prompt unscoped so the session knows
			// this is a book-level task spanning many volumes, and let the
			// session see ALL suggestions (not just its own target), so a
			// book-level issue (e.g. a cost that inflates across 13 volumes)
			// can be rooted out in one session.
			bookLevelInstruction := "FULL-BOOK CROSS-VOLUME SESSION: you are authorized to query, check, and patch EVERY volume of the outline (P1-V1 through P5-V5), not just the target volume. The suggestions below are book-level issues spanning multiple volumes. For each one: locate ALL affected volumes/chapters, patch them consistently in this session, and run checks for every volume you touch. Do NOT defer issues to another volume's session, do NOT mark book-level suggestions as 'outside this volume's boundary' — the boundary is the whole book. If a suggestion references specific chapter IDs in other volumes, query and patch those too."
			if strings.TrimSpace(userPrompt) != "" {
				improveInput.UserPrompt = bookLevelInstruction + "\n\n" + userPrompt
			} else {
				improveInput.UserPrompt = bookLevelInstruction
			}
			if reviewResult != nil {
				improveInput.ReviewResult = *reviewResult
			}
		} else {
			improveInput.UserPrompt = scopeComposeAgentSDKVolumeUserPrompt(userPrompt, volume.ID, volume.Title)
			improveInput.ReviewResult = volumeReview
		}
		// Cross-volume by default (apply mode only): the session may also patch
		// adjacent volumes (previous + next), so volume-end hooks and next-volume
		// payoffs can be fixed consistently in one session instead of being
		// deferred as "outside this volume's boundary". Non-apply mode merges a
		// single volume_patch in Go, so cross-volume patching is only available
		// when the agent writes through tool patch --apply.
		if applyPatches {
			adjacencyOutline := currentOutline
			if crossOutline != nil {
				adjacencyOutline = *crossOutline
			}
			if fullBookSession {
				// Full-book cross-volume mode: every volume in the outline is
				// patchable in this session, so book-level issues (a golden
				// finger cost that inflates across 13 volumes, a character
				// whose intelligence drifts per-volume) can be rooted out in
				// one session instead of being chased volume by volume.
				allIDs := make([]string, 0)
				for _, part := range adjacencyOutline.Parts {
					for _, v := range part.Volumes {
						if strings.TrimSpace(v.ID) != "" {
							allIDs = append(allIDs, v.ID)
						}
					}
				}
				improveInput.CrossVolumeIDs = allIDs
				improveInput.Outline = adjacencyOutline
			} else if adjacent := composeAdjacentVolumeIDs(adjacencyOutline, volume.ID); len(adjacent) > 0 {
				improveInput.CrossVolumeIDs = adjacent
				// The session must see the full outline so the cross-volume
				// patch/query allowlist expansion can find the adjacent volumes.
				improveInput.Outline = adjacencyOutline
			}
		}
		if setup != nil {
			improveInput.Setup = *setup
		}

		forceIssueRepair := shouldForceAgentSDKIssueRepair(forceImprove, improveInput.UserPrompt, volumeReview)
		improveOutput, err := a.ImproveVolumeWithAgentSDK(ctx, improveInput, applyPatches, forceIssueRepair)
		if err != nil {
			progress.CurrentVolumeIdx = idx
			progress.Outline = currentOutline
			if saveErr := a.saveImproveProgress(progress, progressPath); saveErr != nil {
				logger.Warn("Failed to save Agent SDK progress on error: %v", saveErr)
			}
			return nil, finalReview, improvedCount, fmt.Errorf("failed to improve volume %d.%d: %w", partIdx+1, volIdx+1, err)
		}

		review := improveOutput.ReviewResult
		review.Iteration = currentIteration
		review.NormalizeScoreScale()
		finalReview = &review
		progress.ReviewResult = review

		improvedVolume := improveOutput.Volume
		patchChanged := agentSDKVolumeChanged(volume, &improvedVolume)
		if patchChanged {
			part.Volumes[volIdx] = improvedVolume
			improvedCount++
			logger.Info("[ok] Agent SDK applied volume fixes: %s", improvedVolume.Title)
		} else {
			scoreMeetsThreshold := review.OverallScore >= qualityThreshold
			hasBlockingSuggestions := review.HasBlockingSuggestions()
			if scoreMeetsThreshold && !hasBlockingSuggestions && (!forceImprove || !forceIssueRepair) {
				logger.Info("[ok] Agent SDK review passed for %s (%.1f >= %.1f); no effective volume changes", volume.Title, review.OverallScore, qualityThreshold)
			} else {
				logger.Info("[warn] Agent SDK returned no volume changes for %s (score %.1f, blocking=%t)", volume.Title, review.OverallScore, hasBlockingSuggestions)
			}
		}

		progress.CompletedVolumes = append(progress.CompletedVolumes, volume.ID)
		progress.CurrentVolumeIdx = idx + 1
		progress.Outline = currentOutline
		if err := a.saveImproveProgress(progress, progressPath); err != nil {
			logger.Warn("Failed to save Agent SDK progress: %v", err)
		} else {
			logger.Info("[save] Agent SDK progress saved (%d/%d volumes completed)", len(progress.CompletedVolumes), progress.TotalVolumes)
		}
		if err := appendComposeImproveReport(ComposeImproveReportEntry{
			Iteration:           currentIteration,
			VolumeID:            volume.ID,
			VolumeTitle:         volume.Title,
			Score:               review.OverallScore,
			Changed:             patchChanged,
			Summary:             review.Summary,
			RemainingMediumPlus: countMediumOrHigherReviewSuggestions(review),
		}); err != nil {
			logger.Warn("Failed to append improve report entry: %v", err)
		}
	}

	os.Remove(progressPath)
	logger.Info("[ok] Agent SDK iteration %d complete. Progress file removed.", currentIteration)
	return &currentOutline, finalReview, improvedCount, nil
}

func shouldForceAgentSDKIssueRepair(forceImprove bool, userPrompt string, review models.ReviewResult) bool {
	if !forceImprove {
		return false
	}
	return strings.TrimSpace(userPrompt) != "" || len(review.Suggestions) > 0
}

var (
	composeChineseVolumeListPattern = regexp.MustCompile(`第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷(?:\s*(?:和|及|与|、|,|，)\s*第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷)+`)
	composeChineseVolumePattern     = regexp.MustCompile(`第\s*(?:\d+|[一二三四五六七八九十百]+)\s*卷`)
	composeVolumeIDListPattern      = regexp.MustCompile(`P\d+-V\d+(?:\s*(?:和|及|与|、|,|，)\s*P\d+-V\d+)+`)
)

func scopeComposeAgentSDKVolumeUserPrompt(userPrompt, volumeID, volumeTitle string) string {
	prompt := strings.TrimSpace(userPrompt)
	if prompt == "" {
		return ""
	}
	localized := composeVolumeIDListPattern.ReplaceAllString(prompt, nonEmpty(volumeID, "target_volume_id"))
	localized = composeChineseVolumeListPattern.ReplaceAllString(localized, "当前卷")
	localized = composeChineseVolumePattern.ReplaceAllString(localized, "当前卷")

	target := strings.TrimSpace(volumeID)
	if title := strings.TrimSpace(volumeTitle); title != "" {
		if target != "" {
			target += " / "
		}
		target += title
	}
	if target == "" {
		target = "target_volume_id"
	}
	return fmt.Sprintf("当前调用只处理 %s。已将用户请求局部化到当前卷；如果原始请求包含多个卷，只执行当前卷这一份，不要查询、检查或 patch 其它卷。局部化用户请求：%s", target, localized)
}

func composePromptRequestsCheckFirstNoop(prompt string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(prompt)), " "))
	if normalized == "" {
		return false
	}
	hasCheckIntent := strings.Contains(normalized, "check") ||
		strings.Contains(normalized, "检查") ||
		strings.Contains(normalized, "校验") ||
		strings.Contains(normalized, "验证")
	hasCleanCondition := strings.Contains(normalized, "0 issue") ||
		strings.Contains(normalized, "0issue") ||
		strings.Contains(normalized, "无 issue") ||
		strings.Contains(normalized, "无问题") ||
		strings.Contains(normalized, "没有问题") ||
		strings.Contains(normalized, "返回 0") ||
		strings.Contains(normalized, "返回0")
	hasNoopIntent := strings.Contains(normalized, "不要 patch") ||
		strings.Contains(normalized, "不要patch") ||
		strings.Contains(normalized, "不要改") ||
		strings.Contains(normalized, "不要修改") ||
		(strings.Contains(normalized, "不要") && strings.Contains(normalized, "改写")) ||
		(strings.Contains(normalized, "不要") && strings.Contains(normalized, "patch")) ||
		strings.Contains(normalized, "不改写") ||
		strings.Contains(normalized, "no-op") ||
		strings.Contains(normalized, "no patch")
	return hasCheckIntent && hasCleanCondition && hasNoopIntent
}

func isEmptyReviewResult(review *models.ReviewResult) bool {
	if review == nil {
		return true
	}
	return review.OverallScore == 0 &&
		len(review.Dimensions) == 0 &&
		strings.TrimSpace(review.Summary) == "" &&
		len(review.Strengths) == 0 &&
		len(review.Weaknesses) == 0 &&
		len(review.Suggestions) == 0 &&
		len(review.ContinuityIssues) == 0
}

func agentSDKVolumeChanged(original *models.Volume, improved *models.Volume) bool {
	if original == nil || improved == nil {
		return false
	}
	preserveImprovedVolumeIdentity(original, improved)
	return !reflect.DeepEqual(*original, *improved)
}

func preserveImprovedVolumeIdentity(original *models.Volume, improved *models.Volume) {
	if original == nil || improved == nil {
		return
	}
	improved.ID = original.ID
	for i := range improved.Chapters {
		if i >= len(original.Chapters) {
			break
		}
		improved.Chapters[i].ID = original.Chapters[i].ID
	}
}

func allVolumeIndices(outline *models.Outline) [][2]int {
	if outline == nil {
		return nil
	}
	var result [][2]int
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			result = append(result, [2]int{partIdx, volIdx})
		}
	}
	return result
}

func generatedVolumeIndices(outline *models.Outline) [][2]int {
	if outline == nil {
		return nil
	}
	var result [][2]int
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			if len(outline.Parts[partIdx].Volumes[volIdx].Chapters) == 0 {
				continue
			}
			result = append(result, [2]int{partIdx, volIdx})
		}
	}
	return result
}

func filterGeneratedVolumeIndices(outline *models.Outline, indices [][2]int) [][2]int {
	if outline == nil {
		return nil
	}
	var result [][2]int
	for _, index := range indices {
		partIdx, volIdx := index[0], index[1]
		if partIdx < 0 || partIdx >= len(outline.Parts) || volIdx < 0 || volIdx >= len(outline.Parts[partIdx].Volumes) {
			continue
		}
		if len(outline.Parts[partIdx].Volumes[volIdx].Chapters) == 0 {
			continue
		}
		result = append(result, index)
	}
	return result
}

func improveTargetVolumeIDs(outline *models.Outline, indices [][2]int) []string {
	if outline == nil {
		return nil
	}
	targets := make([]string, 0, len(indices))
	for _, index := range indices {
		partIdx, volIdx := index[0], index[1]
		if partIdx < 0 || partIdx >= len(outline.Parts) || volIdx < 0 || volIdx >= len(outline.Parts[partIdx].Volumes) {
			targets = append(targets, fmt.Sprintf("invalid:%d:%d", partIdx, volIdx))
			continue
		}
		volumeID := strings.TrimSpace(outline.Parts[partIdx].Volumes[volIdx].ID)
		if volumeID == "" {
			volumeID = fmt.Sprintf("P%d-V%d", partIdx+1, volIdx+1)
		}
		targets = append(targets, volumeID)
	}
	return targets
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// loadImproveProgress loads improvement progress from file
func (a *ComposeAgent) loadImproveProgress(path string) (*ImproveProgress, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read progress file: %w", err)
	}

	var progress ImproveProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse progress file: %w", err)
	}

	return &progress, nil
}

// saveImproveProgress saves improvement progress to file
func (a *ComposeAgent) saveImproveProgress(progress *ImproveProgress, path string) error {
	if progress == nil {
		return fmt.Errorf("cannot save nil progress")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Write to temporary file first, then rename for atomic operation
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write progress file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename progress file: %w", err)
	}

	return nil
}

// identifyVolumesToImprove identifies which volumes need improvement based on suggestions
func (a *ComposeAgent) identifyVolumesToImprove(outline *models.Outline, review *models.ReviewResult) [][2]int {
	if outline == nil || review == nil {
		return nil
	}

	seen := make(map[[2]int]bool)
	var result [][2]int

	for _, suggestion := range review.Suggestions {
		indices, ok := resolveVolumeTarget(outline, suggestion.TargetID)
		if !ok {
			continue
		}
		if seen[indices] {
			continue
		}
		seen[indices] = true
		result = append(result, indices)
	}

	return result
}

func resolveVolumeTarget(outline *models.Outline, targetID string) ([2]int, bool) {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" || strings.EqualFold(targetID, "global") {
		return [2]int{}, false
	}

	for partIdx, part := range outline.Parts {
		for volIdx, volume := range part.Volumes {
			if targetID == volume.ID || strings.HasPrefix(targetID, volume.ID+"-") {
				return [2]int{partIdx, volIdx}, true
			}
		}
	}

	parts := strings.Split(targetID, "-")
	if len(parts) < 2 {
		return [2]int{}, false
	}
	volumeID := parts[0] + "-" + parts[1]

	var partNum, volNum int
	n, err := fmt.Sscanf(volumeID, "P%d-V%d", &partNum, &volNum)
	if err != nil || n != 2 {
		logger.Warn("Failed to parse volume ID '%s', skipping", volumeID)
		return [2]int{}, false
	}
	if partNum <= 0 || volNum <= 0 {
		logger.Warn("Invalid volume ID '%s' (part=%d, vol=%d), skipping", volumeID, partNum, volNum)
		return [2]int{}, false
	}

	partIdx := partNum - 1
	volIdx := volNum - 1
	if partIdx >= 0 && partIdx < len(outline.Parts) && volIdx >= 0 && volIdx < len(outline.Parts[partIdx].Volumes) {
		return [2]int{partIdx, volIdx}, true
	}

	// Single-part projects are often described by users and LLM review as
	// "volume 2"; tolerate malformed targets like P2-V2 by resolving the
	// volume ordinal against the only part instead of silently skipping it.
	if len(outline.Parts) == 1 && volIdx >= 0 && volIdx < len(outline.Parts[0].Volumes) {
		logger.Warn("Interpreting malformed single-part volume target '%s' as '%s'", volumeID, outline.Parts[0].Volumes[volIdx].ID)
		return [2]int{0, volIdx}, true
	}

	logger.Warn("Skipping invalid volume repair target %s", volumeID)
	return [2]int{}, false
}

// filterReviewForVolume filters review results for a specific volume
func (a *ComposeAgent) filterReviewForVolume(review *models.ReviewResult, volumeID string) models.ReviewResult {
	if review == nil {
		return models.ReviewResult{}
	}
	filtered := models.ReviewResult{
		OverallScore: review.OverallScore,
		Dimensions:   review.Dimensions,
		Summary:      review.Summary,
	}

	for _, suggestion := range review.Suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if strings.HasPrefix(targetID, volumeID) || isBlockingGlobalSuggestion(suggestion) {
			filtered.Suggestions = append(filtered.Suggestions, suggestion)
		}
	}
	sort.SliceStable(filtered.Suggestions, func(i, j int) bool {
		left := reviewPriorityRank(filtered.Suggestions[i].Priority)
		right := reviewPriorityRank(filtered.Suggestions[j].Priority)
		if left != right {
			return left < right
		}
		return strings.TrimSpace(filtered.Suggestions[i].TargetID) < strings.TrimSpace(filtered.Suggestions[j].TargetID)
	})
	if len(filtered.Suggestions) > 12 {
		filtered.Suggestions = filtered.Suggestions[:12]
	}

	return filtered
}

func reviewPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case models.PriorityCritical:
		return 0
	case models.PriorityHigh:
		return 1
	case models.PriorityMedium:
		return 2
	case models.PriorityLow:
		return 3
	default:
		return 4
	}
}

func isBlockingGlobalSuggestion(suggestion models.ReviewSuggestion) bool {
	targetID := strings.TrimSpace(suggestion.TargetID)
	if targetID != "" && !strings.EqualFold(targetID, "global") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
	case models.PriorityCritical, models.PriorityHigh:
		return true
	default:
		return false
	}
}

func hasMediumOrHigherReviewSuggestion(review models.ReviewResult) bool {
	for _, suggestion := range review.Suggestions {
		switch strings.ToLower(strings.TrimSpace(suggestion.Priority)) {
		case models.PriorityCritical, models.PriorityHigh, models.PriorityMedium:
			return true
		}
	}
	return false
}

// BuildVolumeContext builds context for volume regeneration
func (a *ComposeAgent) BuildVolumeContext(volume *models.Volume, outline *models.Outline) string {
	var context strings.Builder

	// Find volume in outline
	for _, part := range outline.Parts {
		for i, vol := range part.Volumes {
			if vol.ID == volume.ID {
				// Add part context
				context.WriteString(fmt.Sprintf("Part: %s\nSummary: %s\n\n", part.Title, part.Summary))

				// Add sibling volumes
				if i > 0 {
					prevVol := part.Volumes[i-1]
					context.WriteString(fmt.Sprintf("Previous Volume (%s): %s\nSummary: %s\n\n",
						prevVol.ID, prevVol.Title, prevVol.Summary))
				}
				if i < len(part.Volumes)-1 {
					nextVol := part.Volumes[i+1]
					context.WriteString(fmt.Sprintf("Next Volume (%s): %s\nSummary: %s\n\n",
						nextVol.ID, nextVol.Title, nextVol.Summary))
				}
				return context.String()
			}
		}
	}

	return context.String()
}

// BuildChapterContext builds context for chapter regeneration
func (a *ComposeAgent) BuildChapterContext(chapter *models.Chapter, outline *models.Outline) string {
	var context strings.Builder

	// Find chapter in outline
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for i, chap := range vol.Chapters {
				if chap.ID == chapter.ID {
					// Add part and volume context
					context.WriteString("=== CURRENT LOCATION IN STORY ===\n")
					context.WriteString(fmt.Sprintf("Part: %s\nPart Summary: %s\n\n", part.Title, part.Summary))
					context.WriteString(fmt.Sprintf("Volume: %s\nVolume Summary: %s\n\n", vol.Title, vol.Summary))

					// Add previous chapters context (up to 2 chapters back for better continuity)
					context.WriteString("=== PREVIOUS CHAPTERS (For Continuity) ===\n")
					if i > 0 {
						prevChap := vol.Chapters[i-1]
						context.WriteString(fmt.Sprintf("Previous Chapter (%s): %s\n", prevChap.ID, prevChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prevChap.Summary))
						context.WriteString(fmt.Sprintf("Events: %s\n", a.formatEvents(prevChap.Events)))
						prevBeats := "None"
						if len(prevChap.GetBeats()) > 0 {
							prevBeats = strings.Join(prevChap.GetBeats(), "; ")
						}
						lastBeat := "None"
						if len(prevChap.GetBeats()) > 0 {
							lastBeat = prevChap.GetBeats()[len(prevChap.GetBeats())-1]
						}
						prevClosing := getClosingBeat(prevChap)
						if prevClosing == "" {
							prevClosing = lastBeat
						}
						context.WriteString(fmt.Sprintf("Beats: %s\n", prevBeats))
						context.WriteString(fmt.Sprintf("Final Beat: %s\n", lastBeat))
						context.WriteString(fmt.Sprintf("Closing Beat: %s\n", prevClosing))
						context.WriteString("\n")
					}
					if i > 1 {
						prev2Chap := vol.Chapters[i-2]
						context.WriteString(fmt.Sprintf("Two Chapters Back (%s): %s\n", prev2Chap.ID, prev2Chap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", prev2Chap.Summary))
						context.WriteString(fmt.Sprintf("Key Events: %s\n\n", a.formatEvents(prev2Chap.Events)))
					}

					// Add next chapter context
					if i < len(vol.Chapters)-1 {
						nextChap := vol.Chapters[i+1]
						context.WriteString("=== NEXT CHAPTER (What This Chapter Must Lead To) ===\n")
						context.WriteString(fmt.Sprintf("Next Chapter (%s): %s\n", nextChap.ID, nextChap.Title))
						context.WriteString(fmt.Sprintf("Summary: %s\n", nextChap.Summary))
						nextFirstBeat := getOpeningBeat(nextChap)
						context.WriteString(fmt.Sprintf("Opening Beat: %s\n", nextFirstBeat))
						context.WriteString(fmt.Sprintf("This chapter MUST set up: %s\n\n", nextChap.Summary))
					}

					// Add current chapter to regenerate
					context.WriteString("=== CURRENT CHAPTER TO REGENERATE ===\n")
					context.WriteString(fmt.Sprintf("Chapter Title: %s\n", chapter.Title))
					context.WriteString(fmt.Sprintf("Current Summary: %s\n", chapter.Summary))
					context.WriteString(fmt.Sprintf("Current Events: %s\n", a.formatEvents(chapter.Events)))

					return context.String()
				}
			}
		}
	}

	return context.String()
}

// formatEvents formats events for context display
func (a *ComposeAgent) formatEvents(events []models.Event) string {
	if len(events) == 0 {
		return "None"
	}
	var parts []string
	for _, e := range events {
		part := fmt.Sprintf("[%s: %s - %s]", e.Type, e.Subject, e.Change)
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// validateOutlineStructure validates the outline matches the expected structure
func (a *ComposeAgent) validateOutlineStructure(outline *models.Outline, structure models.StoryStructure) error {
	if len(outline.Parts) != structure.TargetParts {
		logger.Error("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
		return fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	return nil
}

// validateOutlineChapters validates all chapters in the outline
func (a *ComposeAgent) validateOutlineChapters(outline *models.Outline) error {
	if outline == nil {
		return fmt.Errorf("outline is nil")
	}
	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			for chapIdx := range outline.Parts[partIdx].Volumes[volIdx].Chapters {
				chapter := &outline.Parts[partIdx].Volumes[volIdx].Chapters[chapIdx]
				if err := a.validateChapterOutput(chapter); err != nil {
					return fmt.Errorf("chapter %d.%d.%d invalid: %w", partIdx+1, volIdx+1, chapIdx+1, err)
				}
			}
		}
	}
	return nil
}

// validateChapterOutput validates a chapter's output
func (a *ComposeAgent) validateChapterOutput(chapter *models.Chapter) error {
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	if len(chapter.GetBeats()) == 0 {
		return fmt.Errorf("scene beats are required")
	}
	if len(chapter.Events) == 0 {
		return fmt.Errorf("events are required")
	}
	return nil
}

func validateOutlineSkeleton(outline *models.Outline, structure models.StoryStructure) error {
	if outline == nil {
		return fmt.Errorf("outline is nil")
	}
	if len(outline.Parts) == 0 {
		return fmt.Errorf("outline skeleton has no parts")
	}
	if structure.TargetParts > 0 && len(outline.Parts) != structure.TargetParts {
		return fmt.Errorf("outline skeleton has %d parts, expected %d", len(outline.Parts), structure.TargetParts)
	}
	for partIdx, part := range outline.Parts {
		if strings.TrimSpace(part.Title) == "" {
			return fmt.Errorf("part %d has empty title", partIdx+1)
		}
		if strings.TrimSpace(part.Summary) == "" {
			return fmt.Errorf("part %d has empty summary", partIdx+1)
		}
		if len(part.Volumes) == 0 {
			return fmt.Errorf("part %d has no volumes", partIdx+1)
		}
		if structure.TargetVolumes > 0 && len(part.Volumes) != structure.TargetVolumes {
			return fmt.Errorf("part %d has %d volumes, expected %d", partIdx+1, len(part.Volumes), structure.TargetVolumes)
		}
		for volIdx, volume := range part.Volumes {
			if strings.TrimSpace(volume.Title) == "" {
				return fmt.Errorf("volume %d.%d has empty title", partIdx+1, volIdx+1)
			}
			if strings.TrimSpace(volume.Summary) == "" {
				return fmt.Errorf("volume %d.%d has empty summary", partIdx+1, volIdx+1)
			}
		}
	}
	return nil
}

func preserveSkeletonIdentityAndChapters(original *models.Outline, improved *models.Outline) error {
	if original == nil || improved == nil {
		return fmt.Errorf("outline is nil")
	}
	if len(original.Parts) != len(improved.Parts) {
		return fmt.Errorf("improved skeleton changed part count from %d to %d", len(original.Parts), len(improved.Parts))
	}
	for partIdx := range original.Parts {
		if len(original.Parts[partIdx].Volumes) != len(improved.Parts[partIdx].Volumes) {
			return fmt.Errorf("improved skeleton changed volume count in part %d from %d to %d",
				partIdx+1, len(original.Parts[partIdx].Volumes), len(improved.Parts[partIdx].Volumes))
		}
		improved.Parts[partIdx].ID = original.Parts[partIdx].ID
		for volIdx := range original.Parts[partIdx].Volumes {
			improved.Parts[partIdx].Volumes[volIdx].ID = original.Parts[partIdx].Volumes[volIdx].ID
			improved.Parts[partIdx].Volumes[volIdx].Chapters = original.Parts[partIdx].Volumes[volIdx].Chapters
		}
	}
	return nil
}

// GenerateSkeleton generates the outline skeleton (parts and volumes without chapters)
func (a *ComposeAgent) GenerateSkeleton(ctx context.Context, input ComposeSkeletonInput) (ComposeSkeletonOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Outline Skeleton")
	logger.Info("Project: %s", input.Setup.ProjectName)
	logger.Info("Structure: %d parts × %d volumes",
		input.Structure.TargetParts, input.Structure.TargetVolumes)
	logger.Info("Language: %s", a.base.language)

	var output ComposeSkeletonOutput
	params := InvokeParams{
		Skills:  []string{"compose-skeleton"},
		Command: "generate the story outline skeleton with parts and volumes only",
	}

	promptInput := composeSkeletonPromptInput{
		SetupBrief: a.buildSetupBrief(&input.Setup),
		Structure:  input.Structure,
	}
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeSkeletonOutput{}, err
	}

	// Validate the skeleton structure
	if len(output.Parts) != input.Structure.TargetParts {
		return ComposeSkeletonOutput{}, fmt.Errorf("AI generated %d parts, but %d were requested",
			len(output.Parts), input.Structure.TargetParts)
	}

	for i, part := range output.Parts {
		if len(part.Volumes) != input.Structure.TargetVolumes {
			return ComposeSkeletonOutput{}, fmt.Errorf("part %d has %d volumes, but %d were requested",
				i+1, len(part.Volumes), input.Structure.TargetVolumes)
		}
	}

	logger.Info("Generated skeleton with %d part(s), %d volume(s) per part",
		len(output.Parts), input.Structure.TargetVolumes)

	return output, nil
}

// GenerateChaptersForVolume generates chapters for a specific volume
func (a *ComposeAgent) GenerateChaptersForVolume(ctx context.Context, input ComposeChaptersInput) (ComposeChaptersOutput, error) {
	logger.Section("COMPOSE AGENT - Generating Chapters for Volume")
	logger.Info("Volume: %s (%d/%d)", input.Volume.Title, input.VolumeIndex, input.TotalVolumes)
	logger.Info("Chapters to generate: %d", input.ChaptersPerVol)
	logger.Info("Language: %s", a.base.language)

	var output ComposeChaptersOutput
	params := InvokeParams{
		Skills:  []string{"compose-chapters"},
		Command: fmt.Sprintf("generate %d chapters for this volume with proper continuity", input.ChaptersPerVol),
	}

	promptInput := a.buildChaptersPromptInput(input)
	if err := a.base.Execute(ctx, params, promptInput, &output); err != nil {
		return ComposeChaptersOutput{}, err
	}

	chapters := append([]models.Chapter(nil), output.Chapters...)
	if len(chapters) > input.ChaptersPerVol {
		logger.Warn("AI generated %d chapters, but %d were requested; truncating extras", len(chapters), input.ChaptersPerVol)
		chapters = chapters[:input.ChaptersPerVol]
	}

	const maxAttempts = 3
	for attempt := 2; len(chapters) < input.ChaptersPerVol && attempt <= maxAttempts; attempt++ {
		missing := input.ChaptersPerVol - len(chapters)
		logger.Warn("AI generated %d/%d chapters; requesting %d missing chapter(s) (attempt %d/%d)",
			len(chapters), input.ChaptersPerVol, missing, attempt, maxAttempts)

		continueInput := input
		continueInput.ChaptersPerVol = missing
		continueInput.Volume.Chapters = append([]models.Chapter(nil), chapters...)
		continueInput.OutlineContext = buildChapterContinuationContext(input.OutlineContext, chapters, input.ChaptersPerVol)

		continueParams := InvokeParams{
			Skills:  []string{"compose-chapters"},
			Command: fmt.Sprintf("continue this volume by generating only the remaining %d chapters; do not repeat existing chapters", missing),
		}

		var continued ComposeChaptersOutput
		continuePromptInput := a.buildChaptersPromptInput(continueInput)
		if err := a.base.Execute(ctx, continueParams, continuePromptInput, &continued); err != nil {
			return ComposeChaptersOutput{}, err
		}
		before := len(chapters)
		chapters = appendNewChapters(chapters, continued.Chapters)
		if len(chapters) > input.ChaptersPerVol {
			logger.Warn("Continuation generated too many chapters; truncating to requested %d", input.ChaptersPerVol)
			chapters = chapters[:input.ChaptersPerVol]
		}
		if len(chapters) == before {
			return ComposeChaptersOutput{}, fmt.Errorf("AI generated %d chapters, but %d were requested; continuation returned no new chapters",
				len(chapters), input.ChaptersPerVol)
		}
	}

	if len(chapters) != input.ChaptersPerVol {
		return ComposeChaptersOutput{}, fmt.Errorf("AI generated %d chapters after retries, but %d were requested",
			len(chapters), input.ChaptersPerVol)
	}

	// Validate each chapter
	for i, chapter := range chapters {
		if err := a.validateChapterOutput(&chapter); err != nil {
			return ComposeChaptersOutput{}, fmt.Errorf("chapter %d invalid: %w", i+1, err)
		}
	}

	output.Chapters = chapters
	logger.Info("Generated %d chapters for volume", len(output.Chapters))

	return output, nil
}

func (a *ComposeAgent) buildChaptersPromptInput(input ComposeChaptersInput) composeChaptersPromptInput {
	return composeChaptersPromptInput{
		SetupBrief:     a.buildSetupBrief(&input.Setup),
		Part:           input.Part,
		Volume:         input.Volume,
		VolumeIndex:    input.VolumeIndex,
		TotalVolumes:   input.TotalVolumes,
		ChaptersPerVol: input.ChaptersPerVol,
		PreviousVolume: input.PreviousVolume,
		OutlineContext: input.OutlineContext,
	}
}

func buildChapterContinuationContext(baseContext string, existing []models.Chapter, totalRequested int) string {
	var b strings.Builder
	b.WriteString(baseContext)
	if baseContext != "" && !strings.HasSuffix(baseContext, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n=== EXISTING GENERATED CHAPTERS IN THIS VOLUME ===\n")
	b.WriteString(fmt.Sprintf("The volume must contain %d chapters total. The following %d chapter(s) already exist; continue after them and return only missing chapters.\n",
		totalRequested, len(existing)))
	for _, chapter := range existing {
		b.WriteString(formatChapterBrief(chapter))
	}
	return b.String()
}

func appendNewChapters(existing []models.Chapter, candidates []models.Chapter) []models.Chapter {
	seen := make(map[string]bool, len(existing))
	for _, chapter := range existing {
		seen[chapterIdentity(chapter)] = true
	}
	for _, chapter := range candidates {
		key := chapterIdentity(chapter)
		if key != "" && seen[key] {
			continue
		}
		existing = append(existing, chapter)
		seen[key] = true
	}
	return existing
}

func chapterIdentity(chapter models.Chapter) string {
	key := strings.ToLower(strings.TrimSpace(chapter.Title)) + "|" + strings.ToLower(strings.TrimSpace(chapter.Summary))
	if key == "|" {
		return ""
	}
	return key
}

// GenerateOutlineHierarchical generates a complete outline using hierarchical approach
// First generates skeleton (parts/volumes), then generates chapters for each volume
func (a *ComposeAgent) GenerateOutlineHierarchical(ctx context.Context, setup models.StorySetup, structure models.StoryStructure) (*models.Outline, error) {
	logger.Section("COMPOSE AGENT - Hierarchical Outline Generation")
	logger.Info("This will generate the outline in two phases:")
	logger.Info("  Phase 1: Generate skeleton (parts and volumes)")
	logger.Info("  Phase 2: Generate chapters for each volume")

	// Phase 1: Generate skeleton
	skeletonInput := ComposeSkeletonInput{
		Setup:     setup,
		Structure: structure,
	}
	skeletonOutput, err := a.GenerateSkeleton(ctx, skeletonInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate skeleton: %w", err)
	}

	// Phase 2: Generate chapters for each volume
	outline := &models.Outline{
		Parts: skeletonOutput.Parts,
	}
	logic.NewIDManager(outline).AssignIDsToOutline()
	models.NormalizeOutline(outline)

	return a.GenerateChaptersHierarchical(ctx, setup, structure, outline, nil)
}

// GenerateOutlineHierarchicalAgentSDK generates a full outline with SDK
// workflows for skeleton and per-volume chapter generation.
func (a *ComposeAgent) GenerateOutlineHierarchicalAgentSDK(ctx context.Context, setup models.StorySetup, structure models.StoryStructure) (*models.Outline, error) {
	logger.Section("COMPOSE AGENT SDK - Hierarchical Outline Generation")
	skeletonOutput, err := a.GenerateSkeletonWithAgentSDK(ctx, ComposeSkeletonInput{
		Setup:     setup,
		Structure: structure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate skeleton: %w", err)
	}
	outline := &models.Outline{Parts: skeletonOutput.Parts}
	logic.NewIDManager(outline).AssignIDsToOutline()
	models.NormalizeOutline(outline)
	return a.GenerateChaptersHierarchicalAgentSDK(ctx, setup, structure, outline, nil)
}

// GenerateChaptersHierarchical generates chapters for each volume in hierarchical mode
// Supports incremental generation with save callback
func (a *ComposeAgent) GenerateChaptersHierarchical(ctx context.Context, setup models.StorySetup, structure models.StoryStructure, outline *models.Outline, onVolumeComplete func(*models.Outline, int, int, int)) (*models.Outline, error) {
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	volumeCount := 0

	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			volumeCount++
			volume := &outline.Parts[partIdx].Volumes[volIdx]

			// Skip if already has chapters (resumed generation)
			if len(volume.Chapters) > 0 {
				logger.Info("✓ Volume %d.%d: %s - already has %d chapters (skipping)",
					partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))
				continue
			}

			logger.Info("Generating chapters for Volume %d.%d: %s",
				partIdx+1, volIdx+1, volume.Title)

			// Build context from previous volume (for continuity)
			var outlineContext string
			if partIdx > 0 || volIdx > 0 {
				outlineContext = a.buildHierarchicalContext(outline, partIdx, volIdx)
			}

			// Get previous volume for continuity
			var previousVolume *models.Volume
			if volIdx > 0 {
				previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
			} else if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				if len(prevPart.Volumes) > 0 {
					previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
				}
			}

			chaptersInput := ComposeChaptersInput{
				Setup:          setup,
				Part:           outline.Parts[partIdx],
				Volume:         *volume,
				VolumeIndex:    volumeCount,
				TotalVolumes:   totalVolumes,
				ChaptersPerVol: structure.TargetChapters,
				PreviousVolume: previousVolume,
				OutlineContext: outlineContext,
			}

			chaptersOutput, err := a.GenerateChaptersForVolume(ctx, chaptersInput)
			if err != nil {
				return outline, fmt.Errorf("failed to generate chapters for volume %d.%d: %w",
					partIdx+1, volIdx+1, err)
			}

			// Assign chapters to volume
			volume.Chapters = chaptersOutput.Chapters

			logger.Info("✓ Volume %d.%d: %s - %d chapters generated",
				partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))

			// Call save callback if provided
			if onVolumeComplete != nil {
				onVolumeComplete(outline, partIdx, volIdx, volumeCount)
			}
		}
	}

	// Assign IDs to all elements
	idManager := logic.NewIDManager(outline)
	idManager.AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")

	totalChapters := 0
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			totalChapters += len(vol.Chapters)
		}
	}
	logger.Info("Generated complete outline with %d part(s), %d volume(s), %d chapter(s)",
		len(outline.Parts), structure.TargetVolumes*structure.TargetParts, totalChapters)

	return outline, nil
}

// GenerateChaptersHierarchicalAgentSDK generates chapters one volume at a time
// through SDK workflows and preserves Go-owned IDs/checkpoints.
func (a *ComposeAgent) GenerateChaptersHierarchicalAgentSDK(ctx context.Context, setup models.StorySetup, structure models.StoryStructure, outline *models.Outline, onVolumeComplete func(*models.Outline, int, int, int)) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	totalVolumes := structure.TargetParts * structure.TargetVolumes
	volumeCount := 0

	for partIdx := range outline.Parts {
		for volIdx := range outline.Parts[partIdx].Volumes {
			volumeCount++
			volume := &outline.Parts[partIdx].Volumes[volIdx]
			if len(volume.Chapters) > 0 {
				logger.Info("[ok] Volume %d.%d: %s - already has %d chapters (skipping)",
					partIdx+1, volIdx+1, volume.Title, len(volume.Chapters))
				continue
			}

			logger.Info("Agent SDK generating chapters for Volume %d.%d: %s",
				partIdx+1, volIdx+1, volume.Title)

			var outlineContext string
			if partIdx > 0 || volIdx > 0 {
				outlineContext = a.buildHierarchicalContext(outline, partIdx, volIdx)
			}

			var previousVolume *models.Volume
			if volIdx > 0 {
				previousVolume = &outline.Parts[partIdx].Volumes[volIdx-1]
			} else if partIdx > 0 {
				prevPart := outline.Parts[partIdx-1]
				if len(prevPart.Volumes) > 0 {
					previousVolume = &prevPart.Volumes[len(prevPart.Volumes)-1]
				}
			}

			output, err := a.GenerateChaptersForVolumeWithAgentSDK(ctx, ComposeChaptersInput{
				Setup:          setup,
				Part:           outline.Parts[partIdx],
				Volume:         *volume,
				VolumeIndex:    volumeCount,
				TotalVolumes:   totalVolumes,
				ChaptersPerVol: structure.TargetChapters,
				PreviousVolume: previousVolume,
				OutlineContext: outlineContext,
			})
			if err != nil {
				return outline, fmt.Errorf("failed to generate chapters for volume %d.%d: %w", partIdx+1, volIdx+1, err)
			}

			outline.Parts[partIdx].Volumes[volIdx] = output.Volume
			logic.NewIDManager(outline).AssignIDsToOutline()
			logger.Info("[ok] Agent SDK volume %d.%d: %s - %d chapters generated",
				partIdx+1, volIdx+1, output.Volume.Title, len(output.Volume.Chapters))

			if onVolumeComplete != nil {
				onVolumeComplete(outline, partIdx, volIdx, volumeCount)
			}
		}
	}

	logic.NewIDManager(outline).AssignIDsToOutline()
	logger.Info("Assigned IDs to all outline elements")
	return outline, nil
}

// BuildHierarchicalContext exposes the same continuity context used by full
// hierarchical generation for callers that generate one volume at a time.
func (a *ComposeAgent) BuildHierarchicalContext(outline *models.Outline, partIdx, volIdx int) string {
	return a.buildHierarchicalContext(outline, partIdx, volIdx)
}

// buildHierarchicalContext builds context for hierarchical generation
func (a *ComposeAgent) buildHierarchicalContext(outline *models.Outline, partIdx, volIdx int) string {
	var context strings.Builder

	context.WriteString("=== STORY CONTEXT ===\n\n")

	// Add previous volumes summary
	context.WriteString("Previous Volumes Summary:\n")
	for p := 0; p <= partIdx; p++ {
		for v := 0; v < len(outline.Parts[p].Volumes); v++ {
			if p < partIdx || v < volIdx {
				vol := outline.Parts[p].Volumes[v]
				context.WriteString(fmt.Sprintf("- %s: %s\n", vol.Title, vol.Summary))
				if len(vol.Chapters) > 0 {
					lastChap := vol.Chapters[len(vol.Chapters)-1]
					context.WriteString(fmt.Sprintf("  Last chapter: %s - %s\n", lastChap.Title, lastChap.Summary))
					context.WriteString(fmt.Sprintf("  Closing beat: %s\n", getClosingBeat(lastChap)))
				}
				context.WriteString("\n")
			}
		}
	}
	context.WriteString(a.buildContinuitySnapshot(outline, partIdx, volIdx))

	// Add current part context
	currentPart := outline.Parts[partIdx]
	context.WriteString(fmt.Sprintf("=== CURRENT PART ===\n"))
	context.WriteString(fmt.Sprintf("Part: %s\n", currentPart.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n\n", currentPart.Summary))

	// Add current volume context
	currentVolume := currentPart.Volumes[volIdx]
	context.WriteString(fmt.Sprintf("=== CURRENT VOLUME ===\n"))
	context.WriteString(fmt.Sprintf("Volume: %s\n", currentVolume.Title))
	context.WriteString(fmt.Sprintf("Summary: %s\n", currentVolume.Summary))
	context.WriteString("This volume needs chapters that:\n")
	context.WriteString("1. Follow from the previous volume's ending\n")
	context.WriteString("2. Build toward this volume's summary\n")
	context.WriteString("3. Set up for the next volume (if any)\n")

	return context.String()
}

func (a *ComposeAgent) buildContinuitySnapshot(outline *models.Outline, partIdx, volIdx int) string {
	if outline == nil {
		return ""
	}

	var chapters []models.Chapter
	for p := 0; p <= partIdx && p < len(outline.Parts); p++ {
		for v := 0; v < len(outline.Parts[p].Volumes); v++ {
			if p == partIdx && v >= volIdx {
				break
			}
			chapters = append(chapters, outline.Parts[p].Volumes[v].Chapters...)
		}
	}
	if len(chapters) == 0 {
		return ""
	}

	lastChapter := chapters[len(chapters)-1]
	resources := map[string]int{}
	openMysteries := map[string]string{}
	activeBosses := map[string]string{}
	recentStorylines := map[string]string{}

	for _, chapter := range chapters {
		for _, entry := range chapter.ResourceLedger {
			item := strings.TrimSpace(entry.Item)
			if item != "" {
				resources[item] = entry.End
			}
		}
		for _, planted := range chapter.Mysteries.Planted {
			id := strings.TrimSpace(planted.ID)
			if id != "" {
				openMysteries[id] = clipForPrompt(planted.Clue, 120)
			}
		}
		for _, resolved := range chapter.Mysteries.Resolved {
			delete(openMysteries, strings.TrimSpace(resolved.ID))
		}
		for _, enemy := range chapter.Enemies {
			if !enemy.IsBoss && strings.TrimSpace(enemy.BossID) == "" {
				continue
			}
			id := strings.TrimSpace(enemy.BossID)
			if id == "" {
				id = strings.TrimSpace(enemy.Name)
			}
			if id == "" {
				continue
			}
			status := strings.TrimSpace(enemy.Status)
			if status == "" {
				status = "engaged"
			}
			if status == "defeated" {
				delete(activeBosses, id)
			} else {
				activeBosses[id] = fmt.Sprintf("%s in %s", status, chapter.ID)
			}
		}
		for _, advance := range chapter.StorylineAdvances {
			name := strings.TrimSpace(advance.StorylineName)
			if name == "" {
				continue
			}
			recentStorylines[name] = fmt.Sprintf("%s: %s%s%s",
				nonEmpty(advance.Stage, "progress"),
				clipForPrompt(advance.Change, 120),
				compactLabel("pressure", advance.Pressure),
				compactLabel("consequence", advance.Consequence))
		}
	}

	var b strings.Builder
	b.WriteString("\n=== CONTINUITY SNAPSHOT ===\n")
	b.WriteString(fmt.Sprintf("Last generated chapter: %s %s - %s\n",
		lastChapter.ID, lastChapter.Title, clipForPrompt(lastChapter.Summary, 220)))
	if beats := lastChapter.GetBeats(); len(beats) > 0 {
		b.WriteString(fmt.Sprintf("Last closing beat: %s\n", clipForPrompt(beats[len(beats)-1], 180)))
	}
	if lastChapter.StateAnchor.Cultivation != "" || lastChapter.StateAnchor.Location != "" || len(lastChapter.StateAnchor.Allies) > 0 || len(lastChapter.StateAnchor.Injuries) > 0 || len(lastChapter.StateAnchor.KeyItems) > 0 {
		b.WriteString(fmt.Sprintf("Carry-forward state: cultivation=%s; location=%s; allies=%s; injuries=%s; key_items=%s; notes=%s\n",
			clipForPrompt(lastChapter.StateAnchor.Cultivation, 120),
			clipForPrompt(lastChapter.StateAnchor.Location, 120),
			joinLimited(lastChapter.StateAnchor.Allies, 6),
			joinLimited(lastChapter.StateAnchor.Injuries, 6),
			joinLimited(lastChapter.StateAnchor.KeyItems, 8),
			clipForPrompt(lastChapter.StateAnchor.Notes, 180)))
	}
	if len(resources) > 0 {
		b.WriteString("Resource ledger endings: ")
		b.WriteString(formatIntMap(resources, 8))
		b.WriteString("\n")
	}
	if len(openMysteries) > 0 {
		b.WriteString("Open mysteries to preserve: ")
		b.WriteString(formatStringMap(openMysteries, 6))
		b.WriteString("\n")
	}
	if len(activeBosses) > 0 {
		b.WriteString("Active boss continuity: ")
		b.WriteString(formatStringMap(activeBosses, 6))
		b.WriteString("\n")
	}
	if len(recentStorylines) > 0 {
		b.WriteString("Recent storyline pressure/payoff traces: ")
		b.WriteString(formatStringMap(recentStorylines, 8))
		b.WriteString("\n")
	}
	b.WriteString("Use this snapshot as the starting state for the next generated volume; do not reset resources, injuries, mysteries, bosses, or storyline pressure without an explicit event.\n\n")
	return b.String()
}

func compactLabel(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("; %s=%s", label, clipForPrompt(value, 100))
}

func joinLimited(values []string, limit int) string {
	values = compactStringList(values)
	if len(values) == 0 {
		return "none"
	}
	if limit > 0 && len(values) > limit {
		values = append(values[:limit], fmt.Sprintf("...%d more", len(values)-limit))
	}
	return strings.Join(values, ", ")
}

func compactStringList(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func formatIntMap(values map[string]int, limit int) string {
	keys := sortedKeys(values)
	var parts []string
	for i, key := range keys {
		if limit > 0 && i >= limit {
			parts = append(parts, fmt.Sprintf("...%d more", len(keys)-limit))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, "; ")
}

func formatStringMap(values map[string]string, limit int) string {
	keys := sortedKeys(values)
	var parts []string
	for i, key := range keys {
		if limit > 0 && i >= limit {
			parts = append(parts, fmt.Sprintf("...%d more", len(keys)-limit))
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// getOpeningBeat returns beats[0] or empty string.
func getOpeningBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[0]
	}
	return ""
}

// getClosingBeat returns beats[last] or empty string.
func getClosingBeat(chapter models.Chapter) string {
	if len(chapter.GetBeats()) > 0 {
		return chapter.GetBeats()[len(chapter.GetBeats())-1]
	}
	return ""
}
