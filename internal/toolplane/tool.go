package toolplane

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/project"
	"novelgen/internal/quality"
	"novelgen/internal/toolcmd"
	"novelgen/internal/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ToolQueryFlags struct {
	Type           string
	ID             string
	Name           string
	EntityType     string
	ChapterID      string
	VolumeID       string
	View           string
	Fields         string
	IncludeContent bool
	Limit          int
}

var ToolProcessStartedAt = time.Now()

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
	EntityIndex            OutlineVolumeEntities  `json:"entity_index,omitempty"`
	Events                 interface{}            `json:"events,omitempty"`
	Navigation             map[string]interface{} `json:"navigation,omitempty"`
	Workflow               map[string]interface{} `json:"workflow,omitempty"`
	NextActions            []ToolNextAction       `json:"next_actions,omitempty"`
	Stats                  map[string]int         `json:"stats,omitempty"`
}

type ToolNextAction struct {
	Step    int    `json:"step"`
	Action  string `json:"action"`
	Purpose string `json:"purpose,omitempty"`
	Command string `json:"command,omitempty"`
	When    string `json:"when,omitempty"`
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
	Parts      []PartBrief            `json:"parts,omitempty"`
	Navigation map[string]interface{} `json:"navigation,omitempty"`
}

type PartBrief struct {
	ID      string        `json:"id,omitempty"`
	Title   string        `json:"title,omitempty"`
	Summary string        `json:"summary,omitempty"`
	Volumes []VolumeBrief `json:"volumes,omitempty"`
}

type VolumeBrief struct {
	ID             string                       `json:"id,omitempty"`
	Title          string                       `json:"title,omitempty"`
	Summary        string                       `json:"summary,omitempty"`
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

var ToolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Project tools for agents",
	Long: `Project tools for agents.

	Query and check commands return JSON and never mutate project state. Patch
commands can mutate project state only when explicitly called with --apply.`,
}

var ToolQueryCmd = &cobra.Command{
	Use:   "query [story-setup|outline|craft|chapter|context|logs]",
	Short: "Query novel project data as JSON",
	Long: `Query novel project data as JSON.

Sections:
  story-setup   Query setup, storylines, premises, cast seeds, resources
  outline       Query outline parts, volumes, chapters, refs, events
  craft         Query crafted characters, items, locations, organizations
  chapter       Query chapter outline records and manuscript text
  context       Query compact workflow-specific context bundles
  logs          Query prompt, response, and agent-live log indexes`,
	Example: `  novelgen tool query story-setup
  novelgen tool query story-setup --type storyline --name "Main arc"
  novelgen tool query outline --type chapter --id chap_001
  novelgen tool query outline --type refs --entity-type character --name Lin
  novelgen tool query outline --type events --entity-type item --name "Star Core"
  novelgen tool query craft --type character --name Lin
  novelgen tool query context --type craft-character --name Lin
  novelgen tool query logs --view index
  novelgen tool query chapter --id chap_001 --content
  novelgen tool query chapter --entity-type location --name "Mine"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runToolQuery,
}

var ToolPatchBufferCmd = &cobra.Command{
	Use:   "patch-buffer [append|show|clear]",
	Short: "Manage a novelgen-owned temporary patch buffer",
	Long: `Manage a novelgen-owned temporary patch buffer.

This command exists for agent workflows that need to pass long chapter text to
tool patch without shell temp files or very long --patch-json arguments. It
stores text under .novelgen/agent-patches and does not mutate story state.`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolPatchBuffer,
}

var ToolRefreshCmd = &cobra.Command{
	Use:   "refresh [chapter-dsl]",
	Short: "Refresh derived project artifacts for agent workflows",
	Long: `Refresh derived project artifacts for agent workflows.

Refresh commands are explicit mutating tools. They do not edit story source
state such as setup, outline, craft, recaps, or chapter markdown; they rebuild
derived files that downstream checks consume.`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolRefresh,
}

var allowedToolPatchBufferOps = map[string]bool{
	"append": true,
	"show":   true,
	"clear":  true,
}

func runToolQuery(cmd *cobra.Command, args []string) error {
	section := ""
	if len(args) > 0 {
		section = NormalizeKey(args[0])
	}
	if section == "" {
		return fmt.Errorf("query section is required: story-setup, outline, craft, chapter, or context")
	}
	queryType := NormalizeKey(ToolQueryFlags.Type)
	queryID, idRepaired := RepairToolLookupText(ToolQueryFlags.ID)
	queryName, nameRepaired := RepairToolLookupText(ToolQueryFlags.Name)
	queryChapterID, chapterIDRepaired := RepairToolLookupText(ToolQueryFlags.ChapterID)
	queryVolumeID, volumeIDRepaired := RepairToolLookupText(ToolQueryFlags.VolumeID)
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Dir()

	ctx := ToolProjectContext{Root: root}
	if err := ctx.load(section); err != nil {
		return err
	}

	var resp ToolResponse
	switch section {
	case "story-setup", "setup":
		resp = QueryStorySetup(ctx, queryType, queryName)
	case "outline":
		resp = queryOutline(ctx, queryType, queryID, ToolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID)
	case "craft":
		resp = QueryCraft(ctx, queryType, queryName)
	case "chapter", "chapters":
		resp = queryChapterSection(ctx, queryType, queryID, ToolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID, ToolQueryFlags.IncludeContent)
	case "context":
		resp = QueryContext(ctx, queryType, queryID, queryName)
	case "logs", "log":
		resp = QueryLogs(ctx.Root, queryType, queryID, queryName, ToolQueryFlags.IncludeContent)
	default:
		return fmt.Errorf("unsupported query section %q", section)
	}
	if idRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --id from %q to %q", ToolQueryFlags.ID, queryID))
	}
	if nameRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --name from %q to %q", ToolQueryFlags.Name, queryName))
	}
	if chapterIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --chapter-id from %q to %q", ToolQueryFlags.ChapterID, queryChapterID))
	}
	if volumeIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --volume-id from %q to %q", ToolQueryFlags.VolumeID, queryVolumeID))
	}

	resp.Section = section
	resp.Query = map[string]string{
		"section":     section,
		"type":        queryType,
		"id":          queryID,
		"name":        queryName,
		"entity_type": ToolQueryFlags.EntityType,
		"chapter_id":  queryChapterID,
		"volume_id":   queryVolumeID,
		"fields":      ToolQueryFlags.Fields,
	}
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["project_root"] = root
	ApplyToolView(&resp, ToolQueryFlags.View)
	ApplyToolFields(&resp, ToolQueryFlags.Fields)
	applyToolLimit(&resp, ToolQueryFlags.Limit)
	return WriteToolJSON(cmd, resp)
}

type ToolProjectContext struct {
	Root          string
	Setup         *models.StorySetup
	Outline       *models.Outline
	Characters    map[string]*models.Character
	Locations     map[string]*models.Location
	Items         map[string]*models.Item
	Organizations map[string]*models.Organization
	RawCharacters map[string]map[string]interface{}
	RawLocations  map[string]map[string]interface{}
	RawItems      map[string]map[string]interface{}
	RawOrgs       map[string]map[string]interface{}
}

func (c *ToolProjectContext) load(kind string) error {
	switch kind {
	case "craft":
		return c.loadElements()
	case "story-setup", "setup":
		if err := c.loadSetup(); err != nil {
			return err
		}
		_ = c.loadOutline()
		return nil
	case "outline":
		if err := c.loadOutline(); err != nil {
			return err
		}
		return nil
	case "chapter", "chapters":
		if err := c.loadOutline(); err != nil {
			return err
		}
		_ = c.loadElements()
		return nil
	case "context":
		if err := c.loadSetup(); err != nil {
			return err
		}
		if err := c.loadOutline(); err != nil {
			return err
		}
		return c.loadElements()
	case "logs", "log":
		return nil
	default:
		return fmt.Errorf("unsupported query section %q", kind)
	}
}

func QueryStorySetup(ctx ToolProjectContext, queryType, name string) ToolResponse {
	if ctx.Setup == nil {
		return ToolResponse{OK: true, Count: 0, Results: nil}
	}
	switch normalizeSetupType(queryType) {
	case "index":
		return setupIndexResponse(ctx.Setup)
	case "", "all":
		return ToolResponse{OK: true, Count: 1, Results: ctx.Setup}
	case "search", "find", "refs":
		return queryStorySetupSearch(ctx.Setup, name)
	case "storyline":
		return queryStorylines(ctx, name)
	case "premise":
		return querySetupSlice("premise", name, ctx.Setup.Premises, func(v models.Premise) string { return v.Name })
	case "core-cast":
		return querySetupSlice("core-cast", name, ctx.Setup.CoreCast, func(v models.CoreCastSeed) string { return v.Name })
	case "resource":
		return querySetupSlice("resource", name, ctx.Setup.WorldResources, func(v models.WorldResource) string { return v.Name })
	case "timeline":
		return querySetupSlice("timeline", name, ctx.Setup.WorldTimeline, func(v models.WorldTimelineEntry) string { return v.Event })
	case "rules":
		// 创作铁律（日志边界、能力边界等）。review/setup 核对时按名字直查，
		// 免得只能靠 "--type search" 撞关键词或整本 --type all 拉全量。
		return querySetupSlice("rules", name, ctx.Setup.Rules, func(v string) string { return v })
	case "long-form-plan":
		count := 0
		if ctx.Setup.LongFormPlan != nil {
			count = 1
		}
		return ToolResponse{OK: true, Count: count, Results: ctx.Setup.LongFormPlan}
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported story-setup type %q", queryType)}}
	}
}

// setupIndexResponse 返回 setup 的轻量索引: 各主要字段的数量与名称列表,
// 供 agent 快速了解设定书全貌而不拉取完整数据(省 token)。
func setupIndexResponse(setup *models.StorySetup) ToolResponse {
	if setup == nil {
		return ToolResponse{OK: true, Count: 0, Results: nil}
	}
	index := map[string]interface{}{
		"project_name":    setup.ProjectName,
		"genres":          setup.Genres,
		"premise":         setup.Premise,
		"theme":           setup.Theme,
		"rules":           setup.Rules,
		"target_audience": setup.TargetAudience,
		"tone":            setup.Tone,
		"pov_style":       setup.POVStyle,
		"core_cast":       setup.CoreCast,
		"storylines":      setup.Storylines,
		"premises":        setup.Premises,
		"world_timeline":  setup.WorldTimeline,
		"world_resources": setup.WorldResources,
		"long_form_plan":  setup.LongFormPlan,
		"core_cast_count": len(setup.CoreCast),
		"storyline_count": len(setup.Storylines),
		"premise_count":   len(setup.Premises),
		"timeline_count":  len(setup.WorldTimeline),
		"resource_count":  len(setup.WorldResources),
		"has_long_form":   setup.LongFormPlan != nil,
	}
	return ToolResponse{OK: true, Count: 1, Results: index}
}

func queryStorySetupSearch(setup *models.StorySetup, name string) ToolResponse {
	name = strings.TrimSpace(name)
	if name == "" {
		return ToolResponse{OK: false, Section: "story-setup", Count: 0, Warnings: []string{"--name is required for story-setup --type search"}}
	}

	const perSectionLimit = 3
	result := StorySetupSearchResult{
		Query: name,
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				fmt.Sprintf("novelgen tool query story-setup --type core-cast --name %q --view brief", name),
				fmt.Sprintf("novelgen tool query story-setup --type storyline --name %q --view brief", name),
				fmt.Sprintf("novelgen tool query story-setup --type premise --name %q --view brief", name),
				fmt.Sprintf("novelgen tool query story-setup --type resource --name %q --view brief", name),
				fmt.Sprintf("novelgen tool query story-setup --type timeline --name %q --view brief", name),
			},
			"hint": "Use the detail queries only when the compact search result is insufficient.",
		},
		Stats: map[string]int{},
	}

	coreCastTotal := 0
	for _, cast := range setup.CoreCast {
		if !textMatches(cast.Name, name) && !objectJSONContains(cast, name) {
			continue
		}
		coreCastTotal++
		if len(result.CoreCast) >= perSectionLimit {
			continue
		}
		result.CoreCast = append(result.CoreCast, coreCastBrief{
			ID:                 cast.ID,
			Name:               cast.Name,
			Role:               cast.Role,
			Importance:         cast.Importance,
			StoryFunction:      clipToolString(cast.StoryFunction, 100),
			RelationshipToLead: clipToolString(cast.RelationshipToLead, 80),
			RelationshipArc:    clipToolString(cast.RelationshipArc, 90),
			EntryPhase:         cast.EntryPhase,
			Payoff:             clipToolString(cast.Payoff, 90),
			StorylineRefs:      cast.StorylineRefs,
		})
	}

	storylineTotal := 0
	for _, storyline := range setup.Storylines {
		if !textMatches(storyline.Name, name) && !objectJSONContains(storyline, name) {
			continue
		}
		storylineTotal++
		if len(result.Storylines) >= perSectionLimit {
			continue
		}
		result.Storylines = append(result.Storylines, storylineBrief{
			Name:               storyline.Name,
			Type:               storyline.Type,
			Importance:         storyline.Importance,
			Scope:              storyline.Scope,
			Description:        clipToolString(storyline.Description, 110),
			SetupRole:          clipToolString(storyline.SetupRole, 80),
			RepeatablePressure: clipToolString(storyline.RepeatablePressure, 80),
			PayoffCadence:      clipToolString(storyline.PayoffCadence, 80),
			FailureMode:        clipToolString(storyline.FailureMode, 80),
			OpenQuestion:       clipToolString(storyline.OpenQuestion, 80),
		})
	}

	premiseTotal := 0
	for _, premise := range setup.Premises {
		if !textMatches(premise.Name, name) && !objectJSONContains(premise, name) {
			continue
		}
		premiseTotal++
		if len(result.Premises) >= perSectionLimit {
			continue
		}
		result.Premises = append(result.Premises, premiseBrief{
			Name:        premise.Name,
			Category:    premise.Category,
			Description: clipToolString(premise.Description, 120),
			Progression: makeProgressionBrief(premise.Progression),
		})
	}

	timelineTotal := 0
	for _, entry := range setup.WorldTimeline {
		if !textMatches(entry.Event, name) && !objectJSONContains(entry, name) {
			continue
		}
		timelineTotal++
		if len(result.WorldTimeline) >= perSectionLimit {
			continue
		}
		result.WorldTimeline = append(result.WorldTimeline, worldTimelineBrief{
			Year:           entry.Year,
			Event:          clipToolString(entry.Event, 100),
			Impact:         clipToolString(entry.Impact, 100),
			RelatedMystery: entry.RelatedMystery,
		})
	}

	resourceTotal := 0
	for _, resource := range setup.WorldResources {
		if !textMatches(resource.Name, name) && !objectJSONContains(resource, name) {
			continue
		}
		resourceTotal++
		if len(result.WorldResources) >= perSectionLimit {
			continue
		}
		result.WorldResources = append(result.WorldResources, worldResourceBrief{
			Name:        resource.Name,
			Category:    resource.Category,
			Scarcity:    resource.Scarcity,
			Description: clipToolString(resource.Description, 110),
		})
	}

	result.Stats["core_cast_total"] = coreCastTotal
	result.Stats["core_cast_returned"] = len(result.CoreCast)
	result.Stats["storylines_total"] = storylineTotal
	result.Stats["storylines_returned"] = len(result.Storylines)
	result.Stats["premises_total"] = premiseTotal
	result.Stats["premises_returned"] = len(result.Premises)
	result.Stats["world_timeline_total"] = timelineTotal
	result.Stats["world_timeline_returned"] = len(result.WorldTimeline)
	result.Stats["world_resources_total"] = resourceTotal
	result.Stats["world_resources_returned"] = len(result.WorldResources)

	total := coreCastTotal + storylineTotal + premiseTotal + timelineTotal + resourceTotal
	warnings := make([]string, 0)
	if coreCastTotal > len(result.CoreCast) {
		warnings = append(warnings, fmt.Sprintf("core_cast truncated from %d to %d", coreCastTotal, len(result.CoreCast)))
	}
	if storylineTotal > len(result.Storylines) {
		warnings = append(warnings, fmt.Sprintf("storylines truncated from %d to %d", storylineTotal, len(result.Storylines)))
	}
	if premiseTotal > len(result.Premises) {
		warnings = append(warnings, fmt.Sprintf("premises truncated from %d to %d", premiseTotal, len(result.Premises)))
	}
	if timelineTotal > len(result.WorldTimeline) {
		warnings = append(warnings, fmt.Sprintf("world_timeline truncated from %d to %d", timelineTotal, len(result.WorldTimeline)))
	}
	if resourceTotal > len(result.WorldResources) {
		warnings = append(warnings, fmt.Sprintf("world_resources truncated from %d to %d", resourceTotal, len(result.WorldResources)))
	}
	return ToolResponse{OK: true, Section: "story-setup", Count: total, Results: result, Warnings: warnings}
}

func querySetupSlice[T any](kind, name string, values []T, nameOf func(T) string) ToolResponse {
	hits := make([]T, 0)
	for _, value := range values {
		if name == "" || textMatches(nameOf(value), name) || objectJSONContains(value, name) {
			hits = append(hits, value)
		}
	}
	return ToolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits}
}

func queryOutline(ctx ToolProjectContext, queryType, id, entityType, name, chapterID, volumeID string) ToolResponse {
	switch normalizeOutlineType(queryType) {
	case "", "all":
		count := 0
		if ctx.Outline != nil {
			count = len(ctx.Outline.Parts)
		}
		return ToolResponse{OK: true, Count: count, Results: ctx.Outline}
	case "part":
		return QueryOutlineNode(ctx.Outline, "part", id)
	case "volume":
		return QueryOutlineNode(ctx.Outline, "volume", id)
	case "chapter":
		return QueryOutlineNode(ctx.Outline, "chapter", id)
	case "refs":
		return QueryOutlineRefs(ctx.Outline, entityType, name, false)
	case "events":
		return QueryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported outline type %q", queryType)}}
	}
}

func QueryCraft(ctx ToolProjectContext, queryType, name string) ToolResponse {
	switch normalizeCraftType(queryType) {
	case "", "all":
		results := map[string]interface{}{
			"characters":    rawOrTyped(ctx.RawCharacters, ctx.Characters),
			"items":         rawOrTyped(ctx.RawItems, ctx.Items),
			"locations":     rawOrTyped(ctx.RawLocations, ctx.Locations),
			"organizations": rawOrTyped(ctx.RawOrgs, ctx.Organizations),
		}
		return ToolResponse{OK: true, Count: len(ctx.Characters) + len(ctx.Items) + len(ctx.Locations) + len(ctx.Organizations), Results: results}
	case "character":
		if ctx.RawCharacters != nil {
			return queryNamedRawElement("character", name, ctx.RawCharacters)
		}
		return queryNamedElement("character", name, ctx.Characters)
	case "item":
		if ctx.RawItems != nil {
			return queryNamedRawElement("item", name, ctx.RawItems)
		}
		return queryNamedElement("item", name, ctx.Items)
	case "location":
		if ctx.RawLocations != nil {
			return queryNamedRawElement("location", name, ctx.RawLocations)
		}
		return queryNamedElement("location", name, ctx.Locations)
	case "organization":
		if ctx.RawOrgs != nil {
			return queryNamedRawElement("organization", name, ctx.RawOrgs)
		}
		return queryNamedElement("organization", name, ctx.Organizations)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported craft type %q", queryType)}}
	}
}

func queryChapterSection(ctx ToolProjectContext, queryType, id, entityType, name, chapterID, volumeID string, includeContent bool) ToolResponse {
	if strings.TrimSpace(id) != "" {
		return queryChapter(ctx, id, includeContent)
	}
	switch normalizeChapterType(queryType) {
	case "", "all":
		if strings.TrimSpace(entityType) == "" && strings.TrimSpace(name) == "" {
			return queryAllChapters(ctx, includeContent)
		}
		return queryChaptersByEntity(ctx, entityType, name, includeContent)
	case "refs":
		return queryChaptersByEntity(ctx, entityType, name, includeContent)
	case "events":
		return QueryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	case "outline":
		return QueryOutlineNode(ctx.Outline, "chapter", id)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported chapter type %q", queryType)}}
	}
}

func QueryContext(ctx ToolProjectContext, queryType, id, name string) ToolResponse {
	contextType := normalizeContextType(queryType)
	switch contextType {
	case "craft-character", "craft-item", "craft-location", "craft-organization":
		return queryCraftElementContext(ctx, strings.TrimPrefix(contextType, "craft-"), name)
	case "outline-volume":
		return queryOutlineVolumeContext(ctx, id, name)
	case "outline-repair":
		return queryOutlineRepairContext(ctx, id, name)
	case "outline-global-repair":
		return QueryOutlineGlobalRepairContext(ctx, name)
	case "recap-repair":
		return queryRecapRepairContext(ctx, id)
	case "chapter-write":
		return queryChapterWriteContext(ctx, id)
	case "chapter-repair":
		return queryChapterRepairContext(ctx, id, name)
	default:
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported context type %q", queryType)}}
	}
}

func queryChapterWriteContext(ctx ToolProjectContext, chapterID string) ToolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-write"}}
	}

	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if chapter.Chapter != nil && idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventsResp.Results, 8)
	entities := collectChapterEntities(*target.Chapter)

	var previousChapter *flatChapter
	var nextChapter *flatChapter
	if targetIndex > 0 {
		previousChapter = &chapters[targetIndex-1]
	}
	if targetIndex+1 < len(chapters) {
		nextChapter = &chapters[targetIndex+1]
	}

	var previousRecap *models.ChapterRecap
	var currentRecap *models.ChapterRecap
	warnings := []string{}
	if ctx.Root != "" {
		store := recap.NewStore(ctx.Root)
		if previousChapter != nil && previousChapter.Chapter != nil {
			if loaded, warning := loadUsableToolContextRecap(ctx.Root, store, previousChapter.Chapter); loaded != nil {
				previousRecap = loaded
			} else if warning != "" {
				warnings = append(warnings, "previous recap ignored: "+warning)
			}
		}
		if loaded, warning := loadUsableToolContextRecap(ctx.Root, store, target.Chapter); loaded != nil {
			currentRecap = loaded
		} else if warning != "" {
			warnings = append(warnings, "current recap ignored: "+warning)
		}
	}

	excerpt, _ := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	creativeHistory := buildChapterWriteHistory(ctx.Root)
	result := ChapterWriteContext{
		ChapterID:              target.Chapter.ID,
		Path:                   target.Path,
		StorySetup:             briefStorySetupResults(ctx.Setup, true),
		TargetChapter:          makeRepairChapterBrief(*target.Chapter),
		PreviousRecap:          makeRecapBrief(previousRecap),
		CurrentRecap:           makeRecapBrief(currentRecap),
		ExistingChapterExcerpt: excerpt,
		CreativeHistory:        creativeHistory,
		EntityIndex:            entities,
		Events:                 briefOutlineResults(events, false),
		Navigation:             buildChapterWriteNavigation(target.Chapter.ID, previousChapter, nextChapter, entities),
		Workflow:               buildChapterWriteWorkflow(target.Chapter.ID, previousChapter),
		NextActions:            buildChapterWriteNextActions(target.Chapter.ID, previousChapter),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"chapter_count":   len(chapters),
			"event_count":     eventsResp.Count,
			"returned_events": len(events),
			"character_count": len(entities.Characters),
			"item_count":      len(entities.Items),
			"location_count":  len(entities.Locations),
			"storyline_count": len(entities.Storylines),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(target.Path.VolumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		result.PreviousChapter = makeContextChapterBrief(*previousChapter.Chapter)
		result.Stats["has_previous_chapter"] = 1
		if previousRecap != nil {
			result.Stats["has_previous_recap"] = 1
		}
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		result.NextChapter = makeContextChapterBrief(*nextChapter.Chapter)
		result.Stats["has_next_chapter"] = 1
	}
	if currentRecap != nil {
		result.Stats["has_current_recap"] = 1
	}
	if excerpt != nil {
		result.Stats["excerpt_fields"] = len(excerpt)
	}
	if creativeHistory != nil {
		result.Stats["history_prompt_count"] = creativeHistory.Counts["prompts"]
		result.Stats["history_response_count"] = creativeHistory.Counts["responses"]
		result.Stats["history_recent_prompts"] = len(creativeHistory.RecentPrompts)
		result.Stats["history_recent_responses"] = len(creativeHistory.RecentResponses)
	}

	if eventsResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventsResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildChapterWriteHistory(root string) *chapterWriteHistory {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	logsRoot := filepath.Join(root, "logs")
	if _, err := os.Stat(logsRoot); err != nil {
		return nil
	}
	counts := map[string]int{}
	responses := []chapterWriteHistoryEntry{}
	prompts := []chapterWriteHistoryEntry{}
	_ = filepath.WalkDir(logsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(logsRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		kind := logKindFromRel(rel)
		if kind != "prompts" && kind != "responses" {
			return nil
		}
		if !info.ModTime().Before(ToolProcessStartedAt.Add(-1 * time.Second)) {
			return nil
		}
		counts[kind]++
		preview, truncated := readLogPreviewForQuery(path, kind, 700)
		if strings.TrimSpace(preview) == "" {
			return nil
		}
		item := chapterWriteHistoryEntry{
			ID:         rel,
			Kind:       kind,
			Agent:      logAgentFromFilename(filepath.Base(path)),
			ModifiedAt: info.ModTime(),
			Preview:    preview,
			Truncated:  truncated,
		}
		if kind == "responses" {
			responses = append(responses, item)
		} else {
			prompts = append(prompts, item)
		}
		return nil
	})
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].ModifiedAt.After(responses[j].ModifiedAt)
	})
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].ModifiedAt.After(prompts[j].ModifiedAt)
	})
	responses = firstNHistoryEntries(responses, 3)
	prompts = firstNHistoryEntries(prompts, 2)
	if len(responses) == 0 && len(prompts) == 0 {
		return nil
	}
	return &chapterWriteHistory{
		Counts:          counts,
		RecentResponses: responses,
		RecentPrompts:   prompts,
		Guidance: []string{
			"这些最近日志预览只作为风格与历史连续性感知；事实仍以 story_setup、target_chapter、events、recaps 和 outline context 为准。",
			"不要在正文中引用或总结日志内容；write generation 阶段不要再运行额外 log query。",
		},
	}
}

func firstNHistoryEntries(entries []chapterWriteHistoryEntry, n int) []chapterWriteHistoryEntry {
	if n <= 0 || len(entries) == 0 {
		return nil
	}
	if len(entries) <= n {
		return entries
	}
	return entries[:n]
}

func buildChapterWriteNavigation(chapterID string, previousChapter, nextChapter *flatChapter, entities OutlineVolumeEntities) map[string]interface{} {
	adjacent := map[string]string{}
	if previousChapter != nil && previousChapter.Chapter != nil {
		adjacent["previous_chapter_id"] = previousChapter.Chapter.ID
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		adjacent["next_chapter_id"] = nextChapter.Chapter.ID
	}
	return map[string]interface{}{
		"target_chapter_id": chapterID,
		"adjacent":          adjacent,
		"entity_names": map[string][]string{
			"characters": firstNStrings(entities.Characters, 5),
			"items":      firstNStrings(entities.Items, 5),
			"locations":  firstNStrings(entities.Locations, 4),
		},
		"tool_policy": []string{
			"chapter-write context already contains the focused facts required for generation.",
			"Do not run extra tool queries, checks, refreshes, patch commands, shell fallbacks, or file reads in write generation.",
			"Return final JSON content to Go; Go handles saving, recap, checks, and derived DSL refresh.",
		},
	}
}

func buildChapterWriteWorkflow(chapterID string, previousChapter *flatChapter) map[string]interface{} {
	stopRules := []string{
		"Use this focused bundle as the complete generation context.",
		"Do not query full setup, full outline, logs, source files, or chapter collections for write generation.",
		"Do not run patch/check/refresh commands in write generation; chapter prose JSON is returned to Go for validation and saving.",
		"Go handles recap generation, quality checks, simulation checks, and derived DSL refresh after saving.",
	}
	workflow := map[string]interface{}{
		"goal":              "Write one chapter using compact outline, adjacent continuity, recaps, events, entity names, and creative_history.",
		"target_chapter_id": chapterID,
		"stop_rules":        stopRules,
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		workflow["previous_chapter_id"] = previousChapter.Chapter.ID
	}
	return workflow
}

func buildChapterWriteNextActions(chapterID string, previousChapter *flatChapter) []ToolNextAction {
	actions := []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_context",
			Purpose: "Read target_chapter, parent_volume, adjacent chapters, recap briefs, entity_index, events, and existing_chapter_excerpt before any extra query.",
		},
		{
			Step:    2,
			Action:  "return_final_json",
			Purpose: "Write the chapter content from the bundled facts and return the schema JSON to Go.",
			When:    "After reading this context; no extra tools are needed for write generation.",
		},
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		actions = append(actions, ToolNextAction{
			Step:    len(actions) + 1,
			Action:  "preserve_previous_continuity",
			Purpose: "Use previous_chapter and previous_recap facts already included in this context; do not run recap checks.",
			When:    "The target opening depends on previous chapter continuity.",
		})
	}
	return actions
}

func queryChapterRepairContext(ctx ToolProjectContext, chapterID, issueCategory string) ToolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-repair"}}
	}

	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if chapter.Chapter != nil && idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventsResp.Results, 8)
	entities := collectChapterEntities(*target.Chapter)

	var previousChapter *flatChapter
	var nextChapter *flatChapter
	if targetIndex > 0 {
		previousChapter = &chapters[targetIndex-1]
	}
	if targetIndex+1 < len(chapters) {
		nextChapter = &chapters[targetIndex+1]
	}

	warnings := []string{}
	var check *ToolCheckResult
	if strings.TrimSpace(ctx.Root) == "" {
		warnings = append(warnings, "project root missing; chapter quality/simulation check unavailable")
	} else {
		var err error
		check, err = runToolChapterCheck(ctx.Root, "all", "chapter", target.Chapter.ID)
		if err != nil {
			warnings = append(warnings, "chapter check unavailable: "+err.Error())
		} else if err := ApplyToolCheckIssueFilters(check, "low", ChapterRepairIssueCategoryFilter(issueCategory), 12); err != nil {
			warnings = append(warnings, "chapter check filter ignored: "+err.Error())
		}
	}

	excerpt, excerptWarnings := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	warnings = append(warnings, excerptWarnings...)

	result := ChapterRepairContext{
		ChapterID:              target.Chapter.ID,
		Path:                   target.Path,
		IssueCategory:          categoryOrAll(issueCategory),
		Check:                  check,
		SimulationRepairHints:  BuildChapterSimulationRepairHints(target.Chapter.ID, check),
		TargetChapter:          makeRepairChapterBrief(*target.Chapter),
		ExistingChapterExcerpt: excerpt,
		EntityIndex:            entities,
		Events:                 briefOutlineResults(events, false),
		Navigation:             BuildChapterRepairNavigation(target.Chapter.ID, issueCategory, entities),
		Workflow:               buildChapterRepairWorkflow(target.Chapter.ID, issueCategory),
		NextActions:            buildChapterRepairNextActions(target.Chapter.ID, issueCategory),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"chapter_count":   len(chapters),
			"event_count":     eventsResp.Count,
			"returned_events": len(events),
			"character_count": len(entities.Characters),
			"item_count":      len(entities.Items),
			"location_count":  len(entities.Locations),
			"storyline_count": len(entities.Storylines),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(target.Path.VolumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if previousChapter != nil && previousChapter.Chapter != nil {
		result.PreviousChapter = makeContextChapterBrief(*previousChapter.Chapter)
		result.Stats["has_previous_chapter"] = 1
	}
	if nextChapter != nil && nextChapter.Chapter != nil {
		result.NextChapter = makeContextChapterBrief(*nextChapter.Chapter)
		result.Stats["has_next_chapter"] = 1
	}
	if excerpt != nil {
		result.Stats["excerpt_fields"] = len(excerpt)
	}
	if check != nil {
		result.Stats["check_issues"] = check.Summary.Total
		result.Stats["returned_check_issues"] = len(check.Issues)
	}
	if eventsResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventsResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func BuildChapterRepairNavigation(chapterID, issueCategory string, entities OutlineVolumeEntities) map[string]interface{} {
	category := ChapterRepairIssueCategoryFilter(issueCategory)
	craftQueries := []string{}
	for _, name := range firstNStrings(entities.Characters, 5) {
		craftQueries = append(craftQueries, fmt.Sprintf("novelgen tool query context --type craft-character --name %q --view brief", name))
	}
	for _, name := range firstNStrings(entities.Items, 5) {
		craftQueries = append(craftQueries, fmt.Sprintf("novelgen tool query context --type craft-item --name %q --view brief", name))
	}
	for _, name := range firstNStrings(entities.Locations, 4) {
		craftQueries = append(craftQueries, fmt.Sprintf("novelgen tool query context --type craft-location --name %q --view brief", name))
	}
	return map[string]interface{}{
		"detail_queries": []string{
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", chapterID),
		},
		"craft_context_queries":     craftQueries,
		"quality_check_query":       toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "quality", Category: category, MinPriority: "low", MaxIssues: 12}),
		"simulation_check_query":    toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", Category: category, MinPriority: "low", MaxIssues: 12}),
		"all_check_query":           toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Category: category, MinPriority: "low", MaxIssues: 12}),
		"refresh_chapter_dsl_query": toolcmd.ChapterDSLRefresh(chapterID),
		"repair_command":            fmt.Sprintf("novelgen write improve --agent-sdk --chapter %q --max-rounds 1", chapterID),
	}
}

func ChapterRepairIssueCategoryFilter(issueCategory string) string {
	category := strings.TrimSpace(issueCategory)
	switch NormalizeKey(category) {
	case "", "all", "simulation":
		return categoryOrAll("") + ",simulation"
	default:
		return category
	}
}

func BuildChapterSimulationRepairHints(chapterID string, check *ToolCheckResult) []simulationRepairHint {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	hints := make([]simulationRepairHint, 0, 2)
	added := map[string]bool{}
	for _, issue := range check.Issues {
		text := simulationIssueText(issue)
		if text == "" {
			continue
		}
		if isCombatBalanceSimulationIssue(text) {
			if !added["combat_balance"] {
				hints = append(hints, combatBalanceSimulationRepairHint(chapterID, issue))
				added["combat_balance"] = true
			}
		}
		if isCombatResultSimulationIssue(text) {
			if !added["combat_result"] {
				hints = append(hints, combatResultSimulationRepairHint(chapterID, issue))
				added["combat_result"] = true
			}
		}
	}
	return hints
}

func simulationIssueText(issue models.ReviewSuggestion) string {
	return strings.ToLower(strings.TrimSpace(strings.Join([]string{
		issue.Category,
		issue.Issue,
		issue.Suggestion,
	}, " ")))
}

func isCombatBalanceSimulationIssue(text string) bool {
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"战斗难度",
		"敌人有效战力",
		"主角基础战力",
		"combat balance",
		"enemy effective power",
		"protagonist power",
		"missing_repair_signals=power_change",
		"missing_repair_signals=breakthrough",
		"missing_repair_signals=gene",
		"missing_repair_signals=mech",
		"missing_repair_signals=equipment_or_item",
		"missing_repair_signals=ally",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if strings.Contains(text, "missing_repair_signals=") {
		for _, marker := range []string{"power_change", "breakthrough", "gene", "mech", "equipment_or_item", "ally"} {
			if strings.Contains(text, marker) {
				return true
			}
		}
	}
	return false
}

func isCombatResultSimulationIssue(text string) bool {
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"叙事性结果",
		"战斗结果",
		"narrative result",
		"combat_result=false",
		"combat_result_on_complete",
		"missing combat result",
		"on_complete narration/result",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func combatBalanceSimulationRepairHint(chapterID string, issue models.ReviewSuggestion) simulationRepairHint {
	return simulationRepairHint{
		IssueType:     "combat_balance",
		AppliesWhen:   "RPG simulation says protagonist power plus structured/tactical/ally bonuses is still below enemy effective power.",
		SimulatorRule: "Enemy power is driven by combat.enemies id/count/level/elite/boss and enemy damage-state text. Protagonist side uses base power plus state_delta growth/gene/mech/equipment/item/ally, then tactical text signals such as 机甲/火种/装甲, 伏击/陷阱/地形/高地/狭道, 三方/混战/互相消耗.",
		ProseLevers: []string{
			"Reduce the same-scene enemy count, level, elite/boss framing, or simultaneous engagement scale if the outline allows it.",
			"Make the fight non-frontal: split enemies, ambush, trap, terrain bottleneck, high ground, narrow passage, third-party consumption, or enemy already damaged.",
			"Explicitly show supported power sources already allowed by setup/outline: mech, fire-core, armor, equipment, item, gene breakthrough, power increase, or ally assistance.",
			"If the protagonist gains strength in this chapter, write a concrete breakthrough/power-change moment before or during the decisive exchange.",
		},
		DSLSignals: []string{
			`combat { enemies = [{id = "enemy_x", count = 1, level = 1}] } must reflect the actual current enemy scale.`,
			`state_delta { target = "char_protagonist" kind = "power_change" delta = 50 note = "..." } for explicit combat-power gains.`,
			`state_delta { target = "char_protagonist" kind = "gene" field = "level|stability" to = "..." note = "..." } for gene progression.`,
			`state_delta { target = "char_protagonist" kind = "mech" field = "form|energy|module|damage" to = "..." note = "..." } for mech state.`,
			`state_delta { target = "item_id" kind = "item|equipment" field = "key_item" to = "..." note = "..." } for usable equipment/items.`,
			`state_delta { target = "char_protagonist" kind = "ally" to = "Ally Name" note = "..." } for concrete ally support.`,
			`on_complete { narration = "..." } should state the combat consequence, not just that the fight happened.`,
		},
		ValidationPlan: []string{
			toolcmd.ChapterDSLRefresh(chapterID),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", MinPriority: "low", MaxIssues: 12}),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
		},
		Avoid: []string{
			"Do not only add adjectives, longer action choreography, or courage lines; the simulator needs count/level/support/tactic/state-result signals.",
			"Do not invent a new ally, weapon, or breakthrough that contradicts setup, outline, craft, or adjacent chapters.",
			"Do not lower stakes by silently deleting the fight; preserve the chapter purpose and visible consequence.",
		},
		FocusedQueries: []string{
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", chapterID),
			fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --fields action,actor,target,target_type,details,result --view brief", chapterID),
		},
		RelatedIssue:      issue.Issue,
		RelatedSuggestion: issue.Suggestion,
	}
}

func combatResultSimulationRepairHint(chapterID string, issue models.ReviewSuggestion) simulationRepairHint {
	return simulationRepairHint{
		IssueType:     "combat_result",
		AppliesWhen:   "RPG simulation sees a combat event without a concrete on_complete narration/result.",
		SimulatorRule: "A combat step needs an on_complete narration or result so simulation can see what changed after the fight.",
		ProseLevers: []string{
			"End the exchange with a visible consequence: injury, retreat, loot, clue, damaged gear, power insight, relationship shift, or tactical position change.",
			"Tie the result to the next paragraph or scene so it is not a detached summary line.",
		},
		DSLSignals: []string{
			`on_complete { narration = "The fight ends with a concrete consequence." }`,
			`state_delta { target = "..." kind = "injury|resource|power_change|plot_thread|transition" ... } when the consequence changes durable state.`,
		},
		ValidationPlan: []string{
			toolcmd.ChapterDSLRefresh(chapterID),
			toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Kind: "simulation", MinPriority: "low", MaxIssues: 12}),
		},
		Avoid: []string{
			"Do not add a vague victory sentence that changes no state.",
			"Do not create rewards or injuries that adjacent chapter continuity cannot support.",
		},
		FocusedQueries: []string{
			fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", chapterID),
		},
		RelatedIssue:      issue.Issue,
		RelatedSuggestion: issue.Suggestion,
	}
}

func buildChapterRepairWorkflow(chapterID, issueCategory string) map[string]interface{} {
	return map[string]interface{}{
		"goal":              "Repair one saved final chapter after tool check reported deterministic prose or RPG simulation issues.",
		"target_chapter_id": chapterID,
		"issue_category":    categoryOrAll(issueCategory),
		"steps": []string{
			"Start from this compact repair bundle instead of querying full setup, full outline, or all chapter files.",
			"Use check.issues and existing_chapter_excerpt to identify the smallest prose change that can fix the reported issue.",
			"If simulation_repair_hints is present, choose one or more prose levers that are supported by the outline and make their DSL signals extractable.",
			"Use craft_context_queries only for entities directly involved in the issue.",
			"Return improved content to the write workflow; do not write chapter markdown directly from an Agent SDK tool.",
			"After Go saves the chapter, refresh derived chapter RPG DSL when allowed, then rerun the all_check_query and fix remaining blocking issues. Simulation checks do not invoke LLM conversion; use post_repair_refresh_query first when chapter prose changed.",
		},
		"post_repair_refresh_query": toolcmd.ChapterDSLRefresh(chapterID),
		"post_repair_check_query":   toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{MinPriority: "low", MaxIssues: 12}),
	}
}

func buildChapterRepairNextActions(chapterID, issueCategory string) []ToolNextAction {
	category := ChapterRepairIssueCategoryFilter(issueCategory)
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_repair_bundle",
			Purpose: "Start from the focused repair facts already returned in this response before querying more context.",
		},
		{
			Step:    2,
			Action:  "query_craft_only_if_needed",
			Purpose: "Use navigation.craft_context_queries only for entities directly involved in the failing issue.",
			When:    "The repair depends on a character/item/location fact not already in this bundle.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: fmt.Sprintf("novelgen tool patch chapter --id %q", chapterID),
			Purpose: "Validate complete revised prose before returning it or applying it.",
		},
		{
			Step:    4,
			Action:  "refresh_derived_dsl",
			Command: toolcmd.ChapterDSLRefresh(chapterID),
			Purpose: "Regenerate derived RPG DSL after prose changes; simulation checks do not invoke LLM conversion.",
			When:    "After chapter prose has changed and refresh is allowed by the workflow.",
		},
		{
			Step:    5,
			Action:  "post_repair_check",
			Command: toolcmd.ChapterCheck(chapterID, toolcmd.ChapterCheckOptions{Category: category, MinPriority: "low", MaxIssues: 12}),
			Purpose: "Verify the targeted repair and catch remaining deterministic prose or simulation issues.",
		},
	}
}

func queryRecapRepairContext(ctx ToolProjectContext, chapterID string) ToolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type recap-repair"}}
	}

	chapters := flattenChapters(ctx.Outline)
	var target flatChapter
	found := false
	for _, chapter := range chapters {
		if chapter.Chapter != nil && idMatches(chapter.Chapter.ID, chapterID) {
			target = chapter
			found = true
			break
		}
	}
	if !found {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	check, err := RunToolRecapCheck(ctx.Root, "quality", "chapter", target.Chapter.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", "recap", 8)

	var current *models.ChapterRecap
	if ctx.Root != "" {
		if loaded, err := recap.NewStore(ctx.Root).Load(target.Chapter.ID); err == nil {
			current = loaded
		}
	}
	excerpt, contentWarnings := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	result := RecapRepairContext{
		ChapterID:      target.Chapter.ID,
		Path:           target.Path,
		Check:          check,
		Current:        makeRecapBrief(current),
		Outline:        makeRepairChapterBrief(*target.Chapter),
		ChapterExcerpt: excerpt,
		Navigation: map[string]interface{}{
			"focused_check_query":       fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", target.Chapter.ID),
			"chapter_outline_query":     fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", target.Chapter.ID),
			"chapter_content_query":     fmt.Sprintf("novelgen tool query chapter --id %q --content --view brief", target.Chapter.ID),
			"patch_query":               fmt.Sprintf("novelgen tool patch recap --id %q", target.Chapter.ID),
			"patch_shape":               map[string]interface{}{"location": "<scene anchor>", "present": []string{"<character>"}, "last_line": "<final visible line>", "next_opening_hint": "<continuation from last_line>"},
			"external_regenerate_query": fmt.Sprintf("novelgen recap gen --agent-sdk --chapter %q --source chapters", target.Chapter.ID),
		},
		Workflow: buildRecapRepairWorkflow(target.Chapter.ID),
		NextActions: buildRecapRepairNextActions(
			target.Chapter.ID,
		),
		Stats: map[string]int{
			"returned_issues": len(check.Issues),
		},
	}
	if current != nil {
		result.Stats["present_count"] = len(current.Present)
		result.Stats["plot_beats_count"] = len(current.PlotBeats)
	}
	if excerpt != nil {
		result.Stats["excerpt_fields"] = len(excerpt)
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: contentWarnings}
}

func buildRecapRepairWorkflow(chapterID string) map[string]interface{} {
	return map[string]interface{}{
		"goal":                         "Repair or regenerate a chapter recap without loading the full project context.",
		"agent_rule":                   "Inside an Agent SDK workflow, do not write files or run recap gen. If patching is allowed, use tool patch recap so Go saves only after typed validation; otherwise return corrected recap JSON to Go.",
		"human_or_go_regenerate_query": fmt.Sprintf("novelgen recap gen --agent-sdk --chapter %q --source chapters", chapterID),
		"patch_query":                  fmt.Sprintf("novelgen tool patch recap --id %q", chapterID),
		"patch_shape":                  map[string]interface{}{"location": "<scene anchor>", "present": []string{"<character>"}, "last_line": "<final visible line>", "next_opening_hint": "<continuation from last_line>"},
		"post_save_check_query":        fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", chapterID),
		"stop_rules": []string{
			"Stop when the recap minimal gate is clean.",
			"Dry-run the recap patch first; apply only when the workflow explicitly allows it.",
			"Do not chase medium next_opening_hint warnings unless the workflow explicitly asks for perfect continuity.",
			"Do not query full setup or full outline for recap-only repair.",
		},
	}
}

func buildRecapRepairNextActions(chapterID string) []ToolNextAction {
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_context",
			Purpose: "Read current recap, outline brief, chapter excerpt, and check.issues from this focused response.",
		},
		{
			Step:    2,
			Action:  "patch_dry_run",
			Command: fmt.Sprintf("novelgen tool patch recap --id %q", chapterID),
			Purpose: "Validate a minimal recap patch without writing.",
			When:    "The workflow allows recap patching or you need to test a candidate recap.",
		},
		{
			Step:    3,
			Action:  "post_save_check",
			Command: fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", chapterID),
			Purpose: "Confirm title, last_line, next_opening_hint, and minimum recap fields after Go saves or patch apply succeeds.",
		},
	}
}

func queryOutlineRepairContext(ctx ToolProjectContext, id, category string) ToolResponse {
	id = strings.TrimSpace(id)
	category = NormalizeKey(category)
	if id == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type outline-repair"}}
	}
	if ToolLooksLikeVolumeID(id) {
		return queryOutlineVolumeRepairContext(ctx, id, category)
	}
	if _, ok := ToolVolumeIDFromChapterID(id); ok {
		return queryOutlineChapterRepairContext(ctx, id, category)
	}
	return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{fmt.Sprintf("cannot infer outline repair scope from id %q", id)}}
}

func QueryOutlineGlobalRepairContext(ctx ToolProjectContext, category string) ToolResponse {
	category = NormalizeKey(category)
	check, err := RunToolOutlineCheck("all", ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 12)
	mysteryThreads := CollectOutlineMysteryThreads(ctx.Outline)
	issueContext := buildOutlineGlobalRepairIssues(check, category, ctx.Outline)
	patchTask := buildOutlineGlobalPatchTask(issueContext)
	result := OutlineGlobalRepairContext{
		IssueCategory:  category,
		Check:          check,
		IssueContext:   issueContext,
		PatchTask:      patchTask,
		MysteryThreads: filterOutlineMysteryThreadsForCategory(mysteryThreads, category),
		StorySetup:     briefStorySetupResults(ctx.Setup, true),
		Outline:        makeOutlineGlobalRepairBrief(ctx.Outline),
		Navigation: map[string]interface{}{
			"focused_check_query":  fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", categoryOrAll(category)),
			"setup_search_query":   "novelgen tool query story-setup --type search --name <keyword> --view brief",
			"volume_context_query": "novelgen tool query context --type outline-volume --id <volume_id> --view brief",
			"patch_task_rule":      "If patch_task is present, use patch_task.dry_run_command and patch_task.apply_command directly; do not query outline-volume, outline-repair, chapter, tool-results, or source files.",
			"mystery_repair_rule":  "For unresolved mysteries, patch only the suggested_resolution chapter named in mystery_threads or issue_context; preserve existing mystery entries.",
			"repair_rule":          "Use issue_context patch routes when present; do not patch broad global issues without a patch_shape.",
		},
		Workflow: map[string]interface{}{
			"goal":                   "Repair or classify global outline quality/simulation issues without loading the full setup, full outline, or all chapters.",
			"post_patch_check_query": fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", categoryOrAll(category)),
			"stop_rule":              "If no issue_context item has patch_query and patch_shape, return review_result with the smallest recommended next workflow instead of inventing a broad patch.",
		},
		NextActions: buildOutlineGlobalRepairNextActions(check, issueContext),
		Stats:       outlineGlobalRepairStats(ctx.Outline, check),
	}
	if len(result.MysteryThreads) > 0 {
		result.Stats["mystery_thread_count"] = len(result.MysteryThreads)
		result.Stats["unresolved_mystery_count"] = countUnresolvedOutlineMysteryThreads(result.MysteryThreads)
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result}
}

func queryOutlineVolumeRepairContext(ctx ToolProjectContext, volumeID, category string) ToolResponse {
	volumes := flattenVolumes(ctx.Outline)
	targetIndex := -1
	for i, volume := range volumes {
		if idMatches(volume.Volume.ID, volumeID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: %q", volumeID)}}
	}
	target := volumes[targetIndex]
	check, err := RunToolOutlineCheck("all", ctx.Setup, ctx.Outline, "volume", target.Volume.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 8)
	events := collectRepairVolumeEvents(target, check)
	returnedEvents := events
	if len(returnedEvents) > 8 {
		returnedEvents = returnedEvents[:8]
	}
	result := OutlineRepairContext{
		Scope:         "volume",
		ID:            target.Volume.ID,
		Path:          target.Path,
		IssueCategory: category,
		Check:         check,
		IssueContext:  buildOutlineRepairIssues(check, "volume", target.Volume.ID, target.Volume.ID, category, nil, target.Volume),
		Current:       makeRepairVolumeBrief(*target.Volume, check),
		Events:        briefOutlineResults(returnedEvents, true),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", target.Volume.ID),
				fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --category %s --min-priority low --max-issues 8", target.Volume.ID, categoryOrAll(category)),
			},
			"patch_query":                 fmt.Sprintf("novelgen tool patch outline --target volume --id %q", target.Volume.ID),
			"patch_shape":                 map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
			"chapter_patch_shape":         map[string]interface{}{"changed_chapters": []map[string]string{{"id": "<chapter_id>", "summary": "<new summary>"}}},
			"validated_patch_instruction": "Dry-run the volume patch first; apply only when the workflow explicitly allows it.",
		},
		Workflow: buildOutlineRepairWorkflow("volume", target.Volume.ID, target.Volume.ID, category, map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}}),
		NextActions: buildOutlineRepairNextActions(
			"volume",
			target.Volume.ID,
			target.Volume.ID,
			category,
		),
		Stats: map[string]int{
			"volume_index":    targetIndex,
			"chapter_count":   len(target.Volume.Chapters),
			"event_count":     len(events),
			"returned_events": len(returnedEvents),
			"returned_issues": len(check.Issues),
		},
	}
	warnings := []string{}
	if len(events) > len(returnedEvents) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", len(events), len(returnedEvents)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func queryOutlineChapterRepairContext(ctx ToolProjectContext, chapterID, category string) ToolResponse {
	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}
	target := chapters[targetIndex]
	volumeID := target.Path.VolumeID
	check, err := RunToolOutlineCheck("all", ctx.Setup, ctx.Outline, "chapter", target.Chapter.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", category, 8)
	eventResp := QueryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventResp.Results, 8)
	result := OutlineRepairContext{
		Scope:         "chapter",
		ID:            target.Chapter.ID,
		Path:          target.Path,
		IssueCategory: category,
		Check:         check,
		IssueContext:  buildOutlineRepairIssues(check, "chapter", target.Chapter.ID, volumeID, category, target.Chapter, nil),
		Current:       makeRepairChapterBrief(*target.Chapter),
		Events:        briefOutlineResults(events, false),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				fmt.Sprintf("novelgen tool query outline --type chapter --id %q --view brief", target.Chapter.ID),
				fmt.Sprintf("novelgen tool query outline --type events --chapter-id %q --view brief", target.Chapter.ID),
				fmt.Sprintf("novelgen tool check all --target outline --scope chapter --id %q --category %s --min-priority low --max-issues 8", target.Chapter.ID, categoryOrAll(category)),
			},
			"patch_query":                 fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
			"patch_shape":                 map[string]interface{}{"changed_chapters": []map[string]string{{"id": target.Chapter.ID, "summary": "<new summary>"}}},
			"validated_patch_instruction": "Patch chapters through the parent volume wrapper; dry-run first and apply only when the workflow explicitly allows it.",
		},
		Workflow: buildOutlineRepairWorkflow("chapter", target.Chapter.ID, volumeID, category, map[string]interface{}{"changed_chapters": []map[string]string{{"id": target.Chapter.ID, "summary": "<new summary>"}}}),
		NextActions: buildOutlineRepairNextActions(
			"chapter",
			target.Chapter.ID,
			volumeID,
			category,
		),
		Stats: map[string]int{
			"chapter_index":   targetIndex,
			"event_count":     eventResp.Count,
			"returned_events": len(events),
			"returned_issues": len(check.Issues),
		},
	}
	if parent := ctx.Outline.GetVolumeByID(volumeID); parent != nil {
		result.ParentVolume = makeContextVolumeBrief(*parent, false)
	}
	if targetIndex > 0 && chapters[targetIndex-1].Path.VolumeID == volumeID {
		result.PreviousChapter = makeContextChapterBrief(*chapters[targetIndex-1].Chapter)
	}
	if targetIndex+1 < len(chapters) && chapters[targetIndex+1].Path.VolumeID == volumeID {
		result.NextChapter = makeContextChapterBrief(*chapters[targetIndex+1].Chapter)
	}
	warnings := []string{}
	if eventResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildOutlineRepairIssues(check *ToolCheckResult, scope, targetID, volumeID, category string, chapter *models.Chapter, volume *models.Volume) []OutlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	issues := make([]OutlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueTargetID := strings.TrimSpace(issue.TargetID)
		if issueTargetID == "" {
			issueTargetID = targetID
		}
		issueCategory := NormalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		issueScope := scope
		patchShape := map[string]interface{}{"summary": "<optional new summary>"}
		if _, ok := ToolVolumeIDFromChapterID(issueTargetID); ok {
			issueScope = "chapter"
			patchShape = map[string]interface{}{"changed_chapters": []map[string]string{{"id": issueTargetID}}}
		}
		evidenceChapter := chapter
		if evidenceChapter == nil && volume != nil {
			evidenceChapter = findVolumeChapterByID(volume, issueTargetID)
		}
		item := OutlineRepairIssue{
			Category:            issue.Category,
			TargetID:            issueTargetID,
			TargetName:          issue.TargetName,
			Issue:               issue.Issue,
			Suggestion:          issue.Suggestion,
			Priority:            issue.Priority,
			FocusedCheckQuery:   buildOutlineFocusedCheckQuery(issueScope, issueTargetID, issueCategory, 8),
			RepairRouteQuery:    fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view index", issueTargetID, categoryOrAll(issueCategory)),
			RepairContextQuery:  fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", issueTargetID, categoryOrAll(issueCategory)),
			PatchQuery:          fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
			PatchShape:          patchShape,
			PostPatchCheckQuery: buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
			Evidence:            buildOutlineRepairEvidence(evidenceChapter),
		}
		issues = append(issues, item)
	}
	return issues
}

func buildOutlineGlobalRepairIssues(check *ToolCheckResult, category string, outline *models.Outline) []OutlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	mysteryThreads := CollectOutlineMysteryThreads(outline)
	issues := make([]OutlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueCategory := NormalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		nav := ToolIssueNavigation(check.Kind, check.Target, check.Scope, check.ID, issue, ToolCheckResultTargetWords(*check))
		item := OutlineRepairIssue{
			Category:            issue.Category,
			TargetID:            issue.TargetID,
			TargetName:          issue.TargetName,
			Issue:               issue.Issue,
			Suggestion:          issue.Suggestion,
			Priority:            issue.Priority,
			FocusedCheckQuery:   StringMapValue(nav, "focused_check_query"),
			RepairRouteQuery:    StringMapValue(nav, "repair_route_query"),
			RepairContextQuery:  StringMapValue(nav, "repair_context_query"),
			PatchQuery:          StringMapValue(nav, "patch_query"),
			PostPatchCheckQuery: StringMapValue(nav, "post_patch_check_query"),
		}
		if shape, ok := nav["patch_shape"]; ok {
			item.PatchShape = shape
		}
		if issueCategory == "mysteries" && item.PatchShape == nil {
			if thread := FirstPatchableOutlineMysteryThread(mysteryThreads); thread != nil {
				item.TargetID = thread.ID
				item.TargetName = thread.ID
				item.RepairContextQuery = fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view brief", categoryOrAll(issueCategory))
				item.PatchQuery = thread.PatchQuery
				item.PatchShape = thread.PatchShape
				item.PostPatchCheckQuery = thread.PostPatchCheckQuery
				item.Evidence = map[string]interface{}{
					"mystery_id":                     thread.ID,
					"planted_chapter_id":             thread.PlantedChapterID,
					"planted_volume_id":              thread.PlantedVolumeID,
					"suggested_resolution_chapter":   thread.SuggestedResolutionChapterID,
					"suggested_resolution_volume_id": thread.SuggestedResolutionVolumeID,
					"repair_strategy":                thread.RepairStrategy,
				}
			}
		}
		issues = append(issues, item)
	}
	return issues
}

func buildOutlineGlobalPatchTask(issues []OutlineRepairIssue) *outlinePatchTask {
	issue := firstPatchableOutlineRepairIssue(issues)
	if issue == nil {
		return nil
	}
	patchQuery := strings.TrimSpace(issue.PatchQuery)
	postCheck := strings.TrimSpace(issue.PostPatchCheckQuery)
	if patchQuery == "" || issue.PatchShape == nil || postCheck == "" {
		return nil
	}
	taskID := outlineGlobalPatchTaskID(issue.Category)
	stdinPatchJSON, dryRunCommand, applyCommand := outlinePatchTaskCommands(patchQuery, issue.PatchShape, taskID)
	return &outlinePatchTask{
		Category:            issue.Category,
		TargetID:            issue.TargetID,
		TargetName:          issue.TargetName,
		Priority:            issue.Priority,
		Issue:               issue.Issue,
		TaskID:              taskID,
		PatchQuery:          patchQuery,
		PatchShape:          issue.PatchShape,
		StdinPatchJSON:      stdinPatchJSON,
		DryRunCommand:       dryRunCommand,
		ApplyCommand:        applyCommand,
		PostPatchCheckQuery: postCheck,
		RepairContextQuery:  issue.RepairContextQuery,
		StdinRequired:       taskID == "",
		MaxPatchAttempts:    1,
		MaxApplyAttempts:    1,
		DryRunInstruction:   "Run dry_run_command exactly once. Do not use --patch-json, placeholder text, temp files, or encoding helper commands.",
		ApplyInstruction:    "Only if dry-run succeeds, run apply_command exactly once, then run post_patch_check_query exactly.",
		StopAfterCheck:      true,
		ForbiddenQueries: []string{
			"novelgen tool query context --type outline-volume",
			"novelgen tool query context --type outline-repair",
			"novelgen tool query context --type chapter-repair",
			"novelgen tool query outline",
			"novelgen tool query chapter",
		},
		ForbiddenCommands: []string{
			"type <claude tool-results>",
			"Get-Content <claude tool-results>",
			"findstr",
			"echo test",
			"python/node/powershell encoding helpers",
			"--patch-json",
			"<json>",
			"<compact-json>",
		},
		Evidence: issue.Evidence,
	}
}

func outlineGlobalPatchTaskID(category string) string {
	category = NormalizeKey(category)
	if category == "" {
		return ""
	}
	return "outline-global-repair:" + category
}

func outlinePatchTaskCommands(patchQuery string, patchShape interface{}, taskID string) (string, string, string) {
	if strings.TrimSpace(taskID) != "" {
		taskCommand := patchQuery + " --task " + shellSingleQuote(taskID)
		return "", taskCommand, taskCommand + " --apply"
	}
	payload, err := json.Marshal(patchShape)
	if err != nil || len(payload) == 0 {
		return "", patchQuery, patchQuery + " --apply"
	}
	stdinPatchJSON := string(payload)
	dryRunCommand := fmt.Sprintf("printf '%%s' %s | %s", shellSingleQuote(stdinPatchJSON), patchQuery)
	applyCommand := dryRunCommand + " --apply"
	return stdinPatchJSON, dryRunCommand, applyCommand
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func CollectOutlineMysteryThreads(outline *models.Outline) []outlineMysteryThread {
	chapters := flattenChapters(outline)
	if len(chapters) == 0 {
		return nil
	}
	byID := map[string]int{}
	threads := make([]outlineMysteryThread, 0)
	for idx, chapter := range chapters {
		if chapter.Chapter == nil {
			continue
		}
		for _, planted := range chapter.Chapter.Mysteries.Planted {
			id := strings.TrimSpace(planted.ID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				if threads[existing].Clue == "" {
					threads[existing].Clue = clipToolString(planted.Clue, 220)
				}
				continue
			}
			thread := outlineMysteryThread{
				ID:                    id,
				Clue:                  clipToolString(planted.Clue, 220),
				Horizon:               planted.Horizon,
				Status:                planted.Status,
				PlantedChapterID:      chapter.Chapter.ID,
				PlantedChapterTitle:   chapter.Chapter.Title,
				PlantedChapterSummary: clipToolString(chapter.Chapter.Summary, 220),
				PlantedVolumeID:       chapter.Path.VolumeID,
				PlantedVolumeTitle:    chapter.Path.VolumeTitle,
			}
			if candidate := suggestedMysteryResolutionChapter(chapters, idx); candidate != nil && candidate.Chapter != nil {
				thread.SuggestedResolutionChapterID = candidate.Chapter.ID
				thread.SuggestedResolutionChapterTitle = candidate.Chapter.Title
				thread.SuggestedResolutionChapterSummary = clipToolString(candidate.Chapter.Summary, 260)
				thread.SuggestedResolutionVolumeID = candidate.Path.VolumeID
				thread.SuggestedResolutionVolumeTitle = candidate.Path.VolumeTitle
				thread.SuggestedResolutionHint = mysteryResolutionHint(id, planted.Clue, candidate.Chapter)
				thread.RepairStrategy = "resolve_in_later_chapter"
				thread.PatchQuery = fmt.Sprintf("novelgen tool patch outline --target volume --id %q", candidate.Path.VolumeID)
				thread.PatchShape = mysteryResolutionPatchShape(id, thread.SuggestedResolutionHint, candidate)
				thread.PostPatchCheckQuery = `novelgen tool check all --target outline --scope all --category mysteries --min-priority low --max-issues 12`
			} else {
				thread.RepairStrategy = "mark_deferred_or_extend_outline"
			}
			byID[id] = len(threads)
			threads = append(threads, thread)
		}
		for _, resolved := range chapter.Chapter.Mysteries.Resolved {
			id := strings.TrimSpace(resolved.ID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				threads[existing].ResolvedChapterIDs = append(threads[existing].ResolvedChapterIDs, chapter.Chapter.ID)
				continue
			}
			thread := outlineMysteryThread{
				ID:                 id,
				ResolvedChapterIDs: []string{chapter.Chapter.ID},
				RepairStrategy:     "resolved_without_known_planted_context",
			}
			byID[id] = len(threads)
			threads = append(threads, thread)
		}
	}
	return threads
}

func suggestedMysteryResolutionChapter(chapters []flatChapter, plantedIndex int) *flatChapter {
	for i := plantedIndex + 1; i < len(chapters); i++ {
		if chapters[i].Chapter != nil && strings.TrimSpace(chapters[i].Chapter.ID) != "" {
			return &chapters[i]
		}
	}
	return nil
}

func mysteryResolutionHint(mysteryID, clue string, candidate *models.Chapter) string {
	if candidate == nil {
		return fmt.Sprintf("Reveal or confirm %s using an already planned later beat.", mysteryID)
	}
	parts := []string{fmt.Sprintf("Resolve %s", mysteryID)}
	if clue = strings.TrimSpace(clue); clue != "" {
		parts = append(parts, "from clue: "+clipToolString(clue, 120))
	}
	if summary := strings.TrimSpace(candidate.Summary); summary != "" {
		parts = append(parts, "inside chapter beat: "+clipToolString(summary, 160))
	}
	return strings.Join(parts, "; ")
}

func mysteryResolutionPatchShape(mysteryID, hint string, candidate *flatChapter) map[string]interface{} {
	if candidate == nil || candidate.Chapter == nil {
		return nil
	}
	resolved := make([]map[string]string, 0, len(candidate.Chapter.Mysteries.Resolved)+1)
	for _, existing := range candidate.Chapter.Mysteries.Resolved {
		if strings.TrimSpace(existing.ID) == "" {
			continue
		}
		resolved = append(resolved, map[string]string{
			"id":         existing.ID,
			"resolution": existing.Resolution,
		})
	}
	resolution := strings.TrimSpace(hint)
	if resolution == "" {
		resolution = "<concise in-story answer revealed or confirmed in this chapter>"
	}
	resolved = append(resolved, map[string]string{
		"id":         mysteryID,
		"resolution": resolution,
	})
	return map[string]interface{}{
		"changed_chapters": []map[string]interface{}{{
			"id": candidate.Chapter.ID,
			"mysteries": map[string]interface{}{
				"resolved": resolved,
			},
		}},
	}
}

func filterOutlineMysteryThreadsForCategory(threads []outlineMysteryThread, category string) []outlineMysteryThread {
	if NormalizeKey(category) != "mysteries" {
		return nil
	}
	if len(threads) > 12 {
		return threads[:12]
	}
	return threads
}

func FirstPatchableOutlineMysteryThread(threads []outlineMysteryThread) *outlineMysteryThread {
	for i := range threads {
		if len(threads[i].ResolvedChapterIDs) > 0 {
			continue
		}
		if strings.TrimSpace(threads[i].PatchQuery) == "" || threads[i].PatchShape == nil {
			continue
		}
		return &threads[i]
	}
	return nil
}

func countUnresolvedOutlineMysteryThreads(threads []outlineMysteryThread) int {
	count := 0
	for _, thread := range threads {
		if strings.TrimSpace(thread.PlantedChapterID) == "" || len(thread.ResolvedChapterIDs) > 0 {
			continue
		}
		count++
	}
	return count
}

func buildOutlineGlobalRepairNextActions(check *ToolCheckResult, issues []OutlineRepairIssue) []ToolNextAction {
	if patchable := firstPatchableOutlineRepairIssue(issues); patchable != nil {
		_, dryRunCommand, applyCommand := outlinePatchTaskCommands(patchable.PatchQuery, patchable.PatchShape, outlineGlobalPatchTaskID(patchable.Category))
		return []ToolNextAction{{
			Step:    1,
			Action:  "use_patch_task",
			Purpose: "Use patch_task.dry_run_command and patch_task.apply_command from this response directly; do not query outline-volume, outline-repair, chapters, tool-results, full setup, full outline, or all chapters.",
		}, {
			Step:    2,
			Action:  "patch_dry_run",
			Command: dryRunCommand,
			Purpose: "Validate patch_task once without writing, using the exact dry_run_command.",
		}, {
			Step:    3,
			Action:  "patch_apply",
			Command: applyCommand,
			Purpose: "Apply only after the same patch task succeeds in dry-run.",
		}, {
			Step:    4,
			Action:  "post_patch_check",
			Command: patchable.PostPatchCheckQuery,
			Purpose: "Confirm the global issue improved and no blocking outline regression was introduced.",
		}}
	}
	if check == nil || len(check.Issues) == 0 {
		return []ToolNextAction{{
			Step:    1,
			Action:  "return_final_json",
			Purpose: "No global outline issues match the current filters.",
		}}
	}
	actions := toolCheckNextActions(*check)
	if len(actions) == 0 {
		return actions
	}
	for i := range actions {
		actions[i].Step = i + 1
	}
	if actions[0].Action == "repair_first_returned_issue" || actions[0].Action == "inspect_first_returned_issue_route" {
		actions[0].Action = "inspect_first_global_issue"
		actions[0].Purpose = "Use issue_context for the first global issue; patch only when it has a concrete patch_query and patch_shape."
	}
	return actions
}

func firstPatchableOutlineRepairIssue(issues []OutlineRepairIssue) *OutlineRepairIssue {
	for i := range issues {
		if strings.TrimSpace(issues[i].PatchQuery) == "" || issues[i].PatchShape == nil {
			continue
		}
		return &issues[i]
	}
	return nil
}

func makeOutlineGlobalRepairBrief(outline *models.Outline) interface{} {
	if outline == nil {
		return nil
	}
	brief := OutlineBrief{Navigation: map[string]interface{}{
		"volume_context_query":  "novelgen tool query context --type outline-volume --id <volume_id> --view brief",
		"chapter_context_query": "novelgen tool query context --type outline-repair --id <chapter_id> --name <category> --view brief",
	}}
	for _, part := range outline.Parts {
		partItem := PartBrief{
			ID:      part.ID,
			Title:   part.Title,
			Summary: clipToolString(part.Summary, 360),
		}
		for _, volume := range part.Volumes {
			partItem.Volumes = append(partItem.Volumes, VolumeBrief{
				ID:      volume.ID,
				Title:   volume.Title,
				Summary: clipToolString(volume.Summary, 260),
			})
		}
		brief.Parts = append(brief.Parts, partItem)
	}
	return brief
}

func outlineGlobalRepairStats(outline *models.Outline, check *ToolCheckResult) map[string]int {
	stats := map[string]int{}
	if outline != nil {
		stats["part_count"] = len(outline.Parts)
		stats["volume_count"] = quality.CountOutlineVolumes(outline)
		stats["chapter_count"] = quality.CountOutlineChapters(outline)
	}
	if check != nil {
		stats["returned_issues"] = len(check.Issues)
		stats["summary_total"] = check.Summary.Total
		stats["summary_medium"] = check.Summary.Medium
		stats["summary_low"] = check.Summary.Low
	}
	return stats
}

func collectRepairVolumeEvents(target flatVolume, check *ToolCheckResult) []EventHit {
	if check == nil || len(check.Issues) == 0 {
		return collectVolumeEvents(target)
	}
	chapterIDs := map[string]bool{}
	for _, issue := range check.Issues {
		if _, ok := ToolVolumeIDFromChapterID(issue.TargetID); ok {
			chapterIDs[issue.TargetID] = true
		}
	}
	if len(chapterIDs) == 0 {
		return collectVolumeEvents(target)
	}
	events := []EventHit{}
	for _, chapter := range target.Volume.Chapters {
		if !chapterIDs[chapter.ID] {
			continue
		}
		for _, event := range chapter.Events {
			events = append(events, EventHit{
				ChapterID:    chapter.ID,
				ChapterTitle: chapter.Title,
				Path: OutlinePath{
					PartID:       target.Path.PartID,
					PartTitle:    target.Path.PartTitle,
					VolumeID:     target.Path.VolumeID,
					VolumeTitle:  target.Path.VolumeTitle,
					ChapterID:    chapter.ID,
					ChapterTitle: chapter.Title,
				},
				Event: event,
			})
		}
	}
	return events
}

func findVolumeChapterByID(volume *models.Volume, chapterID string) *models.Chapter {
	if volume == nil {
		return nil
	}
	for i := range volume.Chapters {
		if idMatches(volume.Chapters[i].ID, chapterID) {
			return &volume.Chapters[i]
		}
	}
	return nil
}

func buildOutlineRepairEvidence(chapter *models.Chapter) map[string]interface{} {
	if chapter == nil {
		return nil
	}
	evidence := map[string]interface{}{}
	if strings.TrimSpace(chapter.StateChange) != "" {
		evidence["state_change"] = clipToolString(chapter.StateChange, 180)
	}
	if strings.TrimSpace(chapter.Conflict) != "" {
		evidence["conflict"] = clipToolString(chapter.Conflict, 180)
	}
	if chapter.ChapterPayoff != nil && !chapter.ChapterPayoff.IsZero() {
		evidence["chapter_payoff"] = chapter.ChapterPayoff
	}
	if advances := compactStorylineAdvances(chapter.StorylineAdvances, 4); len(advances) > 0 {
		evidence["storyline_advances"] = advances
	}
	if len(evidence) == 0 {
		return nil
	}
	return evidence
}

func buildOutlineRepairWorkflow(scope, targetID, volumeID, category string, patchShape interface{}) map[string]interface{} {
	workflow := map[string]interface{}{
		"scope":                  scope,
		"target_id":              targetID,
		"focused_check_query":    buildOutlineFocusedCheckQuery(scope, targetID, category, 8),
		"patch_query":            fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
		"patch_shape":            patchShape,
		"post_patch_check_query": buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
		"stop_rules": []string{
			"Patch only the returned target volume and only the listed changed chapters.",
			"If issue_context is empty after filtering, treat the focused issue as not currently reproduced and return an empty patch.",
			"After a non-empty patch, run dry-run first; apply only when the agent workflow input allows apply_patches=true.",
			"After apply, run post_patch_check_query and repair any directly caused medium+ issue before returning.",
		},
	}
	if scope == "chapter" {
		workflow["parent_volume_id"] = volumeID
	}
	return workflow
}

func buildOutlineRepairNextActions(scope, targetID, volumeID, category string) []ToolNextAction {
	scope = normalizeToolScope(scope)
	if scope == "" {
		scope = "volume"
	}
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_repair_bundle",
			Purpose: "Read check.issues, issue_context, current target, events, and adjacent context from this response.",
		},
		{
			Step:    2,
			Action:  "focused_check",
			Command: buildOutlineFocusedCheckQuery(scope, targetID, category, 8),
			Purpose: "Reproduce the target issue after narrowing category or priority.",
			When:    "The current check output is insufficient or was filtered.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
			Purpose: "Validate a minimal volume patch; chapter repairs must use changed_chapters under the parent volume.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: buildOutlineFocusedCheckQuery("volume", volumeID, "", 12),
			Purpose: "Run after apply to catch medium+ regressions in the whole patched volume.",
		},
	}
}

func buildOutlineFocusedCheckQuery(scope, id, category string, maxIssues int) string {
	scope = normalizeToolScope(scope)
	if scope == "" {
		scope = "volume"
	}
	args := []string{
		"novelgen tool check all --target outline --scope",
		scope,
		"--id",
		fmt.Sprintf("%q", id),
	}
	if strings.TrimSpace(category) != "" {
		args = append(args, "--category", categoryOrAll(category))
	}
	args = append(args, "--min-priority", "low", "--max-issues", fmt.Sprintf("%d", maxIssues))
	return strings.Join(args, " ")
}

func queryOutlineVolumeContext(ctx ToolProjectContext, id, name string) ToolResponse {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" && name == "" {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id or --name is required for context --type outline-volume"}}
	}

	volumes := flattenVolumes(ctx.Outline)
	targetIndex := -1
	for i, volume := range volumes {
		if id != "" && idMatches(volume.Volume.ID, id) {
			targetIndex = i
			break
		}
		if id == "" && (textMatches(volume.Volume.Title, name) || objectJSONContains(volume.Volume, name)) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return ToolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: id=%q name=%q", id, name)}}
	}

	target := volumes[targetIndex]
	events := collectVolumeEvents(target)
	entities := collectVolumeEntities(target)

	result := OutlineVolumeContext{
		VolumeID:     target.Volume.ID,
		Path:         target.Path,
		TargetVolume: makeContextVolumeRouteMap(*target.Volume, true),
		EntityIndex:  entities,
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", target.Volume.ID),
				fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority medium --max-issues 12", target.Volume.ID),
				fmt.Sprintf("novelgen tool patch outline --target volume --id %q", target.Volume.ID),
			},
			"chapter_detail_query":        "novelgen tool query outline --type chapter --id <chapter_id> --view brief",
			"chapter_field_query":         "novelgen tool query outline --type chapter --id <chapter_id> --fields storyline_advances,chapter_payoff,conflict --view brief",
			"chapter_events_query":        "novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			"chapter_event_field_query":   "novelgen tool query outline --type events --chapter-id <chapter_id> --fields result,details,target,target_type --view brief",
			"setup_search_query":          "novelgen tool query story-setup --type search --name <keyword> --view brief",
			"entity_reference_query":      "novelgen tool query outline --type refs --entity-type character|item|location --name <name> --view brief",
			"patch_query":                 fmt.Sprintf("novelgen tool patch outline --target volume --id %q", target.Volume.ID),
			"patch_shape":                 map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
			"post_patch_check_query":      fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority low --max-issues 12", target.Volume.ID),
			"validated_patch_instruction": "Dry-run with tool patch outline first; apply only when the workflow explicitly allows it.",
		},
		NextActions: buildOutlineVolumeNextActions(target.Volume.ID),
		Stats: map[string]int{
			"volume_index":    targetIndex,
			"volume_count":    len(volumes),
			"chapter_count":   len(target.Volume.Chapters),
			"event_count":     len(events),
			"character_count": len(entities.Characters),
			"item_count":      len(entities.Items),
			"location_count":  len(entities.Locations),
			"storyline_count": len(entities.Storylines),
		},
	}
	if targetIndex > 0 {
		result.PreviousVolume = makeContextVolumeRouteMap(*volumes[targetIndex-1].Volume, false)
	}
	if targetIndex+1 < len(volumes) {
		result.NextVolume = makeContextVolumeRouteMap(*volumes[targetIndex+1].Volume, false)
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result}
}

func buildOutlineVolumeNextActions(volumeID string) []ToolNextAction {
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "query_brief_context",
			Command: fmt.Sprintf("novelgen tool query context --type outline-volume --id %q --view brief", volumeID),
			Purpose: "Fetch the focused volume bundle before generating, improving, or patching; index view is only the route map and counts.",
		},
		{
			Step:    2,
			Action:  "focused_check",
			Command: fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority medium --max-issues 12", volumeID),
			Purpose: "Check the volume before deciding which chapters or contracts need repair.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: fmt.Sprintf("novelgen tool patch outline --target volume --id %q", volumeID),
			Purpose: "Validate a minimal volume patch; chapter repairs must use changed_chapters under this parent volume.",
			When:    "The workflow allows outline patching or you need to test a candidate volume repair.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: fmt.Sprintf("novelgen tool check all --target outline --scope volume --id %q --min-priority low --max-issues 12", volumeID),
			Purpose: "Run after patch apply to catch medium+ regressions in the patched volume.",
		},
	}
}

func QueryCraftCharacterContext(ctx ToolProjectContext, name string) ToolResponse {
	return queryCraftElementContext(ctx, "character", name)
}

func queryCraftElementContext(ctx ToolProjectContext, target, name string) ToolResponse {
	target = normalizeCraftType(target)
	name = strings.TrimSpace(name)
	if name == "" {
		return ToolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("--name is required for context --type craft-%s", target)}}
	}

	craftResp := QueryCraft(ctx, target, name)
	refResp := ToolResponse{OK: true, Section: "outline", Results: []OutlineHit{}}
	chapterResp := ToolResponse{OK: true, Section: "chapter", Results: []chapterHit{}}
	if target == "character" || target == "item" || target == "location" {
		refResp = QueryOutlineRefs(ctx.Outline, target, name, false)
		chapterResp = queryChaptersByEntity(ctx, target, name, false)
	}
	eventEntityType := target
	if target == "organization" {
		eventEntityType = ""
	}
	eventResp := QueryEvents(ctx.Outline, "", "", eventEntityType, name)

	refs := limitOutlineHitResults(refResp.Results, 3)
	chapters := limitChapterHitResults(chapterResp.Results, 3)
	events := limitEventHitResults(eventResp.Results, 4)

	result := CraftElementContext{
		Target:           target,
		Name:             name,
		StorySetup:       briefStorySetupForElementContext(ctx.Setup, target, name),
		ExistingCraft:    BriefCraftContextResults(craftResp.Results),
		OutlineRefs:      briefOutlineResults(refs, true),
		RelevantChapters: briefChapterNavigationResults(chapters),
		Events:           briefOutlineResults(events, true),
		Navigation:       craftElementContextNavigation(target, name),
		NextActions:      buildCraftContextNextActions(target, name),
		Stats: map[string]int{
			"existing_craft":    craftResp.Count,
			"outline_refs":      refResp.Count,
			"relevant_chapters": chapterResp.Count,
			"events":            eventResp.Count,
			"returned_refs":     len(refs),
			"returned_chapters": len(chapters),
			"returned_events":   len(events),
		},
	}
	if target == "character" {
		result.CharacterName = name
	}
	warnings := []string{}
	if refResp.Count > len(refs) {
		warnings = append(warnings, fmt.Sprintf("outline_refs truncated from %d to %d", refResp.Count, len(refs)))
	}
	if chapterResp.Count > len(chapters) {
		warnings = append(warnings, fmt.Sprintf("relevant_chapters truncated from %d to %d", chapterResp.Count, len(chapters)))
	}
	if eventResp.Count > len(events) {
		warnings = append(warnings, fmt.Sprintf("events truncated from %d to %d", eventResp.Count, len(events)))
	}
	return ToolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func craftElementContextNavigation(target, name string) map[string]interface{} {
	detailQueries := []string{
		fmt.Sprintf("novelgen tool query story-setup --type search --name %q --view brief", name),
		fmt.Sprintf("novelgen tool query craft --type %s --name %q --view brief", target, name),
	}
	if target == "character" || target == "item" || target == "location" {
		detailQueries = append(detailQueries,
			fmt.Sprintf("novelgen tool query outline --type refs --entity-type %s --name %q --view brief", target, name),
			fmt.Sprintf("novelgen tool query outline --type events --entity-type %s --name %q --view brief", target, name),
		)
	} else {
		detailQueries = append(detailQueries,
			fmt.Sprintf("novelgen tool query outline --type events --name %q --fields result,details,target,target_type,actor,action --view brief --limit 12", name),
		)
	}
	return map[string]interface{}{
		"detail_queries":              detailQueries,
		"chapter_detail_query":        "novelgen tool query outline --type chapter --id <chapter_id> --view brief",
		"chapter_events_query":        "novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
		"schema_check_query":          fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, name),
		"patch_query":                 fmt.Sprintf("novelgen tool patch craft --target %s --id %q", target, name),
		"patch_shape":                 map[string]interface{}{"<field_to_change>": "<new value>"},
		"post_patch_check_query":      fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, name),
		"validated_patch_instruction": "Dry-run with tool patch craft first; apply only when the workflow explicitly allows it.",
	}
}

func buildCraftContextNextActions(target, name string) []ToolNextAction {
	return []ToolNextAction{
		{
			Step:    1,
			Action:  "use_current_context",
			Purpose: "Read story_setup, existing_craft, outline_refs, relevant_chapters, and events from this focused response.",
		},
		{
			Step:    2,
			Action:  "schema_check",
			Command: fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, name),
			Purpose: "Check whether the current or candidate craft object satisfies the typed schema.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: fmt.Sprintf("novelgen tool patch craft --target %s --id %q", target, name),
			Purpose: "Validate a minimal craft patch without writing.",
			When:    "The workflow allows craft patching or you need to test a candidate object.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: fmt.Sprintf("novelgen tool check schema --target craft --scope %s --id %q", target, name),
			Purpose: "Run after patch apply to catch schema, text-safety, and RPG metadata regressions.",
		},
	}
}

func queryAllChapters(ctx ToolProjectContext, includeContent bool) ToolResponse {
	chapters := make([]chapterHit, 0)
	for _, item := range flattenChapters(ctx.Outline) {
		hit := chapterHit{
			ID:      item.Chapter.ID,
			Title:   item.Chapter.Title,
			Summary: item.Chapter.Summary,
			Path:    item.Path,
			Outline: item.Chapter,
		}
		if includeContent {
			hit.ContentPath, hit.Content = LoadToolChapterContent(ctx.Root, item.Chapter.ID)
		}
		chapters = append(chapters, hit)
	}
	return ToolResponse{OK: true, Section: "chapter", Count: len(chapters), Results: chapters}
}

func (c *ToolProjectContext) loadSetup() error {
	path := filepath.Join(c.Root, "story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(path)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	c.Setup = setup
	return nil
}

func (c *ToolProjectContext) loadOutline() error {
	path := filepath.Join(c.Root, "story", "compose", "outline.json")
	outline, err := models.LoadOutline(path)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	c.Outline = outline
	return nil
}

func (c *ToolProjectContext) loadElements() error {
	characters, locations, items, organizations, err := project.LoadAllElements()
	if err != nil {
		return err
	}
	c.Characters = characters
	c.Locations = locations
	c.Items = items
	c.Organizations = organizations
	c.RawCharacters, err = loadRawCraftMap(c.Root, "characters.json")
	if err != nil {
		return err
	}
	c.RawLocations, err = loadRawCraftMap(c.Root, "locations.json")
	if err != nil {
		return err
	}
	c.RawItems, err = loadRawCraftMap(c.Root, "items.json")
	if err != nil {
		return err
	}
	c.RawOrgs, err = loadRawCraftMap(c.Root, "organizations.json")
	if err != nil {
		return err
	}
	return nil
}

func loadRawCraftMap(root, filename string) (map[string]map[string]interface{}, error) {
	path := filepath.Join(root, "story", "craft", filename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}
	data = utils.StripUTF8BOM(data)
	var values map[string]map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}
	return values, nil
}

func queryNamedElement[T any](kind, name string, values map[string]*T) ToolResponse {
	hits := make([]map[string]interface{}, 0)
	for key, value := range values {
		if value == nil {
			continue
		}
		if name == "" || namedObjectMatches(key, value, name) {
			hits = append(hits, map[string]interface{}{
				"key":    key,
				"object": value,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return fmt.Sprint(hits[i]["key"]) < fmt.Sprint(hits[j]["key"])
	})
	return ToolResponse{OK: true, Section: "story-setup", Count: len(hits), Results: hits}
}

func rawObjectMatches(key string, value map[string]interface{}, name string) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	return textMatches(key, name) || objectNameMatches(value, name)
}

func queryNamedRawElement(kind, name string, values map[string]map[string]interface{}) ToolResponse {
	hits := make([]map[string]interface{}, 0)
	for key, value := range values {
		if value == nil {
			continue
		}
		if name == "" || rawObjectMatches(key, value, name) {
			hits = append(hits, map[string]interface{}{
				"type":   kind,
				"key":    key,
				"object": value,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return fmt.Sprint(hits[i]["key"]) < fmt.Sprint(hits[j]["key"])
	})
	return ToolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits}
}

func BriefCraftContextResults(results interface{}) interface{} {
	values, ok := results.([]map[string]interface{})
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{}
		if key, ok := hit["key"]; ok {
			item["key"] = key
		}
		if raw, ok := hit["object"].(map[string]interface{}); ok {
			item["object"] = compactCraftObject(raw)
		} else if object, ok := hit["object"]; ok {
			item["object"] = object
		}
		out = append(out, item)
	}
	return out
}

func briefStorySetupForElementContext(setup *models.StorySetup, target, name string) interface{} {
	if setup == nil {
		return nil
	}
	brief := StorySetupBrief{
		ProjectName:    setup.ProjectName,
		Genres:         setup.Genres,
		Tone:           clipToolString(setup.Tone, 120),
		POVStyle:       setup.POVStyle,
		TargetAudience: clipToolString(setup.TargetAudience, 120),
		LongFormPlan:   makeLongFormPlanBrief(setup.LongFormPlan, true),
		Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query story-setup --type search --name <keyword> --view brief",
				"novelgen tool query story-setup --type storyline --name <name>",
				"novelgen tool query story-setup --type core-cast --name <name>",
				"novelgen tool query story-setup --type premise --name <name>",
				"novelgen tool query story-setup --type resource --name <name>",
			},
		},
	}
	if target == "character" {
		brief.CoreCast = append(brief.CoreCast, matchingCoreCastBriefs(setup.CoreCast, name, 4)...)
	}
	brief.Storylines = append(brief.Storylines, relevantStorylineBriefs(setup.Storylines, brief.CoreCast, 5)...)
	brief.Premises = append(brief.Premises, matchingPremiseBriefs(setup.Premises, name, 5)...)
	brief.WorldResources = append(brief.WorldResources, matchingWorldResourceBriefs(setup.WorldResources, name, 5)...)
	if len(brief.Storylines) == 0 && target != "character" {
		brief.Storylines = append(brief.Storylines, matchingStorylineBriefs(setup.Storylines, name, 3)...)
	}
	return brief
}

func matchingStorylineBriefs(values []models.Storyline, name string, limit int) []storylineBrief {
	out := make([]storylineBrief, 0)
	for _, storyline := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(storyline, name) {
			continue
		}
		out = append(out, storylineBrief{
			Name:        storyline.Name,
			Type:        storyline.Type,
			Importance:  storyline.Importance,
			Scope:       storyline.Scope,
			Description: clipToolString(storyline.Description, 160),
		})
	}
	return out
}

func matchingCoreCastBriefs(values []models.CoreCastSeed, name string, limit int) []coreCastBrief {
	out := make([]coreCastBrief, 0)
	for _, cast := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !textMatches(cast.Name, name) && !objectJSONContains(cast, name) {
			continue
		}
		out = append(out, coreCastBrief{
			ID:                 cast.ID,
			Name:               cast.Name,
			Role:               cast.Role,
			Importance:         cast.Importance,
			StoryFunction:      clipToolString(cast.StoryFunction, 180),
			RelationshipToLead: clipToolString(cast.RelationshipToLead, 120),
			RelationshipArc:    clipToolString(cast.RelationshipArc, 140),
			EntryPhase:         cast.EntryPhase,
			Payoff:             clipToolString(cast.Payoff, 140),
			StorylineRefs:      cast.StorylineRefs,
		})
	}
	return out
}

func relevantStorylineBriefs(values []models.Storyline, cast []coreCastBrief, limit int) []storylineBrief {
	refs := map[string]bool{}
	for _, item := range cast {
		for _, ref := range item.StorylineRefs {
			refs[normalizeSearch(ref)] = true
		}
	}
	out := make([]storylineBrief, 0)
	if len(refs) > 0 {
		for _, storyline := range values {
			if len(out) >= limit {
				break
			}
			if !refs[normalizeSearch(storyline.Name)] {
				continue
			}
			out = append(out, storylineBrief{
				Name:               storyline.Name,
				Type:               storyline.Type,
				Importance:         storyline.Importance,
				Scope:              storyline.Scope,
				Description:        clipToolString(storyline.Description, 160),
				SetupRole:          clipToolString(storyline.SetupRole, 120),
				RepeatablePressure: clipToolString(storyline.RepeatablePressure, 100),
				PayoffCadence:      clipToolString(storyline.PayoffCadence, 100),
				FailureMode:        clipToolString(storyline.FailureMode, 100),
				OpenQuestion:       clipToolString(storyline.OpenQuestion, 100),
			})
		}
	}
	if len(out) > 0 || len(refs) > 0 {
		return out
	}
	for _, storyline := range values {
		if len(out) >= limit {
			break
		}
		out = append(out, storylineBrief{Name: storyline.Name, Type: storyline.Type, Importance: storyline.Importance, Scope: storyline.Scope})
	}
	return out
}

func matchingPremiseBriefs(values []models.Premise, name string, limit int) []premiseBrief {
	out := make([]premiseBrief, 0)
	for _, premise := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(premise, name) {
			continue
		}
		out = append(out, premiseBrief{
			Name:     premise.Name,
			Category: premise.Category,
		})
	}
	return out
}

func matchingWorldResourceBriefs(values []models.WorldResource, name string, limit int) []worldResourceBrief {
	out := make([]worldResourceBrief, 0)
	for _, resource := range values {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(name) != "" && !objectJSONContains(resource, name) {
			continue
		}
		out = append(out, worldResourceBrief{
			Name:     resource.Name,
			Category: resource.Category,
			Scarcity: resource.Scarcity,
		})
	}
	return out
}

func briefChapterNavigationResults(values []chapterHit) interface{} {
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{
			"id":      hit.ID,
			"title":   hit.Title,
			"summary": clipToolString(hit.Summary, 180),
			"path":    hit.Path,
		}
		if len(hit.Reasons) > 0 {
			item["reasons"] = hit.Reasons
		}
		out = append(out, item)
	}
	return out
}

func compactCraftObject(raw map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range []string{
		"name", "aliases", "role_in_story", "rpg_role", "combat_role",
		"type", "description", "appearance", "personality", "background",
		"motivation", "skills", "abilities", "voice", "function", "significance",
		"atmosphere", "goals", "rarity", "power_level", "danger_level",
		"resource_tags", "encounter_tags", "members", "allies", "enemies",
		"dsl_tags", "affiliations", "notes",
	} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			out[key] = clipToolString(typed, 260)
		case []interface{}:
			out[key] = clipInterfaceStrings(typed, 6, 120)
		default:
			out[key] = value
		}
	}
	out["available_fields"] = sortedMapKeys(raw)
	return out
}

func clipInterfaceStrings(values []interface{}, maxItems, maxRunes int) []interface{} {
	if maxItems > 0 && len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, clipToolString(text, maxRunes))
			continue
		}
		out = append(out, value)
	}
	return out
}

func sortedMapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func rawOrTyped[T any](raw map[string]map[string]interface{}, typed map[string]*T) interface{} {
	if raw != nil {
		return raw
	}
	return typed
}

func QueryOutlineNode(outline *models.Outline, nodeType, id string) ToolResponse {
	hits := make([]OutlineHit, 0)
	for _, part := range outline.Parts {
		partPath := OutlinePath{PartID: part.ID, PartTitle: part.Title}
		if nodeType == "part" && idMatches(part.ID, id) {
			hits = append(hits, OutlineHit{Type: "part", ID: part.ID, Title: part.Title, Summary: part.Summary, Path: partPath, Object: part})
		}
		for _, volume := range part.Volumes {
			volumePath := partPath
			volumePath.VolumeID = volume.ID
			volumePath.VolumeTitle = volume.Title
			if nodeType == "volume" && idMatches(volume.ID, id) {
				hits = append(hits, OutlineHit{Type: "volume", ID: volume.ID, Title: volume.Title, Summary: volume.Summary, Path: volumePath, Object: volume})
			}
			for _, chapter := range volume.Chapters {
				chapterPath := volumePath
				chapterPath.ChapterID = chapter.ID
				chapterPath.ChapterTitle = chapter.Title
				if nodeType == "chapter" && idMatches(chapter.ID, id) {
					hits = append(hits, OutlineHit{Type: "chapter", ID: chapter.ID, Title: chapter.Title, Summary: chapter.Summary, Path: chapterPath, Object: chapter})
				}
			}
		}
	}
	return ToolResponse{OK: true, Section: "outline", Count: len(hits), Results: hits}
}

func queryChapter(ctx ToolProjectContext, id string, includeContent bool) ToolResponse {
	if strings.TrimSpace(id) == "" {
		return ToolResponse{OK: false, Count: 0, Warnings: []string{"--id is required for chapter queries"}}
	}
	for _, chapter := range flattenChapters(ctx.Outline) {
		if !idMatches(chapter.Chapter.ID, id) {
			continue
		}
		hit := chapterHit{
			ID:      chapter.Chapter.ID,
			Title:   chapter.Chapter.Title,
			Summary: chapter.Chapter.Summary,
			Path:    chapter.Path,
			Outline: chapter.Chapter,
		}
		if includeContent {
			hit.ContentPath, hit.Content = LoadToolChapterContent(ctx.Root, chapter.Chapter.ID)
		}
		return ToolResponse{OK: true, Count: 1, Results: []chapterHit{hit}}
	}
	return ToolResponse{OK: true, Count: 0, Results: []chapterHit{}}
}

func QueryOutlineRefs(outline *models.Outline, entityType, name string, chaptersOnly bool) ToolResponse {
	hits := make([]OutlineHit, 0)
	for _, chapter := range flattenChapters(outline) {
		reasons := chapterEntityReasons(chapter.Chapter, entityType, name)
		if len(reasons) == 0 {
			continue
		}
		hits = append(hits, OutlineHit{
			Type:    "chapter",
			ID:      chapter.Chapter.ID,
			Title:   chapter.Chapter.Title,
			Summary: chapter.Chapter.Summary,
			Path:    chapter.Path,
			Object:  chapter.Chapter,
			Reasons: reasons,
		})
	}
	if chaptersOnly {
		return ToolResponse{OK: true, Count: len(hits), Results: hits}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}

func queryChaptersByEntity(ctx ToolProjectContext, entityType, name string, includeContent bool) ToolResponse {
	outlineResp := QueryOutlineRefs(ctx.Outline, entityType, name, true)
	outlineHits, _ := outlineResp.Results.([]OutlineHit)
	chapters := make([]chapterHit, 0, len(outlineHits))
	for _, hit := range outlineHits {
		chapter := ctx.Outline.GetChapterByID(hit.ID)
		chapterHit := chapterHit{
			ID:      hit.ID,
			Title:   hit.Title,
			Summary: hit.Summary,
			Path:    hit.Path,
			Outline: chapter,
			Reasons: hit.Reasons,
		}
		if includeContent {
			chapterHit.ContentPath, chapterHit.Content = LoadToolChapterContent(ctx.Root, hit.ID)
		}
		chapters = append(chapters, chapterHit)
	}
	return ToolResponse{OK: true, Count: len(chapters), Results: chapters}
}

func queryStorylines(ctx ToolProjectContext, name string) ToolResponse {
	type storylineHit struct {
		Source  string      `json:"source"`
		Name    string      `json:"name"`
		Object  interface{} `json:"object"`
		Chapter string      `json:"chapter_id,omitempty"`
		Title   string      `json:"chapter_title,omitempty"`
		Path    OutlinePath `json:"path,omitempty"`
	}
	hits := make([]storylineHit, 0)
	if ctx.Setup != nil {
		for _, storyline := range ctx.Setup.Storylines {
			if name == "" || textMatches(storyline.Name, name) {
				hits = append(hits, storylineHit{Source: "setup", Name: storyline.Name, Object: storyline})
			}
		}
	}
	if ctx.Outline != nil {
		for _, chapter := range flattenChapters(ctx.Outline) {
			for _, advance := range chapter.Chapter.StorylineAdvances {
				if name == "" || textMatches(advance.StorylineName, name) {
					hits = append(hits, storylineHit{
						Source:  "chapter_advance",
						Name:    advance.StorylineName,
						Object:  advance,
						Chapter: chapter.Chapter.ID,
						Title:   chapter.Chapter.Title,
						Path:    chapter.Path,
					})
				}
			}
			for _, event := range chapter.Chapter.Events {
				if event.GetTargetType() != models.TargetTypeStoryline && event.Type != models.EventTypeStoryline {
					continue
				}
				target := event.GetTarget()
				if name == "" || textMatches(target, name) {
					hits = append(hits, storylineHit{
						Source:  "event",
						Name:    target,
						Object:  event,
						Chapter: chapter.Chapter.ID,
						Title:   chapter.Chapter.Title,
						Path:    chapter.Path,
					})
				}
			}
		}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}

func QueryEvents(outline *models.Outline, chapterID, volumeID, entityType, name string) ToolResponse {
	hits := make([]EventHit, 0)
	for _, chapter := range flattenChapters(outline) {
		if strings.TrimSpace(volumeID) != "" && !idMatches(chapter.Path.VolumeID, volumeID) {
			continue
		}
		if strings.TrimSpace(chapterID) != "" && !idMatches(chapter.Chapter.ID, chapterID) {
			continue
		}
		for eventIndex, event := range chapter.Chapter.Events {
			reasons := eventReasons(event, entityType, name)
			if strings.TrimSpace(entityType) != "" || strings.TrimSpace(name) != "" {
				if len(reasons) == 0 {
					continue
				}
			}
			hits = append(hits, EventHit{
				ChapterID:    chapter.Chapter.ID,
				ChapterTitle: chapter.Chapter.Title,
				EventIndex:   eventIndex,
				Path:         chapter.Path,
				Event:        event,
				Reasons:      reasons,
			})
		}
	}
	return ToolResponse{OK: true, Count: len(hits), Results: hits}
}

type flatChapter struct {
	Chapter *models.Chapter
	Path    OutlinePath
}

type flatVolume struct {
	Volume *models.Volume
	Path   OutlinePath
}

func flattenVolumes(outline *models.Outline) []flatVolume {
	if outline == nil {
		return nil
	}
	result := make([]flatVolume, 0)
	for _, part := range outline.Parts {
		base := OutlinePath{PartID: part.ID, PartTitle: part.Title}
		for i := range part.Volumes {
			volume := &part.Volumes[i]
			path := base
			path.VolumeID = volume.ID
			path.VolumeTitle = volume.Title
			result = append(result, flatVolume{Volume: volume, Path: path})
		}
	}
	return result
}

func flattenChapters(outline *models.Outline) []flatChapter {
	if outline == nil {
		return nil
	}
	result := make([]flatChapter, 0)
	for _, part := range outline.Parts {
		base := OutlinePath{PartID: part.ID, PartTitle: part.Title}
		for _, volume := range part.Volumes {
			volumePath := base
			volumePath.VolumeID = volume.ID
			volumePath.VolumeTitle = volume.Title
			for i := range volume.Chapters {
				chapter := &volume.Chapters[i]
				path := volumePath
				path.ChapterID = chapter.ID
				path.ChapterTitle = chapter.Title
				result = append(result, flatChapter{Chapter: chapter, Path: path})
			}
		}
	}
	return result
}

func collectVolumeEvents(volume flatVolume) []EventHit {
	if volume.Volume == nil {
		return nil
	}
	hits := make([]EventHit, 0)
	for _, chapter := range volume.Volume.Chapters {
		chapterPath := volume.Path
		chapterPath.ChapterID = chapter.ID
		chapterPath.ChapterTitle = chapter.Title
		for eventIndex, event := range chapter.Events {
			hits = append(hits, EventHit{
				ChapterID:    chapter.ID,
				ChapterTitle: chapter.Title,
				EventIndex:   eventIndex,
				Path:         chapterPath,
				Event:        event,
			})
		}
	}
	return hits
}

func collectVolumeEntities(volume flatVolume) OutlineVolumeEntities {
	if volume.Volume == nil {
		return OutlineVolumeEntities{}
	}
	characters := make([]string, 0)
	items := make([]string, 0)
	locations := make([]string, 0)
	storylines := make([]string, 0)
	for _, chapter := range volume.Volume.Chapters {
		characters = append(characters, chapter.Characters...)
		if strings.TrimSpace(chapter.Location) != "" {
			locations = append(locations, chapter.Location)
		}
		if strings.TrimSpace(chapter.StateAnchor.Location) != "" {
			locations = append(locations, chapter.StateAnchor.Location)
		}
		items = append(items, chapter.StateAnchor.KeyItems...)
		for _, advance := range chapter.StorylineAdvances {
			if strings.TrimSpace(advance.StorylineName) != "" {
				storylines = append(storylines, advance.StorylineName)
			}
		}
		for _, event := range chapter.Events {
			characters = append(characters, event.Characters...)
			if actor := strings.TrimSpace(event.GetActor()); actor != "" {
				characters = append(characters, actor)
			}
			target := strings.TrimSpace(event.GetTarget())
			switch event.GetTargetType() {
			case models.TargetTypeCharacter:
				characters = append(characters, target)
			case models.TargetTypeItem:
				items = append(items, target)
			case models.TargetTypeLocation:
				locations = append(locations, target)
			case models.TargetTypeStoryline:
				storylines = append(storylines, target)
			}
		}
	}
	return OutlineVolumeEntities{
		Characters: clipToolStrings(utils.CompactStrings(characters), 25, 70),
		Items:      clipToolStrings(utils.CompactStrings(items), 25, 70),
		Locations:  clipToolStrings(utils.CompactStrings(locations), 25, 70),
		Storylines: clipToolStrings(utils.CompactStrings(storylines), 25, 70),
	}
}

func collectChapterEntities(chapter models.Chapter) OutlineVolumeEntities {
	characters := append([]string{}, chapter.Characters...)
	items := append([]string{}, chapter.StateAnchor.KeyItems...)
	locations := []string{}
	storylines := []string{}
	if strings.TrimSpace(chapter.Location) != "" {
		locations = append(locations, chapter.Location)
	}
	if strings.TrimSpace(chapter.StateAnchor.Location) != "" {
		locations = append(locations, chapter.StateAnchor.Location)
	}
	for _, advance := range chapter.StorylineAdvances {
		if strings.TrimSpace(advance.StorylineName) != "" {
			storylines = append(storylines, advance.StorylineName)
		}
	}
	for _, event := range chapter.Events {
		characters = append(characters, event.Characters...)
		if actor := strings.TrimSpace(event.GetActor()); actor != "" {
			characters = append(characters, actor)
		}
		target := strings.TrimSpace(event.GetTarget())
		switch event.GetTargetType() {
		case models.TargetTypeCharacter:
			characters = append(characters, target)
		case models.TargetTypeItem:
			items = append(items, target)
		case models.TargetTypeLocation:
			locations = append(locations, target)
		case models.TargetTypeStoryline:
			storylines = append(storylines, target)
		}
	}
	return OutlineVolumeEntities{
		Characters: clipToolStrings(utils.CompactStrings(characters), 18, 70),
		Items:      clipToolStrings(utils.CompactStrings(items), 18, 70),
		Locations:  clipToolStrings(utils.CompactStrings(locations), 12, 70),
		Storylines: clipToolStrings(utils.CompactStrings(storylines), 12, 70),
	}
}

func chapterEntityReasons(chapter *models.Chapter, entityType, name string) []string {
	if chapter == nil {
		return nil
	}
	entityType = normalizeEntityType(entityType)
	switch entityType {
	case "character":
		return characterChapterReasons(chapter, name)
	case "item":
		return itemChapterReasons(chapter, name)
	case "location":
		return locationChapterReasons(chapter, name)
	case "storyline":
		return storylineChapterReasons(chapter, name)
	default:
		reasons := make([]string, 0)
		reasons = append(reasons, prefixedReasons("character", characterChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("item", itemChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("location", locationChapterReasons(chapter, name))...)
		return utils.CompactStrings(reasons)
	}
}

func characterChapterReasons(chapter *models.Chapter, name string) []string {
	var reasons []string
	if anyTextMatches(chapter.Characters, name) {
		reasons = append(reasons, "chapter.characters")
	}
	for _, scene := range chapter.Scenes {
		if anyTextMatches(scene.Characters, name) || textMatches(scene.POV, name) {
			reasons = append(reasons, fmt.Sprintf("scene.%d.characters", scene.Order))
		}
	}
	for idx, event := range chapter.Events {
		if eventMatchesEntity(event, "character", name) {
			reasons = append(reasons, fmt.Sprintf("events[%d]", idx))
		}
	}
	return utils.CompactStrings(reasons)
}

func itemChapterReasons(chapter *models.Chapter, name string) []string {
	var reasons []string
	if anyTextMatches(chapter.StateAnchor.KeyItems, name) {
		reasons = append(reasons, "state_anchor.key_items")
	}
	for idx, entry := range chapter.ResourceLedger {
		if textMatches(entry.Item, name) {
			reasons = append(reasons, fmt.Sprintf("resource_ledger[%d]", idx))
		}
	}
	for idx, event := range chapter.Events {
		if eventMatchesEntity(event, "item", name) {
			reasons = append(reasons, fmt.Sprintf("events[%d]", idx))
		}
	}
	if objectJSONContains(chapter.Scenes, name) {
		reasons = append(reasons, "scenes.text")
	}
	return utils.CompactStrings(reasons)
}

func locationChapterReasons(chapter *models.Chapter, name string) []string {
	var reasons []string
	if textMatches(chapter.Location, name) {
		reasons = append(reasons, "chapter.location")
	}
	if textMatches(chapter.StateAnchor.Location, name) {
		reasons = append(reasons, "state_anchor.location")
	}
	for _, scene := range chapter.Scenes {
		if textMatches(scene.Location, name) {
			reasons = append(reasons, fmt.Sprintf("scene.%d.location", scene.Order))
		}
	}
	for idx, event := range chapter.Events {
		if eventMatchesEntity(event, "location", name) {
			reasons = append(reasons, fmt.Sprintf("events[%d]", idx))
		}
	}
	return utils.CompactStrings(reasons)
}

// storylineChapterReasons reports where a chapter advances a named storyline.
func storylineChapterReasons(chapter *models.Chapter, name string) []string {
	var reasons []string
	for idx, advance := range chapter.StorylineAdvances {
		if textMatches(advance.StorylineName, name) {
			reasons = append(reasons, fmt.Sprintf("storyline_advances[%d]", idx))
		}
	}
	if len(reasons) > 0 {
		return utils.CompactStrings(reasons)
	}
	// Fall back to free-text matching against the advance payload fields,
	// so a query with no exact storyline name can still surface mentions.
	for idx, advance := range chapter.StorylineAdvances {
		payload := advance.StorylineName + " " + advance.Stage + " " + advance.Change + " " + advance.Consequence + " " + advance.Pressure
		if textMatches(payload, name) {
			reasons = append(reasons, fmt.Sprintf("storyline_advances[%d].text", idx))
		}
	}
	return utils.CompactStrings(reasons)
}

func eventReasons(event models.Event, entityType, name string) []string {
	entityType = normalizeEntityType(entityType)
	if entityType == "" {
		if name == "" || objectJSONContains(event, name) {
			return []string{"event.text"}
		}
		return nil
	}
	if eventMatchesEntity(event, entityType, name) {
		return []string{"event.entity"}
	}
	return nil
}

func eventMatchesEntity(event models.Event, entityType, name string) bool {
	entityType = normalizeEntityType(entityType)
	if name == "" {
		if entityType == "" {
			return true
		}
		return event.GetTargetType() == entityType || legacyEventTypeMatches(event.Type, entityType)
	}
	if entityType == "character" {
		if textMatches(event.GetActor(), name) || anyTextMatches(event.Characters, name) {
			return true
		}
	}
	if event.GetTargetType() == entityType || legacyEventTypeMatches(event.Type, entityType) {
		return textMatches(event.GetTarget(), name) || objectJSONContains(event, name)
	}
	return false
}

func legacyEventTypeMatches(eventType, entityType string) bool {
	switch entityType {
	case "character":
		return eventType == models.EventTypeRelationship || eventType == models.EventTypeGoal || eventType == models.EventTypeStatus
	case "item":
		return eventType == models.EventTypeItem
	case "location":
		return false
	case "storyline":
		return eventType == models.EventTypeStoryline
	default:
		return false
	}
}

func LoadToolChapterContent(root, chapterID string) (string, string) {
	candidates := []string{
		filepath.Join(root, "chapters", "chapter-"+chapterID+".md"),
		filepath.Join(root, "chapters", chapterID+".md"),
		filepath.Join(root, "drafts", chapterID+".md"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, string(data)
		}
	}
	return "", ""
}

func namedObjectMatches[T any](key string, value *T, name string) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	return textMatches(key, name) || objectNameMatches(value, name)
}

func objectNameMatches(value interface{}, name string) bool {
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	if textMatches(fmt.Sprint(raw["name"]), name) {
		return true
	}
	if aliases, ok := raw["aliases"].([]interface{}); ok {
		for _, alias := range aliases {
			if textMatches(fmt.Sprint(alias), name) {
				return true
			}
		}
	}
	return false
}

func objectJSONContains(value interface{}, needle string) bool {
	if strings.TrimSpace(needle) == "" {
		return true
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return textMatches(string(data), needle)
}

func anyTextMatches(values []string, needle string) bool {
	if strings.TrimSpace(needle) == "" {
		return len(values) > 0
	}
	for _, value := range values {
		if textMatches(value, needle) {
			return true
		}
	}
	return false
}

func textMatches(value, needle string) bool {
	needle = normalizeSearch(needle)
	if needle == "" {
		return strings.TrimSpace(value) != ""
	}
	return strings.Contains(normalizeSearch(value), needle)
}

func idMatches(value, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(value), id)
}

func normalizeSearch(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func RepairToolLookupText(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	repaired := utils.RepairLikelyMojibakeText(trimmed)
	return repaired, repaired != trimmed
}

func NormalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeContextType(value string) string {
	switch NormalizeKey(value) {
	case "", "craft-character", "character-craft", "character", "char":
		return "craft-character"
	case "craft-item", "item-craft", "item":
		return "craft-item"
	case "craft-location", "location-craft", "location", "loc":
		return "craft-location"
	case "craft-organization", "organization-craft", "organization", "org":
		return "craft-organization"
	case "outline-volume", "volume-outline", "volume", "compose-volume":
		return "outline-volume"
	case "outline-repair", "repair-outline", "repair", "issue", "issue-repair":
		return "outline-repair"
	case "outline-global-repair", "global-outline-repair", "outline-global", "global-repair":
		return "outline-global-repair"
	case "recap-repair", "repair-recap", "recap", "recap-context":
		return "recap-repair"
	case "chapter-repair", "repair-chapter", "final-chapter-repair", "chapter-issue", "chapter-check-repair":
		return "chapter-repair"
	case "chapter-write", "write-chapter", "chapter-draft", "draft-chapter", "write", "chapter-context":
		return "chapter-write"
	default:
		return NormalizeKey(value)
	}
}

func normalizeEntityType(value string) string {
	switch NormalizeKey(value) {
	case "characters", "char":
		return "character"
	case "items":
		return "item"
	case "locations", "loc":
		return "location"
	case "storylines":
		return "storyline"
	default:
		return NormalizeKey(value)
	}
}

func limitOutlineHitResults(results interface{}, max int) []OutlineHit {
	values, _ := results.([]OutlineHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func limitChapterHitResults(results interface{}, max int) []chapterHit {
	values, _ := results.([]chapterHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func limitEventHitResults(results interface{}, max int) []EventHit {
	values, _ := results.([]EventHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func normalizeSetupType(value string) string {
	switch NormalizeKey(value) {
	case "storylines", "arc", "arcs":
		return "storyline"
	case "premises":
		return "premise"
	case "cast", "core_cast", "corecast":
		return "core-cast"
	case "resources", "world-resource", "world-resources":
		return "resource"
	case "world-timeline":
		return "timeline"
	case "rule", "rules", "setup-rules", "iron-rules":
		return "rules"
	case "longform", "long_form_plan", "long-form":
		return "long-form-plan"
	default:
		return NormalizeKey(value)
	}
}

func normalizeOutlineType(value string) string {
	switch NormalizeKey(value) {
	case "parts":
		return "part"
	case "volumes":
		return "volume"
	case "chapters", "outline-chapter", "outline-chapters":
		return "chapter"
	case "reference", "references":
		return "refs"
	default:
		return NormalizeKey(value)
	}
}

func normalizeCraftType(value string) string {
	switch NormalizeKey(value) {
	case "characters", "char":
		return "character"
	case "items":
		return "item"
	case "locations", "loc":
		return "location"
	case "organizations", "org":
		return "organization"
	default:
		return NormalizeKey(value)
	}
}

func normalizeChapterType(value string) string {
	switch NormalizeKey(value) {
	case "reference", "references":
		return "refs"
	case "event":
		return "events"
	case "outline-chapter":
		return "outline"
	default:
		return NormalizeKey(value)
	}
}

func prefixedReasons(prefix string, reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, prefix+"."+reason)
	}
	return out
}

func WriteToolJSON(cmd *cobra.Command, resp ToolResponse) error {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func init() {
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Type, "type", "", "Section-specific query type")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.ID, "id", "", "ID for part, volume, or chapter queries")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Name, "name", "", "Name or substring to search")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.EntityType, "entity-type", "", "Entity type for ref queries: character, item, location, storyline")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.ChapterID, "chapter-id", "", "Limit event queries to a chapter ID")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.VolumeID, "volume-id", "", "Limit event queries to a volume ID")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.View, "view", "brief", "Output detail level: brief, index, or full")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Fields, "fields", "", "Comma-separated JSON fields to return from each result, e.g. storyline_advances,chapter_payoff,conflict")
	ToolQueryCmd.Flags().BoolVar(&ToolQueryFlags.IncludeContent, "content", false, "Include chapter text content when available")
	ToolQueryCmd.Flags().IntVar(&ToolQueryFlags.Limit, "limit", 0, "Maximum number of results to return")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Target, "target", "outline", "Check target: outline, setup, craft, chapter, or recap")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Scope, "scope", "all", "Check scope: outline all/volume/chapter, craft character/item/location/organization, chapter chapter, or recap chapter")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.ID, "id", "", "Volume/chapter ID, craft element name, final chapter ID, or recap chapter ID for scoped checks")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.MinPriority, "min-priority", "", "Only return issues at or above priority: low, medium, high, critical")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Category, "category", "", "Only return issues in comma-separated categories, e.g. logic,plot,structure")
	ToolCheckCmd.Flags().IntVar(&ToolCheckFlags.MaxIssues, "max-issues", 0, "Maximum number of issues to return without changing the summary")
	ToolCheckCmd.Flags().IntVar(&ToolCheckFlags.TargetWords, "target-words", 0, "Override target narrative units for final chapter length checks")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.Target, "target", "chapter", "Patch target: outline chapter/volume or craft character/item/location/organization")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.ID, "id", "", "Target outline ID, final chapter ID, recap chapter ID, or craft element name/key")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.PatchJSON, "patch-json", "", "Patch JSON string; stdin is used when omitted")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.PatchBuffer, "patch-buffer", "", "Read patch content from a novelgen-managed agent patch buffer")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.Task, "task", "", "Use a validated project patch task instead of stdin or --patch-json")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.Apply, "apply", false, "Apply the patch and save project files")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.DryRun, "dry-run", false, "Force dry-run mode even when --apply is present")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.RefreshDerived, "refresh-derived", false, "For chapter --apply, regenerate derived RPG DSL and return a post-refresh all check")
	ToolPatchCmd.Flags().IntVar(&ToolPatchFlags.TargetWords, "target-words", 0, "Override target narrative units for chapter patch length checks")
	ToolPatchBufferCmd.Flags().StringVar(&ToolPatchBufferFlags.ID, "id", "", "Patch buffer id")
	ToolPatchBufferCmd.Flags().StringVar(&ToolPatchBufferFlags.Text, "text", "", "Text chunk to append; stdin is used when omitted")
	ToolPatchBufferCmd.Flags().BoolVar(&ToolPatchBufferFlags.Stdin, "stdin", false, "Read text chunk from stdin explicitly")
	ToolPatchBufferCmd.Flags().IntVar(&ToolPatchBufferFlags.MaxChars, "max-chars", 400, "Preview character limit for show")
	ToolRefreshCmd.Flags().StringVar(&ToolRefreshFlags.ID, "id", "", "Target chapter ID for scoped refresh")
	ToolRefreshCmd.Flags().IntVar(&ToolRefreshFlags.BatchSize, "batch-size", 10, "Chapter markdown batch size for AI -> RPG DSL conversion")
	ToolCmd.AddCommand(ToolQueryCmd)
	ToolCmd.AddCommand(ToolCheckCmd)
	ToolCmd.AddCommand(ToolPatchCmd)
	ToolCmd.AddCommand(ToolPatchBufferCmd)
	ToolCmd.AddCommand(ToolRefreshCmd)
}

// NewCommand returns the novelgen tool command tree.
func NewCommand() *cobra.Command {
	return ToolCmd
}
