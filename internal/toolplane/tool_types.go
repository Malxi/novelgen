// Response and context DTOs shared across the tool plane. Every type here is serialized straight to
// the agent-facing JSON, so field names and omitempty tags are part of the published contract.

package toolplane

import (
	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/memory"
	"novelgen/internal/fiction/domain/project"
	"time"
)

type ToolResponse struct {
	OK       bool                   `json:"ok"`
	Section  string                 `json:"section"`
	Query    map[string]string      `json:"query,omitempty"`
	Count    int                    `json:"count"`
	Results  interface{}            `json:"results,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

type OutlineHit struct {
	Type    string      `json:"type"`
	ID      string      `json:"id"`
	Title   string      `json:"title,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Path    OutlinePath `json:"path"`
	Object  interface{} `json:"object,omitempty"`
	Reasons []string    `json:"reasons,omitempty"`
}

type OutlinePath struct {
	PartID       string `json:"part_id,omitempty"`
	PartTitle    string `json:"part_title,omitempty"`
	VolumeID     string `json:"volume_id,omitempty"`
	VolumeTitle  string `json:"volume_title,omitempty"`
	ChapterID    string `json:"chapter_id,omitempty"`
	ChapterTitle string `json:"chapter_title,omitempty"`
}

type chapterHit struct {
	ID          string          `json:"id"`
	Title       string          `json:"title,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Path        OutlinePath     `json:"path"`
	Outline     *models.Chapter `json:"outline,omitempty"`
	ContentPath string          `json:"content_path,omitempty"`
	Content     string          `json:"content,omitempty"`
	Reasons     []string        `json:"reasons,omitempty"`
}

type chapterParagraphsHit struct {
	ChapterID      string                     `json:"chapter_id"`
	Title          string                     `json:"title,omitempty"`
	ContentPath    string                     `json:"content_path"`
	ContentSHA256  string                     `json:"content_sha256"`
	ParagraphCount int                        `json:"paragraph_count"`
	Paragraphs     []project.ChapterParagraph `json:"paragraphs"`
}

type EventHit struct {
	ChapterID    string       `json:"chapter_id,omitempty"`
	ChapterTitle string       `json:"chapter_title,omitempty"`
	EventIndex   int          `json:"event_index"`
	Path         OutlinePath  `json:"path"`
	Event        models.Event `json:"event"`
	Reasons      []string     `json:"reasons,omitempty"`
}

type EventHitBrief struct {
	ChapterID    string       `json:"chapter_id,omitempty"`
	ChapterTitle string       `json:"chapter_title,omitempty"`
	EventIndex   int          `json:"event_index"`
	Path         *OutlinePath `json:"path,omitempty"`
	Event        EventBrief   `json:"event"`
	Reasons      []string     `json:"reasons,omitempty"`
}

type LogHit struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"`
	Agent       string                 `json:"agent,omitempty"`
	Path        string                 `json:"path"`
	SizeBytes   int64                  `json:"size_bytes"`
	ModifiedAt  time.Time              `json:"modified_at"`
	Summary     map[string]interface{} `json:"summary,omitempty"`
	Preview     string                 `json:"preview,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Truncated   bool                   `json:"truncated,omitempty"`
	DetailQuery string                 `json:"detail_query,omitempty"`
}

type CraftElementContext struct {
	Target           string                 `json:"target"`
	Name             string                 `json:"name"`
	CharacterName    string                 `json:"character_name,omitempty"`
	StorySetup       interface{}            `json:"story_setup,omitempty"`
	ExistingCraft    interface{}            `json:"existing_craft,omitempty"`
	OutlineRefs      interface{}            `json:"outline_refs,omitempty"`
	RelevantChapters interface{}            `json:"relevant_chapters,omitempty"`
	Events           interface{}            `json:"events,omitempty"`
	Navigation       map[string]interface{} `json:"navigation,omitempty"`
	NextActions      []ToolNextAction       `json:"next_actions,omitempty"`
	Stats            map[string]int         `json:"stats,omitempty"`
}

type OutlineVolumeContext struct {
	VolumeID       string                 `json:"volume_id"`
	Path           OutlinePath            `json:"path"`
	StorySetup     interface{}            `json:"story_setup,omitempty"`
	TargetVolume   interface{}            `json:"target_volume,omitempty"`
	PreviousVolume interface{}            `json:"previous_volume,omitempty"`
	NextVolume     interface{}            `json:"next_volume,omitempty"`
	EntityIndex    OutlineVolumeEntities  `json:"entity_index,omitempty"`
	Events         interface{}            `json:"events,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	NextActions    []ToolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type OutlineRepairContext struct {
	Scope           string                 `json:"scope"`
	ID              string                 `json:"id"`
	Path            OutlinePath            `json:"path"`
	IssueCategory   string                 `json:"issue_category,omitempty"`
	Check           *ToolCheckResult       `json:"check,omitempty"`
	IssueContext    []OutlineRepairIssue   `json:"issue_context,omitempty"`
	Current         interface{}            `json:"current,omitempty"`
	ParentVolume    interface{}            `json:"parent_volume,omitempty"`
	PreviousChapter interface{}            `json:"previous_chapter,omitempty"`
	NextChapter     interface{}            `json:"next_chapter,omitempty"`
	Events          interface{}            `json:"events,omitempty"`
	Navigation      map[string]interface{} `json:"navigation,omitempty"`
	Workflow        map[string]interface{} `json:"workflow,omitempty"`
	NextActions     []ToolNextAction       `json:"next_actions,omitempty"`
	Stats           map[string]int         `json:"stats,omitempty"`
}

type OutlineGlobalRepairContext struct {
	IssueCategory  string                 `json:"issue_category,omitempty"`
	Check          *ToolCheckResult       `json:"check,omitempty"`
	IssueContext   []OutlineRepairIssue   `json:"issue_context,omitempty"`
	PatchTask      *outlinePatchTask      `json:"patch_task,omitempty"`
	MysteryThreads []outlineMysteryThread `json:"mystery_threads,omitempty"`
	StorySetup     interface{}            `json:"story_setup,omitempty"`
	Outline        interface{}            `json:"outline,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	Workflow       map[string]interface{} `json:"workflow,omitempty"`
	NextActions    []ToolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type RecapRepairContext struct {
	ChapterID      string                 `json:"chapter_id"`
	Path           OutlinePath            `json:"path"`
	Check          *ToolCheckResult       `json:"check,omitempty"`
	Current        interface{}            `json:"current,omitempty"`
	Outline        interface{}            `json:"outline,omitempty"`
	ChapterExcerpt map[string]string      `json:"chapter_excerpt,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	Workflow       map[string]interface{} `json:"workflow,omitempty"`
	NextActions    []ToolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type ChapterWriteContext struct {
	ChapterID              string                 `json:"chapter_id"`
	Path                   OutlinePath            `json:"path"`
	StorySetup             interface{}            `json:"story_setup,omitempty"`
	ParentVolume           interface{}            `json:"parent_volume,omitempty"`
	TargetChapter          interface{}            `json:"target_chapter,omitempty"`
	PreviousChapter        interface{}            `json:"previous_chapter,omitempty"`
	NextChapter            interface{}            `json:"next_chapter,omitempty"`
	PreviousRecap          interface{}            `json:"previous_recap,omitempty"`
	CurrentRecap           interface{}            `json:"current_recap,omitempty"`
	ExistingChapterExcerpt map[string]string      `json:"existing_chapter_excerpt,omitempty"`
	CreativeHistory        *chapterWriteHistory   `json:"creative_history,omitempty"`
	NarrativeMemory        *memory.ContextResult  `json:"narrative_memory,omitempty"`
	EntityIndex            OutlineVolumeEntities  `json:"entity_index,omitempty"`
	Events                 interface{}            `json:"events,omitempty"`
	Navigation             map[string]interface{} `json:"navigation,omitempty"`
	Workflow               map[string]interface{} `json:"workflow,omitempty"`
	NextActions            []ToolNextAction       `json:"next_actions,omitempty"`
	Stats                  map[string]int         `json:"stats,omitempty"`
}

type chapterWriteHistory struct {
	Counts          map[string]int             `json:"counts,omitempty"`
	RecentResponses []chapterWriteHistoryEntry `json:"recent_responses,omitempty"`
	RecentPrompts   []chapterWriteHistoryEntry `json:"recent_prompts,omitempty"`
	Guidance        []string                   `json:"guidance,omitempty"`
}

type chapterWriteHistoryEntry struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Agent      string    `json:"agent,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
	Preview    string    `json:"preview,omitempty"`
	Truncated  bool      `json:"truncated,omitempty"`
}

type ChapterRepairContext struct {
	ChapterID              string                 `json:"chapter_id"`
	Path                   OutlinePath            `json:"path"`
	IssueCategory          string                 `json:"issue_category,omitempty"`
	Check                  *ToolCheckResult       `json:"check,omitempty"`
	SimulationRepairHints  []simulationRepairHint `json:"simulation_repair_hints,omitempty"`
	TargetChapter          interface{}            `json:"target_chapter,omitempty"`
	ParentVolume           interface{}            `json:"parent_volume,omitempty"`
	PreviousChapter        interface{}            `json:"previous_chapter,omitempty"`
	NextChapter            interface{}            `json:"next_chapter,omitempty"`
	ExistingChapterExcerpt map[string]string      `json:"existing_chapter_excerpt,omitempty"`
	NarrativeMemory        *memory.ContextResult  `json:"narrative_memory,omitempty"`
	EntityIndex            OutlineVolumeEntities  `json:"entity_index,omitempty"`
	Events                 interface{}            `json:"events,omitempty"`
	Navigation             map[string]interface{} `json:"navigation,omitempty"`
	Workflow               map[string]interface{} `json:"workflow,omitempty"`
	NextActions            []ToolNextAction       `json:"next_actions,omitempty"`
	Stats                  map[string]int         `json:"stats,omitempty"`
}

type simulationRepairHint struct {
	IssueType         string   `json:"issue_type"`
	AppliesWhen       string   `json:"applies_when"`
	SimulatorRule     string   `json:"simulator_rule"`
	ProseLevers       []string `json:"prose_levers"`
	DSLSignals        []string `json:"dsl_signals"`
	ValidationPlan    []string `json:"validation_plan"`
	Avoid             []string `json:"avoid"`
	FocusedQueries    []string `json:"focused_queries,omitempty"`
	RelatedIssue      string   `json:"related_issue,omitempty"`
	RelatedSuggestion string   `json:"related_suggestion,omitempty"`
}

type OutlineRepairIssue struct {
	Category            string                 `json:"category,omitempty"`
	TargetID            string                 `json:"target_id,omitempty"`
	TargetName          string                 `json:"target_name,omitempty"`
	Issue               string                 `json:"issue,omitempty"`
	Suggestion          string                 `json:"suggestion,omitempty"`
	Priority            string                 `json:"priority,omitempty"`
	FocusedCheckQuery   string                 `json:"focused_check_query,omitempty"`
	RepairRouteQuery    string                 `json:"repair_route_query,omitempty"`
	RepairContextQuery  string                 `json:"repair_context_query,omitempty"`
	PatchQuery          string                 `json:"patch_query,omitempty"`
	PatchShape          interface{}            `json:"patch_shape,omitempty"`
	PostPatchCheckQuery string                 `json:"post_patch_check_query,omitempty"`
	Evidence            map[string]interface{} `json:"evidence,omitempty"`
}

type outlinePatchTask struct {
	Category            string                 `json:"category,omitempty"`
	TargetID            string                 `json:"target_id,omitempty"`
	TargetName          string                 `json:"target_name,omitempty"`
	Priority            string                 `json:"priority,omitempty"`
	Issue               string                 `json:"issue,omitempty"`
	TaskID              string                 `json:"task_id,omitempty"`
	PatchQuery          string                 `json:"patch_query"`
	PatchShape          interface{}            `json:"patch_shape"`
	StdinPatchJSON      string                 `json:"stdin_patch_json,omitempty"`
	DryRunCommand       string                 `json:"dry_run_command,omitempty"`
	ApplyCommand        string                 `json:"apply_command,omitempty"`
	PostPatchCheckQuery string                 `json:"post_patch_check_query"`
	RepairContextQuery  string                 `json:"repair_context_query,omitempty"`
	StdinRequired       bool                   `json:"stdin_required"`
	MaxPatchAttempts    int                    `json:"max_patch_attempts"`
	MaxApplyAttempts    int                    `json:"max_apply_attempts"`
	DryRunInstruction   string                 `json:"dry_run_instruction"`
	ApplyInstruction    string                 `json:"apply_instruction"`
	StopAfterCheck      bool                   `json:"stop_after_check"`
	ForbiddenQueries    []string               `json:"forbidden_queries,omitempty"`
	ForbiddenCommands   []string               `json:"forbidden_commands,omitempty"`
	Evidence            map[string]interface{} `json:"evidence,omitempty"`
}

type outlineMysteryThread struct {
	ID                                string                 `json:"id"`
	Clue                              string                 `json:"clue,omitempty"`
	Horizon                           string                 `json:"horizon,omitempty"`
	Status                            string                 `json:"status,omitempty"`
	PlantedChapterID                  string                 `json:"planted_chapter_id,omitempty"`
	PlantedChapterTitle               string                 `json:"planted_chapter_title,omitempty"`
	PlantedChapterSummary             string                 `json:"planted_chapter_summary,omitempty"`
	PlantedVolumeID                   string                 `json:"planted_volume_id,omitempty"`
	PlantedVolumeTitle                string                 `json:"planted_volume_title,omitempty"`
	ResolvedChapterIDs                []string               `json:"resolved_chapter_ids,omitempty"`
	SuggestedResolutionChapterID      string                 `json:"suggested_resolution_chapter_id,omitempty"`
	SuggestedResolutionChapterTitle   string                 `json:"suggested_resolution_chapter_title,omitempty"`
	SuggestedResolutionChapterSummary string                 `json:"suggested_resolution_chapter_summary,omitempty"`
	SuggestedResolutionVolumeID       string                 `json:"suggested_resolution_volume_id,omitempty"`
	SuggestedResolutionVolumeTitle    string                 `json:"suggested_resolution_volume_title,omitempty"`
	SuggestedResolutionHint           string                 `json:"suggested_resolution_hint,omitempty"`
	RepairStrategy                    string                 `json:"repair_strategy,omitempty"`
	PatchQuery                        string                 `json:"patch_query,omitempty"`
	PatchShape                        map[string]interface{} `json:"patch_shape,omitempty"`
	PostPatchCheckQuery               string                 `json:"post_patch_check_query,omitempty"`
}

type OutlineVolumeEntities struct {
	Characters []string `json:"characters,omitempty"`
	Items      []string `json:"items,omitempty"`
	Locations  []string `json:"locations,omitempty"`
	Storylines []string `json:"storylines,omitempty"`
}

type StorySetupBrief struct {
	ProjectName    string                 `json:"project_name,omitempty"`
	Genres         []string               `json:"genres,omitempty"`
	Premise        string                 `json:"premise,omitempty"`
	Theme          string                 `json:"theme,omitempty"`
	Rules          []string               `json:"rules,omitempty"`
	TargetAudience string                 `json:"target_audience,omitempty"`
	Tone           string                 `json:"tone,omitempty"`
	POVStyle       string                 `json:"pov_style,omitempty"`
	LongFormPlan   interface{}            `json:"long_form_plan,omitempty"`
	CoreCast       []coreCastBrief        `json:"core_cast,omitempty"`
	Storylines     []storylineBrief       `json:"storylines,omitempty"`
	Premises       []premiseBrief         `json:"premises,omitempty"`
	WorldTimeline  []worldTimelineBrief   `json:"world_timeline,omitempty"`
	WorldResources []worldResourceBrief   `json:"world_resources,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
}

type StorySetupSearchResult struct {
	Query          string                 `json:"query"`
	CoreCast       []coreCastBrief        `json:"core_cast,omitempty"`
	Storylines     []storylineBrief       `json:"storylines,omitempty"`
	Premises       []premiseBrief         `json:"premises,omitempty"`
	WorldTimeline  []worldTimelineBrief   `json:"world_timeline,omitempty"`
	WorldResources []worldResourceBrief   `json:"world_resources,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
}

type longFormPlanBrief struct {
	TargetChapters   int      `json:"target_chapters,omitempty"`
	TargetVolumes    int      `json:"target_volumes,omitempty"`
	MainLoop         string   `json:"main_loop,omitempty"`
	EscalationLadder []string `json:"escalation_ladder,omitempty"`
	ReaderPromises   []string `json:"reader_promises,omitempty"`
	PayoffCadence    string   `json:"payoff_cadence,omitempty"`
	VolumePattern    []string `json:"volume_pattern,omitempty"`
	MidpointMutation string   `json:"midpoint_mutation,omitempty"`
	EndgamePromise   string   `json:"endgame_promise,omitempty"`
}

type coreCastBrief struct {
	ID                 string   `json:"id,omitempty"`
	Name               string   `json:"name,omitempty"`
	Role               string   `json:"role,omitempty"`
	Importance         int      `json:"importance,omitempty"`
	StoryFunction      string   `json:"story_function,omitempty"`
	RelationshipToLead string   `json:"relationship_to_lead,omitempty"`
	RelationshipArc    string   `json:"relationship_arc,omitempty"`
	EntryPhase         string   `json:"entry_phase,omitempty"`
	Payoff             string   `json:"payoff,omitempty"`
	StorylineRefs      []string `json:"storyline_refs,omitempty"`
}

type storylineBrief struct {
	Name               string      `json:"name,omitempty"`
	Type               string      `json:"type,omitempty"`
	Importance         int         `json:"importance,omitempty"`
	Scope              string      `json:"scope,omitempty"`
	Description        string      `json:"description,omitempty"`
	SetupRole          string      `json:"setup_role,omitempty"`
	RepeatablePressure string      `json:"repeatable_pressure,omitempty"`
	PayoffCadence      string      `json:"payoff_cadence,omitempty"`
	FailureMode        string      `json:"failure_mode,omitempty"`
	OpenQuestion       string      `json:"open_question,omitempty"`
	AppealEngine       interface{} `json:"appeal_engine,omitempty"`
}

type premiseBrief struct {
	Name        string      `json:"name,omitempty"`
	Category    string      `json:"category,omitempty"`
	Description string      `json:"description,omitempty"`
	Progression interface{} `json:"progression,omitempty"`
}

type progressionStageBrief struct {
	Level int    `json:"level,omitempty"`
	Name  string `json:"name,omitempty"`
}

type worldTimelineBrief struct {
	Year           string `json:"year,omitempty"`
	Event          string `json:"event,omitempty"`
	Impact         string `json:"impact,omitempty"`
	RelatedMystery string `json:"related_mystery,omitempty"`
}

type worldResourceBrief struct {
	Name        string `json:"name,omitempty"`
	Category    string `json:"category,omitempty"`
	Scarcity    string `json:"scarcity,omitempty"`
	Description string `json:"description,omitempty"`
}

type OutlineBrief struct {
	PartCount    int                    `json:"part_count,omitempty"`
	VolumeCount  int                    `json:"volume_count,omitempty"`
	ChapterCount int                    `json:"chapter_count,omitempty"`
	Parts        []PartBrief            `json:"parts,omitempty"`
	Navigation   map[string]interface{} `json:"navigation,omitempty"`
}

type PartBrief struct {
	ID           string        `json:"id,omitempty"`
	Title        string        `json:"title,omitempty"`
	Summary      string        `json:"summary,omitempty"`
	VolumeCount  int           `json:"volume_count,omitempty"`
	ChapterCount int           `json:"chapter_count,omitempty"`
	Volumes      []VolumeBrief `json:"volumes,omitempty"`
}

type VolumeBrief struct {
	ID             string                       `json:"id,omitempty"`
	Title          string                       `json:"title,omitempty"`
	Summary        string                       `json:"summary,omitempty"`
	ChapterCount   int                          `json:"chapter_count,omitempty"`
	PayoffContract *models.VolumePayoffContract `json:"payoff_contract,omitempty"`
	Chapters       []ChapterBrief               `json:"chapters,omitempty"`
	Navigation     map[string]interface{}       `json:"navigation,omitempty"`
}

type ChapterBrief struct {
	ID                string                    `json:"id,omitempty"`
	Title             string                    `json:"title,omitempty"`
	Summary           string                    `json:"summary,omitempty"`
	Characters        []string                  `json:"characters,omitempty"`
	Location          string                    `json:"location,omitempty"`
	OpeningBeat       string                    `json:"opening_beat,omitempty"`
	ClosingBeat       string                    `json:"closing_beat,omitempty"`
	StateChange       string                    `json:"state_change,omitempty"`
	Conflict          string                    `json:"conflict,omitempty"`
	Pacing            string                    `json:"pacing,omitempty"`
	Timeline          models.ChapterTimeline    `json:"timeline,omitempty"`
	StateAnchor       models.StateAnchor        `json:"state_anchor,omitempty"`
	Events            []EventBrief              `json:"events,omitempty"`
	StorylineAdvances []models.StorylineAdvance `json:"storyline_advances,omitempty"`
	ChapterPayoff     *models.ChapterPayoff     `json:"chapter_payoff,omitempty"`
	EventCount        int                       `json:"event_count,omitempty"`
	AdvanceCount      int                       `json:"advance_count,omitempty"`
}

type EventBrief struct {
	Type       string   `json:"type,omitempty"`
	Characters []string `json:"characters,omitempty"`
	Subject    string   `json:"subject,omitempty"`
	Change     string   `json:"change,omitempty"`
	Details    string   `json:"details,omitempty"`
	Actor      string   `json:"actor,omitempty"`
	Action     string   `json:"action,omitempty"`
	Target     string   `json:"target,omitempty"`
	TargetType string   `json:"target_type,omitempty"`
	Context    string   `json:"context,omitempty"`
	Result     string   `json:"result,omitempty"`
}
