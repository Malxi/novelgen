package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/utils"

	"github.com/spf13/cobra"
)

var toolQueryFlags struct {
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

var toolProcessStartedAt = time.Now()

type toolResponse struct {
	OK       bool                   `json:"ok"`
	Section  string                 `json:"section"`
	Query    map[string]string      `json:"query,omitempty"`
	Count    int                    `json:"count"`
	Results  interface{}            `json:"results,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
}

type outlineHit struct {
	Type    string      `json:"type"`
	ID      string      `json:"id"`
	Title   string      `json:"title,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Path    outlinePath `json:"path"`
	Object  interface{} `json:"object,omitempty"`
	Reasons []string    `json:"reasons,omitempty"`
}

type outlinePath struct {
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
	Path        outlinePath     `json:"path"`
	Outline     *models.Chapter `json:"outline,omitempty"`
	ContentPath string          `json:"content_path,omitempty"`
	Content     string          `json:"content,omitempty"`
	Reasons     []string        `json:"reasons,omitempty"`
}

type eventHit struct {
	ChapterID    string       `json:"chapter_id,omitempty"`
	ChapterTitle string       `json:"chapter_title,omitempty"`
	EventIndex   int          `json:"event_index"`
	Path         outlinePath  `json:"path"`
	Event        models.Event `json:"event"`
	Reasons      []string     `json:"reasons,omitempty"`
}

type eventHitBrief struct {
	ChapterID    string       `json:"chapter_id,omitempty"`
	ChapterTitle string       `json:"chapter_title,omitempty"`
	EventIndex   int          `json:"event_index"`
	Path         *outlinePath `json:"path,omitempty"`
	Event        eventBrief   `json:"event"`
	Reasons      []string     `json:"reasons,omitempty"`
}

type logHit struct {
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

type craftElementContext struct {
	Target           string                 `json:"target"`
	Name             string                 `json:"name"`
	CharacterName    string                 `json:"character_name,omitempty"`
	StorySetup       interface{}            `json:"story_setup,omitempty"`
	ExistingCraft    interface{}            `json:"existing_craft,omitempty"`
	OutlineRefs      interface{}            `json:"outline_refs,omitempty"`
	RelevantChapters interface{}            `json:"relevant_chapters,omitempty"`
	Events           interface{}            `json:"events,omitempty"`
	Navigation       map[string]interface{} `json:"navigation,omitempty"`
	NextActions      []toolNextAction       `json:"next_actions,omitempty"`
	Stats            map[string]int         `json:"stats,omitempty"`
}

type outlineVolumeContext struct {
	VolumeID       string                 `json:"volume_id"`
	Path           outlinePath            `json:"path"`
	StorySetup     interface{}            `json:"story_setup,omitempty"`
	TargetVolume   interface{}            `json:"target_volume,omitempty"`
	PreviousVolume interface{}            `json:"previous_volume,omitempty"`
	NextVolume     interface{}            `json:"next_volume,omitempty"`
	EntityIndex    outlineVolumeEntities  `json:"entity_index,omitempty"`
	Events         interface{}            `json:"events,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	NextActions    []toolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type outlineRepairContext struct {
	Scope           string                 `json:"scope"`
	ID              string                 `json:"id"`
	Path            outlinePath            `json:"path"`
	IssueCategory   string                 `json:"issue_category,omitempty"`
	Check           *toolCheckResult       `json:"check,omitempty"`
	IssueContext    []outlineRepairIssue   `json:"issue_context,omitempty"`
	Current         interface{}            `json:"current,omitempty"`
	ParentVolume    interface{}            `json:"parent_volume,omitempty"`
	PreviousChapter interface{}            `json:"previous_chapter,omitempty"`
	NextChapter     interface{}            `json:"next_chapter,omitempty"`
	Events          interface{}            `json:"events,omitempty"`
	Navigation      map[string]interface{} `json:"navigation,omitempty"`
	Workflow        map[string]interface{} `json:"workflow,omitempty"`
	NextActions     []toolNextAction       `json:"next_actions,omitempty"`
	Stats           map[string]int         `json:"stats,omitempty"`
}

type outlineGlobalRepairContext struct {
	IssueCategory  string                 `json:"issue_category,omitempty"`
	Check          *toolCheckResult       `json:"check,omitempty"`
	IssueContext   []outlineRepairIssue   `json:"issue_context,omitempty"`
	PatchTask      *outlinePatchTask      `json:"patch_task,omitempty"`
	MysteryThreads []outlineMysteryThread `json:"mystery_threads,omitempty"`
	StorySetup     interface{}            `json:"story_setup,omitempty"`
	Outline        interface{}            `json:"outline,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	Workflow       map[string]interface{} `json:"workflow,omitempty"`
	NextActions    []toolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type recapRepairContext struct {
	ChapterID      string                 `json:"chapter_id"`
	Path           outlinePath            `json:"path"`
	Check          *toolCheckResult       `json:"check,omitempty"`
	Current        interface{}            `json:"current,omitempty"`
	Outline        interface{}            `json:"outline,omitempty"`
	ChapterExcerpt map[string]string      `json:"chapter_excerpt,omitempty"`
	Navigation     map[string]interface{} `json:"navigation,omitempty"`
	Workflow       map[string]interface{} `json:"workflow,omitempty"`
	NextActions    []toolNextAction       `json:"next_actions,omitempty"`
	Stats          map[string]int         `json:"stats,omitempty"`
}

type chapterWriteContext struct {
	ChapterID              string                 `json:"chapter_id"`
	Path                   outlinePath            `json:"path"`
	StorySetup             interface{}            `json:"story_setup,omitempty"`
	ParentVolume           interface{}            `json:"parent_volume,omitempty"`
	TargetChapter          interface{}            `json:"target_chapter,omitempty"`
	PreviousChapter        interface{}            `json:"previous_chapter,omitempty"`
	NextChapter            interface{}            `json:"next_chapter,omitempty"`
	PreviousRecap          interface{}            `json:"previous_recap,omitempty"`
	CurrentRecap           interface{}            `json:"current_recap,omitempty"`
	ExistingChapterExcerpt map[string]string      `json:"existing_chapter_excerpt,omitempty"`
	CreativeHistory        *chapterWriteHistory   `json:"creative_history,omitempty"`
	EntityIndex            outlineVolumeEntities  `json:"entity_index,omitempty"`
	Events                 interface{}            `json:"events,omitempty"`
	Navigation             map[string]interface{} `json:"navigation,omitempty"`
	Workflow               map[string]interface{} `json:"workflow,omitempty"`
	NextActions            []toolNextAction       `json:"next_actions,omitempty"`
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

type chapterRepairContext struct {
	ChapterID              string                 `json:"chapter_id"`
	Path                   outlinePath            `json:"path"`
	IssueCategory          string                 `json:"issue_category,omitempty"`
	Check                  *toolCheckResult       `json:"check,omitempty"`
	SimulationRepairHints  []simulationRepairHint `json:"simulation_repair_hints,omitempty"`
	TargetChapter          interface{}            `json:"target_chapter,omitempty"`
	ParentVolume           interface{}            `json:"parent_volume,omitempty"`
	PreviousChapter        interface{}            `json:"previous_chapter,omitempty"`
	NextChapter            interface{}            `json:"next_chapter,omitempty"`
	ExistingChapterExcerpt map[string]string      `json:"existing_chapter_excerpt,omitempty"`
	EntityIndex            outlineVolumeEntities  `json:"entity_index,omitempty"`
	Events                 interface{}            `json:"events,omitempty"`
	Navigation             map[string]interface{} `json:"navigation,omitempty"`
	Workflow               map[string]interface{} `json:"workflow,omitempty"`
	NextActions            []toolNextAction       `json:"next_actions,omitempty"`
	Stats                  map[string]int         `json:"stats,omitempty"`
}

type toolNextAction struct {
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

type outlineRepairIssue struct {
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

type outlineVolumeEntities struct {
	Characters []string `json:"characters,omitempty"`
	Items      []string `json:"items,omitempty"`
	Locations  []string `json:"locations,omitempty"`
	Storylines []string `json:"storylines,omitempty"`
}

type storySetupBrief struct {
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

type storySetupSearchResult struct {
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

type outlineBrief struct {
	Parts      []partBrief            `json:"parts,omitempty"`
	Navigation map[string]interface{} `json:"navigation,omitempty"`
}

type partBrief struct {
	ID      string        `json:"id,omitempty"`
	Title   string        `json:"title,omitempty"`
	Summary string        `json:"summary,omitempty"`
	Volumes []volumeBrief `json:"volumes,omitempty"`
}

type volumeBrief struct {
	ID             string                       `json:"id,omitempty"`
	Title          string                       `json:"title,omitempty"`
	Summary        string                       `json:"summary,omitempty"`
	PayoffContract *models.VolumePayoffContract `json:"payoff_contract,omitempty"`
	Chapters       []chapterBrief               `json:"chapters,omitempty"`
	Navigation     map[string]interface{}       `json:"navigation,omitempty"`
}

type chapterBrief struct {
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
	Events            []eventBrief              `json:"events,omitempty"`
	StorylineAdvances []models.StorylineAdvance `json:"storyline_advances,omitempty"`
	ChapterPayoff     *models.ChapterPayoff     `json:"chapter_payoff,omitempty"`
	EventCount        int                       `json:"event_count,omitempty"`
	AdvanceCount      int                       `json:"advance_count,omitempty"`
}

type eventBrief struct {
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

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Project tools for agents",
	Long: `Project tools for agents.

	Query and check commands return JSON and never mutate project state. Patch
commands can mutate project state only when explicitly called with --apply.`,
}

var toolQueryCmd = &cobra.Command{
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

var toolPatchBufferCmd = &cobra.Command{
	Use:   "patch-buffer [append|show|clear]",
	Short: "Manage a novelgen-owned temporary patch buffer",
	Long: `Manage a novelgen-owned temporary patch buffer.

This command exists for agent workflows that need to pass long chapter text to
tool patch without shell temp files or very long --patch-json arguments. It
stores text under .novelgen/agent-patches and does not mutate story state.`,
	Args: cobra.ExactArgs(1),
	RunE: runToolPatchBuffer,
}

var toolRefreshCmd = &cobra.Command{
	Use:   "refresh [chapter-dsl]",
	Short: "Refresh derived project artifacts for agent workflows",
	Long: `Refresh derived project artifacts for agent workflows.

Refresh commands are explicit mutating tools. They do not edit story source
state such as setup, outline, craft, recaps, or chapter markdown; they rebuild
derived files that downstream checks consume.`,
	Args: cobra.ExactArgs(1),
	RunE: runToolRefresh,
}

var allowedToolPatchBufferOps = map[string]bool{
	"append": true,
	"show":   true,
	"clear":  true,
}

func runToolQuery(cmd *cobra.Command, args []string) error {
	section := ""
	if len(args) > 0 {
		section = normalizeKey(args[0])
	}
	if section == "" {
		return fmt.Errorf("query section is required: story-setup, outline, craft, chapter, or context")
	}
	queryType := normalizeKey(toolQueryFlags.Type)
	queryID, idRepaired := repairToolLookupText(toolQueryFlags.ID)
	queryName, nameRepaired := repairToolLookupText(toolQueryFlags.Name)
	queryChapterID, chapterIDRepaired := repairToolLookupText(toolQueryFlags.ChapterID)
	queryVolumeID, volumeIDRepaired := repairToolLookupText(toolQueryFlags.VolumeID)
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	ctx := toolProjectContext{Root: root}
	if err := ctx.load(section); err != nil {
		return err
	}

	var resp toolResponse
	switch section {
	case "story-setup", "setup":
		resp = queryStorySetup(ctx, queryType, queryName)
	case "outline":
		resp = queryOutline(ctx, queryType, queryID, toolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID)
	case "craft":
		resp = queryCraft(ctx, queryType, queryName)
	case "chapter", "chapters":
		resp = queryChapterSection(ctx, queryType, queryID, toolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID, toolQueryFlags.IncludeContent)
	case "context":
		resp = queryContext(ctx, queryType, queryID, queryName)
	case "logs", "log":
		resp = queryLogs(ctx.Root, queryType, queryID, queryName, toolQueryFlags.IncludeContent)
	default:
		return fmt.Errorf("unsupported query section %q", section)
	}
	if idRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --id from %q to %q", toolQueryFlags.ID, queryID))
	}
	if nameRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --name from %q to %q", toolQueryFlags.Name, queryName))
	}
	if chapterIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --chapter-id from %q to %q", toolQueryFlags.ChapterID, queryChapterID))
	}
	if volumeIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --volume-id from %q to %q", toolQueryFlags.VolumeID, queryVolumeID))
	}

	resp.Section = section
	resp.Query = map[string]string{
		"section":     section,
		"type":        queryType,
		"id":          queryID,
		"name":        queryName,
		"entity_type": toolQueryFlags.EntityType,
		"chapter_id":  queryChapterID,
		"volume_id":   queryVolumeID,
		"fields":      toolQueryFlags.Fields,
	}
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["project_root"] = root
	applyToolView(&resp, toolQueryFlags.View)
	applyToolFields(&resp, toolQueryFlags.Fields)
	applyToolLimit(&resp, toolQueryFlags.Limit)
	return writeToolJSON(cmd, resp)
}

type toolProjectContext struct {
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

func (c *toolProjectContext) load(kind string) error {
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

func queryStorySetup(ctx toolProjectContext, queryType, name string) toolResponse {
	if ctx.Setup == nil {
		return toolResponse{OK: true, Count: 0, Results: nil}
	}
	switch normalizeSetupType(queryType) {
	case "", "all":
		return toolResponse{OK: true, Count: 1, Results: ctx.Setup}
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
	case "long-form-plan":
		count := 0
		if ctx.Setup.LongFormPlan != nil {
			count = 1
		}
		return toolResponse{OK: true, Count: count, Results: ctx.Setup.LongFormPlan}
	default:
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported story-setup type %q", queryType)}}
	}
}

func queryLogs(root, queryType, id, name string, includeContent bool) toolResponse {
	logsRoot := filepath.Join(root, "logs")
	if _, err := os.Stat(logsRoot); err != nil {
		return toolResponse{OK: true, Section: "logs", Count: 0, Results: nil, Warnings: []string{"logs directory not found"}}
	}
	kindFilter := normalizeLogKind(queryType)
	id = filepath.ToSlash(strings.TrimSpace(id))
	name = strings.ToLower(strings.TrimSpace(name))
	historyCutoff, hasHistoryCutoff := logHistoryCutoff()

	hits := []logHit{}
	err := filepath.WalkDir(logsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(logsRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		kind := logKindFromRel(rel)
		if kindFilter != "" && kind != kindFilter {
			return nil
		}
		if id == "" && kindFilter == "" && kind != "prompts" && kind != "responses" && kind != "agent-live" {
			return nil
		}
		if id == "" && hasHistoryCutoff && info.ModTime().After(historyCutoff) {
			return nil
		}
		if id != "" && !strings.EqualFold(rel, id) {
			return nil
		}
		agent := logAgentFromFilename(filepath.Base(path))
		searchHaystack := strings.ToLower(rel + " " + agent)
		if name != "" && !strings.Contains(searchHaystack, name) {
			return nil
		}
		summary := map[string]interface{}(nil)
		if kind == "agent-live" {
			summary = summarizeAgentLiveLogFile(path)
			if id == "" && !agentLiveLogSummaryComplete(summary) {
				return nil
			}
		}
		hit := logHit{
			ID:          rel,
			Kind:        kind,
			Agent:       agent,
			Path:        rel,
			SizeBytes:   info.Size(),
			ModifiedAt:  info.ModTime(),
			DetailQuery: fmt.Sprintf("novelgen tool query logs --id %q --view brief", rel),
		}
		if kind == "agent-live" {
			hit.Summary = summary
		}
		if includeContent && id != "" {
			hit.Content, hit.Truncated = readLogExcerptForQuery(path, kind, 12000)
		} else {
			hit.Preview, hit.Truncated = readLogPreviewForQuery(path, kind, 1000)
		}
		hits = append(hits, hit)
		return nil
	})
	if err != nil {
		return toolResponse{OK: false, Section: "logs", Count: 0, Warnings: []string{err.Error()}}
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].ModifiedAt.After(hits[j].ModifiedAt)
	})
	meta := map[string]interface{}{
		"content_policy": "content is returned only when --content is used with an exact --id; excerpts are capped",
		"next_queries": []string{
			"novelgen tool query logs --view index",
			"novelgen tool query logs --type agent-live --view index",
			"novelgen tool query logs --type prompts --name <agent> --view brief --limit 5",
			"novelgen tool query logs --id <relative_log_path> --content --view brief",
		},
	}
	return toolResponse{OK: true, Section: "logs", Count: len(hits), Results: hits, Meta: meta}
}

func logHistoryCutoff() (time.Time, bool) {
	raw := strings.TrimSpace(os.Getenv("NOVELGEN_LOG_HISTORY_CUTOFF"))
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func agentLiveLogSummaryComplete(summary map[string]interface{}) bool {
	return summaryInt(summary, "final_records") > 0
}

func summaryInt(summary map[string]interface{}, key string) int {
	if summary == nil {
		return 0
	}
	switch value := summary[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func normalizeLogKind(value string) string {
	switch normalizeKey(value) {
	case "", "all":
		return ""
	case "prompt", "prompts":
		return "prompts"
	case "response", "responses":
		return "responses"
	case "agent-live", "agent_live", "live", "livelog", "live-log":
		return "agent-live"
	default:
		return normalizeKey(value)
	}
}

func logKindFromRel(rel string) string {
	first, _, ok := strings.Cut(filepath.ToSlash(rel), "/")
	if !ok {
		return "other"
	}
	return normalizeLogKind(first)
}

func logAgentFromFilename(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, marker := range []string{"_20", "-20"} {
		if idx := strings.Index(base, marker); idx > 0 {
			return base[:idx]
		}
	}
	if idx := strings.Index(base, "_"); idx > 0 {
		return base[:idx]
	}
	return base
}

func readLogPreview(path string, maxRunes int) (string, bool) {
	content, truncated := readLogExcerpt(path, maxRunes)
	return logPreviewFromExcerpt(content, truncated)
}

func readLogPreviewForQuery(path string, kind string, maxRunes int) (string, bool) {
	content, truncated := readLogExcerptForQuery(path, kind, maxRunes)
	return logPreviewFromExcerpt(content, truncated)
}

func logPreviewFromExcerpt(content string, truncated bool) (string, bool) {
	if content == "" {
		return "", truncated
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	kept := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isLogPreviewMetadataLine(line) {
			continue
		}
		kept = append(kept, line)
		if len(kept) == 3 {
			break
		}
	}
	return strings.Join(kept, "\n"), truncated
}

func isLogPreviewMetadataLine(line string) bool {
	normalized := strings.ToLower(strings.TrimSpace(line))
	if normalized == "" {
		return true
	}
	if normalized == "---" || normalized == "{" || normalized == "[" {
		return true
	}
	if strings.HasPrefix(normalized, "```") {
		return true
	}
	for _, prefix := range []string{
		"# agent:",
		"# time:",
		"# ai response",
		"# system prompt",
		"# user prompt",
		"# response",
		"# prompt",
	} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func readLogExcerptForQuery(path string, kind string, maxRunes int) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text := string(utils.StripUTF8BOM(data))
	if normalizeLogKind(kind) == "agent-live" {
		text = sanitizeAgentLiveLogText(text)
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text, false
	}
	return string(runes[:maxRunes]), true
}

func readLogExcerpt(path string, maxRunes int) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text := string(utils.StripUTF8BOM(data))
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text, false
	}
	return string(runes[:maxRunes]), true
}

func sanitizeAgentLiveLogText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = sanitizeAgentLiveLogLine(line)
	}
	return strings.Join(lines, "\n")
}

func summarizeAgentLiveLogFile(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return summarizeAgentLiveLogText(string(utils.StripUTF8BOM(data)))
}

func summarizeAgentLiveLogText(text string) map[string]interface{} {
	summary := map[string]interface{}{}
	allowedCommands := []string{}
	deniedCommands := []string{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		incrementSummaryInt(summary, "events")
		event := strings.TrimSpace(fmt.Sprint(record["event"]))
		switch event {
		case "start":
			if model := liveLogRecordString(record, "model"); model != "" {
				summary["model"] = model
			}
			addSummaryStringSlice(summary, "sdk_skills", liveLogRecordStringSlice(record, "sdk_skills"))
			addSummaryStringSlice(summary, "loaded_sdk_skills", liveLogRecordStringSlice(record, "loaded_sdk_skills"))
			addSummaryStringSlice(summary, "missing_sdk_skills", liveLogRecordStringSlice(record, "missing_sdk_skills"))
		case "message":
			incrementSummaryInt(summary, "messages")
		case "final":
			incrementSummaryInt(summary, "final_records")
			if model := liveLogRecordString(record, "model"); model != "" {
				summary["final_model"] = model
			}
		case "tool_hook", "tool_permission":
			hook := strings.TrimSpace(fmt.Sprint(record["hook"]))
			if hook != "PreToolUse" && event != "tool_permission" {
				continue
			}
			command := summarizeAgentLiveToolCommand(liveLogRecordString(record, "command"))
			if command == "" {
				continue
			}
			if allowed, ok := record["allowed"].(bool); ok {
				incrementSummaryInt(summary, "tool_calls")
				if allowed {
					incrementSummaryInt(summary, "tool_allowed")
					countAgentLiveSummaryCommand(summary, command)
					appendUniqueSummaryCommand(&allowedCommands, command, 12)
				} else {
					incrementSummaryInt(summary, "tool_denied")
					appendUniqueSummaryCommand(&deniedCommands, command, 8)
				}
			}
		}
	}
	if _, ok := summary["final_model"]; !ok {
		if model, ok := summary["model"]; ok {
			summary["final_model"] = model
		}
	}
	if len(allowedCommands) > 0 {
		summary["allowed_tool_commands"] = allowedCommands
	}
	if len(deniedCommands) > 0 {
		summary["denied_tool_commands"] = deniedCommands
	}
	if len(summary) == 0 {
		return nil
	}
	return summary
}

func incrementSummaryInt(summary map[string]interface{}, key string) {
	current, _ := summary[key].(int)
	summary[key] = current + 1
}

func addSummaryStringSlice(summary map[string]interface{}, key string, values []string) {
	if len(values) == 0 {
		return
	}
	summary[key] = values
}

func liveLogRecordString(record map[string]interface{}, key string) string {
	if record == nil {
		return ""
	}
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func liveLogRecordStringSlice(record map[string]interface{}, key string) []string {
	value, ok := record[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func countAgentLiveSummaryCommand(summary map[string]interface{}, command string) {
	normalized := strings.ToLower(command)
	switch {
	case strings.Contains(normalized, " tool query "):
		incrementSummaryInt(summary, "query_calls")
	case strings.Contains(normalized, " tool check "):
		incrementSummaryInt(summary, "check_calls")
	case strings.Contains(normalized, " tool refresh "):
		incrementSummaryInt(summary, "refresh_calls")
	case strings.Contains(normalized, " tool patch "), strings.Contains(normalized, " tool patch-buffer "):
		incrementSummaryInt(summary, "patch_calls")
		if strings.Contains(normalized, " --apply") {
			incrementSummaryInt(summary, "patch_applies")
		}
	}
}

func appendUniqueSummaryCommand(commands *[]string, command string, max int) {
	if command == "" || len(*commands) >= max {
		return
	}
	for _, existing := range *commands {
		if existing == command {
			return
		}
	}
	*commands = append(*commands, command)
}

func sanitizeAgentLiveLogLine(line string) string {
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(line), &record); err == nil {
		if command, ok := record["command"].(string); ok {
			record["command"] = summarizeAgentLiveToolCommand(command)
			if encoded, err := json.Marshal(record); err == nil {
				return string(encoded)
			}
		}
		return line
	}
	return sanitizeAgentLiveRawCommandText(line)
}

func sanitizeAgentLiveRawCommandText(line string) string {
	lower := strings.ToLower(line)
	idx := novelgenToolCommandIndexForQuery(lower)
	if idx < 0 {
		return line
	}
	command := summarizeAgentLiveToolCommand(line[idx:])
	if command == "" {
		return "<command>"
	}
	return command
}

func summarizeAgentLiveToolCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	lower := strings.ToLower(command)
	if isClaudeTempOutputReadCommand(lower) {
		return "powershell Get-Content <claude-temp-tool-output>"
	}
	if idx := novelgenToolCommandIndexForQuery(lower); idx >= 0 {
		command = strings.TrimSpace(command[idx:])
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--patch-json"); idx >= 0 {
		suffix := ""
		if strings.Contains(lower[idx:], " --apply") {
			suffix = " --apply"
		}
		command = strings.TrimSpace(command[:idx]) + " --patch-json <json>" + suffix
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--text"); idx >= 0 && strings.Contains(lower, " tool patch-buffer ") {
		command = strings.TrimSpace(command[:idx]) + " --text <text>"
		lower = strings.ToLower(command)
	}
	if idx := strings.Index(lower, "--stdin"); idx >= 0 && strings.Contains(lower, " tool patch-buffer ") {
		command = strings.TrimSpace(command[:idx]) + " --stdin <stdin>"
	}
	return clipToolString(command, 240)
}

func isClaudeTempOutputReadCommand(lowerCommand string) bool {
	return strings.Contains(lowerCommand, "get-content") &&
		strings.Contains(lowerCommand, `\temp\claude\`) &&
		strings.Contains(lowerCommand, `\tasks\`) &&
		strings.Contains(lowerCommand, ".output")
}

func novelgenToolCommandIndexForQuery(lowerCommand string) int {
	best := -1
	for _, marker := range []string{
		"novelgen tool",
		"novelgen.exe tool",
		"novelgen.exe' tool",
		"novelgen.exe\" tool",
	} {
		if idx := strings.Index(lowerCommand, marker); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func queryStorySetupSearch(setup *models.StorySetup, name string) toolResponse {
	name = strings.TrimSpace(name)
	if name == "" {
		return toolResponse{OK: false, Section: "story-setup", Count: 0, Warnings: []string{"--name is required for story-setup --type search"}}
	}

	const perSectionLimit = 3
	result := storySetupSearchResult{
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
	return toolResponse{OK: true, Section: "story-setup", Count: total, Results: result, Warnings: warnings}
}

func querySetupSlice[T any](kind, name string, values []T, nameOf func(T) string) toolResponse {
	hits := make([]T, 0)
	for _, value := range values {
		if name == "" || textMatches(nameOf(value), name) || objectJSONContains(value, name) {
			hits = append(hits, value)
		}
	}
	return toolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits}
}

func queryOutline(ctx toolProjectContext, queryType, id, entityType, name, chapterID, volumeID string) toolResponse {
	switch normalizeOutlineType(queryType) {
	case "", "all":
		count := 0
		if ctx.Outline != nil {
			count = len(ctx.Outline.Parts)
		}
		return toolResponse{OK: true, Count: count, Results: ctx.Outline}
	case "part":
		return queryOutlineNode(ctx.Outline, "part", id)
	case "volume":
		return queryOutlineNode(ctx.Outline, "volume", id)
	case "chapter":
		return queryOutlineNode(ctx.Outline, "chapter", id)
	case "refs":
		return queryOutlineRefs(ctx.Outline, entityType, name, false)
	case "events":
		return queryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	default:
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported outline type %q", queryType)}}
	}
}

func queryCraft(ctx toolProjectContext, queryType, name string) toolResponse {
	switch normalizeCraftType(queryType) {
	case "", "all":
		results := map[string]interface{}{
			"characters":    rawOrTyped(ctx.RawCharacters, ctx.Characters),
			"items":         rawOrTyped(ctx.RawItems, ctx.Items),
			"locations":     rawOrTyped(ctx.RawLocations, ctx.Locations),
			"organizations": rawOrTyped(ctx.RawOrgs, ctx.Organizations),
		}
		return toolResponse{OK: true, Count: len(ctx.Characters) + len(ctx.Items) + len(ctx.Locations) + len(ctx.Organizations), Results: results}
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
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported craft type %q", queryType)}}
	}
}

func queryChapterSection(ctx toolProjectContext, queryType, id, entityType, name, chapterID, volumeID string, includeContent bool) toolResponse {
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
		return queryEvents(ctx.Outline, chapterID, volumeID, entityType, name)
	case "outline":
		return queryOutlineNode(ctx.Outline, "chapter", id)
	default:
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported chapter type %q", queryType)}}
	}
}

func queryContext(ctx toolProjectContext, queryType, id, name string) toolResponse {
	contextType := normalizeContextType(queryType)
	switch contextType {
	case "craft-character", "craft-item", "craft-location", "craft-organization":
		return queryCraftElementContext(ctx, strings.TrimPrefix(contextType, "craft-"), name)
	case "outline-volume":
		return queryOutlineVolumeContext(ctx, id, name)
	case "outline-repair":
		return queryOutlineRepairContext(ctx, id, name)
	case "outline-global-repair":
		return queryOutlineGlobalRepairContext(ctx, name)
	case "recap-repair":
		return queryRecapRepairContext(ctx, id)
	case "chapter-write":
		return queryChapterWriteContext(ctx, id)
	case "chapter-repair":
		return queryChapterRepairContext(ctx, id, name)
	default:
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("unsupported context type %q", queryType)}}
	}
}

func queryChapterWriteContext(ctx toolProjectContext, chapterID string) toolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-write"}}
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
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := queryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
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
	result := chapterWriteContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
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
		if !info.ModTime().Before(toolProcessStartedAt.Add(-1 * time.Second)) {
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

func buildChapterWriteNavigation(chapterID string, previousChapter, nextChapter *flatChapter, entities outlineVolumeEntities) map[string]interface{} {
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

func buildChapterWriteNextActions(chapterID string, previousChapter *flatChapter) []toolNextAction {
	actions := []toolNextAction{
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
		actions = append(actions, toolNextAction{
			Step:    len(actions) + 1,
			Action:  "preserve_previous_continuity",
			Purpose: "Use previous_chapter and previous_recap facts already included in this context; do not run recap checks.",
			When:    "The target opening depends on previous chapter continuity.",
		})
	}
	return actions
}

func queryChapterRepairContext(ctx toolProjectContext, chapterID, issueCategory string) toolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type chapter-repair"}}
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
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	target := chapters[targetIndex]
	eventsResp := queryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
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
	var check *toolCheckResult
	if strings.TrimSpace(ctx.Root) == "" {
		warnings = append(warnings, "project root missing; chapter quality/simulation check unavailable")
	} else {
		var err error
		check, err = runToolChapterCheck(ctx.Root, "all", "chapter", target.Chapter.ID)
		if err != nil {
			warnings = append(warnings, "chapter check unavailable: "+err.Error())
		} else if err := applyToolCheckIssueFilters(check, "low", chapterRepairIssueCategoryFilter(issueCategory), 12); err != nil {
			warnings = append(warnings, "chapter check filter ignored: "+err.Error())
		}
	}

	excerpt, excerptWarnings := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	warnings = append(warnings, excerptWarnings...)

	result := chapterRepairContext{
		ChapterID:              target.Chapter.ID,
		Path:                   target.Path,
		IssueCategory:          categoryOrAll(issueCategory),
		Check:                  check,
		SimulationRepairHints:  buildChapterSimulationRepairHints(target.Chapter.ID, check),
		TargetChapter:          makeRepairChapterBrief(*target.Chapter),
		ExistingChapterExcerpt: excerpt,
		EntityIndex:            entities,
		Events:                 briefOutlineResults(events, false),
		Navigation:             buildChapterRepairNavigation(target.Chapter.ID, issueCategory, entities),
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildChapterRepairNavigation(chapterID, issueCategory string, entities outlineVolumeEntities) map[string]interface{} {
	category := chapterRepairIssueCategoryFilter(issueCategory)
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
		"craft_context_queries":  craftQueries,
		"quality_check_query":    fmt.Sprintf("novelgen tool check quality --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 12", chapterID, category),
		"simulation_check_query": fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 12", chapterID, category),
		"all_check_query":        fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 12", chapterID, category),
		"refresh_chapter_dsl_query": fmt.Sprintf(
			"novelgen tool refresh chapter-dsl --id %q",
			chapterID,
		),
		"repair_command": fmt.Sprintf("novelgen write improve --agent-sdk --chapter %q --max-rounds 1", chapterID),
	}
}

func chapterRepairIssueCategoryFilter(issueCategory string) string {
	category := strings.TrimSpace(issueCategory)
	switch normalizeKey(category) {
	case "", "all", "simulation":
		return categoryOrAll("") + ",simulation"
	default:
		return category
	}
}

func buildChapterSimulationRepairHints(chapterID string, check *toolCheckResult) []simulationRepairHint {
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
			fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
			fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
			fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
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
			fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
			fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
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
		"post_repair_refresh_query": fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
		"post_repair_check_query":   fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --min-priority low --max-issues 12", chapterID),
	}
}

func buildChapterRepairNextActions(chapterID, issueCategory string) []toolNextAction {
	category := chapterRepairIssueCategoryFilter(issueCategory)
	return []toolNextAction{
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
			Command: fmt.Sprintf("novelgen tool refresh chapter-dsl --id %q", chapterID),
			Purpose: "Regenerate derived RPG DSL after prose changes; simulation checks do not invoke LLM conversion.",
			When:    "After chapter prose has changed and refresh is allowed by the workflow.",
		},
		{
			Step:    5,
			Action:  "post_repair_check",
			Command: fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 12", chapterID, category),
			Purpose: "Verify the targeted repair and catch remaining deterministic prose or simulation issues.",
		},
	}
}

func queryRecapRepairContext(ctx toolProjectContext, chapterID string) toolResponse {
	chapterID = strings.TrimSpace(chapterID)
	if chapterID == "" {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type recap-repair"}}
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
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}

	check, err := runToolRecapCheck(ctx.Root, "quality", "chapter", target.Chapter.ID)
	if err != nil {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = applyToolCheckIssueFilters(check, "low", "recap", 8)

	var current *models.ChapterRecap
	if ctx.Root != "" {
		if loaded, err := recap.NewStore(ctx.Root).Load(target.Chapter.ID); err == nil {
			current = loaded
		}
	}
	excerpt, contentWarnings := loadChapterExcerptForTool(ctx.Root, target.Chapter.ID)
	result := recapRepairContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: contentWarnings}
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

func buildRecapRepairNextActions(chapterID string) []toolNextAction {
	return []toolNextAction{
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

func queryOutlineRepairContext(ctx toolProjectContext, id, category string) toolResponse {
	id = strings.TrimSpace(id)
	category = normalizeKey(category)
	if id == "" {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id is required for context --type outline-repair"}}
	}
	if toolLooksLikeVolumeID(id) {
		return queryOutlineVolumeRepairContext(ctx, id, category)
	}
	if _, ok := toolVolumeIDFromChapterID(id); ok {
		return queryOutlineChapterRepairContext(ctx, id, category)
	}
	return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{fmt.Sprintf("cannot infer outline repair scope from id %q", id)}}
}

func queryOutlineGlobalRepairContext(ctx toolProjectContext, category string) toolResponse {
	category = normalizeKey(category)
	check, err := runToolOutlineCheck("all", ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = applyToolCheckIssueFilters(check, "low", category, 12)
	mysteryThreads := collectOutlineMysteryThreads(ctx.Outline)
	issueContext := buildOutlineGlobalRepairIssues(check, category, ctx.Outline)
	patchTask := buildOutlineGlobalPatchTask(issueContext)
	result := outlineGlobalRepairContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result}
}

func queryOutlineVolumeRepairContext(ctx toolProjectContext, volumeID, category string) toolResponse {
	volumes := flattenVolumes(ctx.Outline)
	targetIndex := -1
	for i, volume := range volumes {
		if idMatches(volume.Volume.ID, volumeID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: %q", volumeID)}}
	}
	target := volumes[targetIndex]
	check, err := runToolOutlineCheck("all", ctx.Setup, ctx.Outline, "volume", target.Volume.ID)
	if err != nil {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = applyToolCheckIssueFilters(check, "low", category, 8)
	events := collectRepairVolumeEvents(target, check)
	returnedEvents := events
	if len(returnedEvents) > 8 {
		returnedEvents = returnedEvents[:8]
	}
	result := outlineRepairContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func queryOutlineChapterRepairContext(ctx toolProjectContext, chapterID, category string) toolResponse {
	chapters := flattenChapters(ctx.Outline)
	targetIndex := -1
	for i, chapter := range chapters {
		if idMatches(chapter.Chapter.ID, chapterID) {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline chapter not found: %q", chapterID)}}
	}
	target := chapters[targetIndex]
	volumeID := target.Path.VolumeID
	check, err := runToolOutlineCheck("all", ctx.Setup, ctx.Outline, "chapter", target.Chapter.ID)
	if err != nil {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = applyToolCheckIssueFilters(check, "low", category, 8)
	eventResp := queryEvents(ctx.Outline, target.Chapter.ID, "", "", "")
	events := limitEventHitResults(eventResp.Results, 8)
	result := outlineRepairContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
}

func buildOutlineRepairIssues(check *toolCheckResult, scope, targetID, volumeID, category string, chapter *models.Chapter, volume *models.Volume) []outlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	issues := make([]outlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueTargetID := strings.TrimSpace(issue.TargetID)
		if issueTargetID == "" {
			issueTargetID = targetID
		}
		issueCategory := normalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		issueScope := scope
		patchShape := map[string]interface{}{"summary": "<optional new summary>"}
		if _, ok := toolVolumeIDFromChapterID(issueTargetID); ok {
			issueScope = "chapter"
			patchShape = map[string]interface{}{"changed_chapters": []map[string]string{{"id": issueTargetID}}}
		}
		evidenceChapter := chapter
		if evidenceChapter == nil && volume != nil {
			evidenceChapter = findVolumeChapterByID(volume, issueTargetID)
		}
		item := outlineRepairIssue{
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

func buildOutlineGlobalRepairIssues(check *toolCheckResult, category string, outline *models.Outline) []outlineRepairIssue {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	mysteryThreads := collectOutlineMysteryThreads(outline)
	issues := make([]outlineRepairIssue, 0, len(check.Issues))
	for _, issue := range check.Issues {
		issueCategory := normalizeKey(issue.Category)
		if issueCategory == "" {
			issueCategory = category
		}
		nav := toolIssueNavigation(check.Kind, check.Target, check.Scope, check.ID, issue, toolCheckResultTargetWords(*check))
		item := outlineRepairIssue{
			Category:            issue.Category,
			TargetID:            issue.TargetID,
			TargetName:          issue.TargetName,
			Issue:               issue.Issue,
			Suggestion:          issue.Suggestion,
			Priority:            issue.Priority,
			FocusedCheckQuery:   stringMapValue(nav, "focused_check_query"),
			RepairRouteQuery:    stringMapValue(nav, "repair_route_query"),
			RepairContextQuery:  stringMapValue(nav, "repair_context_query"),
			PatchQuery:          stringMapValue(nav, "patch_query"),
			PostPatchCheckQuery: stringMapValue(nav, "post_patch_check_query"),
		}
		if shape, ok := nav["patch_shape"]; ok {
			item.PatchShape = shape
		}
		if issueCategory == "mysteries" && item.PatchShape == nil {
			if thread := firstPatchableOutlineMysteryThread(mysteryThreads); thread != nil {
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

func buildOutlineGlobalPatchTask(issues []outlineRepairIssue) *outlinePatchTask {
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
	category = normalizeKey(category)
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

func collectOutlineMysteryThreads(outline *models.Outline) []outlineMysteryThread {
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
	if normalizeKey(category) != "mysteries" {
		return nil
	}
	if len(threads) > 12 {
		return threads[:12]
	}
	return threads
}

func firstPatchableOutlineMysteryThread(threads []outlineMysteryThread) *outlineMysteryThread {
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

func buildOutlineGlobalRepairNextActions(check *toolCheckResult, issues []outlineRepairIssue) []toolNextAction {
	if patchable := firstPatchableOutlineRepairIssue(issues); patchable != nil {
		_, dryRunCommand, applyCommand := outlinePatchTaskCommands(patchable.PatchQuery, patchable.PatchShape, outlineGlobalPatchTaskID(patchable.Category))
		return []toolNextAction{{
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
		return []toolNextAction{{
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

func firstPatchableOutlineRepairIssue(issues []outlineRepairIssue) *outlineRepairIssue {
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
	brief := outlineBrief{Navigation: map[string]interface{}{
		"volume_context_query":  "novelgen tool query context --type outline-volume --id <volume_id> --view brief",
		"chapter_context_query": "novelgen tool query context --type outline-repair --id <chapter_id> --name <category> --view brief",
	}}
	for _, part := range outline.Parts {
		partItem := partBrief{
			ID:      part.ID,
			Title:   part.Title,
			Summary: clipToolString(part.Summary, 360),
		}
		for _, volume := range part.Volumes {
			partItem.Volumes = append(partItem.Volumes, volumeBrief{
				ID:      volume.ID,
				Title:   volume.Title,
				Summary: clipToolString(volume.Summary, 260),
			})
		}
		brief.Parts = append(brief.Parts, partItem)
	}
	return brief
}

func outlineGlobalRepairStats(outline *models.Outline, check *toolCheckResult) map[string]int {
	stats := map[string]int{}
	if outline != nil {
		stats["part_count"] = len(outline.Parts)
		stats["volume_count"] = countOutlineVolumes(outline)
		stats["chapter_count"] = countOutlineChapters(outline)
	}
	if check != nil {
		stats["returned_issues"] = len(check.Issues)
		stats["summary_total"] = check.Summary.Total
		stats["summary_medium"] = check.Summary.Medium
		stats["summary_low"] = check.Summary.Low
	}
	return stats
}

func collectRepairVolumeEvents(target flatVolume, check *toolCheckResult) []eventHit {
	if check == nil || len(check.Issues) == 0 {
		return collectVolumeEvents(target)
	}
	chapterIDs := map[string]bool{}
	for _, issue := range check.Issues {
		if _, ok := toolVolumeIDFromChapterID(issue.TargetID); ok {
			chapterIDs[issue.TargetID] = true
		}
	}
	if len(chapterIDs) == 0 {
		return collectVolumeEvents(target)
	}
	events := []eventHit{}
	for _, chapter := range target.Volume.Chapters {
		if !chapterIDs[chapter.ID] {
			continue
		}
		for _, event := range chapter.Events {
			events = append(events, eventHit{
				ChapterID:    chapter.ID,
				ChapterTitle: chapter.Title,
				Path: outlinePath{
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

func buildOutlineRepairNextActions(scope, targetID, volumeID, category string) []toolNextAction {
	scope = normalizeToolScope(scope)
	if scope == "" {
		scope = "volume"
	}
	return []toolNextAction{
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

func queryOutlineVolumeContext(ctx toolProjectContext, id, name string) toolResponse {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" && name == "" {
		return toolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{"--id or --name is required for context --type outline-volume"}}
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
		return toolResponse{OK: true, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: id=%q name=%q", id, name)}}
	}

	target := volumes[targetIndex]
	events := collectVolumeEvents(target)
	entities := collectVolumeEntities(target)

	result := outlineVolumeContext{
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result}
}

func buildOutlineVolumeNextActions(volumeID string) []toolNextAction {
	return []toolNextAction{
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

func queryCraftCharacterContext(ctx toolProjectContext, name string) toolResponse {
	return queryCraftElementContext(ctx, "character", name)
}

func queryCraftElementContext(ctx toolProjectContext, target, name string) toolResponse {
	target = normalizeCraftType(target)
	name = strings.TrimSpace(name)
	if name == "" {
		return toolResponse{OK: false, Count: 0, Warnings: []string{fmt.Sprintf("--name is required for context --type craft-%s", target)}}
	}

	craftResp := queryCraft(ctx, target, name)
	refResp := toolResponse{OK: true, Section: "outline", Results: []outlineHit{}}
	chapterResp := toolResponse{OK: true, Section: "chapter", Results: []chapterHit{}}
	if target == "character" || target == "item" || target == "location" {
		refResp = queryOutlineRefs(ctx.Outline, target, name, false)
		chapterResp = queryChaptersByEntity(ctx, target, name, false)
	}
	eventEntityType := target
	if target == "organization" {
		eventEntityType = ""
	}
	eventResp := queryEvents(ctx.Outline, "", "", eventEntityType, name)

	refs := limitOutlineHitResults(refResp.Results, 3)
	chapters := limitChapterHitResults(chapterResp.Results, 3)
	events := limitEventHitResults(eventResp.Results, 4)

	result := craftElementContext{
		Target:           target,
		Name:             name,
		StorySetup:       briefStorySetupForElementContext(ctx.Setup, target, name),
		ExistingCraft:    briefCraftContextResults(craftResp.Results),
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
	return toolResponse{OK: true, Section: "context", Count: 1, Results: result, Warnings: warnings}
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

func buildCraftContextNextActions(target, name string) []toolNextAction {
	return []toolNextAction{
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

func queryAllChapters(ctx toolProjectContext, includeContent bool) toolResponse {
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
			hit.ContentPath, hit.Content = loadToolChapterContent(ctx.Root, item.Chapter.ID)
		}
		chapters = append(chapters, hit)
	}
	return toolResponse{OK: true, Section: "chapter", Count: len(chapters), Results: chapters}
}

func (c *toolProjectContext) loadSetup() error {
	path := filepath.Join(c.Root, "story", "setup", "story_setup.json")
	setup, err := models.LoadStorySetup(path)
	if err != nil {
		return fmt.Errorf("failed to load story setup: %w", err)
	}
	c.Setup = setup
	return nil
}

func (c *toolProjectContext) loadOutline() error {
	path := filepath.Join(c.Root, "story", "compose", "outline.json")
	outline, err := models.LoadOutline(path)
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}
	c.Outline = outline
	return nil
}

func (c *toolProjectContext) loadElements() error {
	characters, locations, items, organizations, err := loadAllElements()
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

func queryNamedElement[T any](kind, name string, values map[string]*T) toolResponse {
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
	return toolResponse{OK: true, Section: "story-setup", Count: len(hits), Results: hits}
}

func rawObjectMatches(key string, value map[string]interface{}, name string) bool {
	if strings.TrimSpace(name) == "" {
		return true
	}
	return textMatches(key, name) || objectNameMatches(value, name)
}

func queryNamedRawElement(kind, name string, values map[string]map[string]interface{}) toolResponse {
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
	return toolResponse{OK: true, Section: "craft", Count: len(hits), Results: hits}
}

func briefCraftContextResults(results interface{}) interface{} {
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

func briefStorySetupForCharacterContext(setup *models.StorySetup, name string) interface{} {
	return briefStorySetupForElementContext(setup, "character", name)
}

func briefStorySetupForElementContext(setup *models.StorySetup, target, name string) interface{} {
	if setup == nil {
		return nil
	}
	brief := storySetupBrief{
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

func queryOutlineNode(outline *models.Outline, nodeType, id string) toolResponse {
	hits := make([]outlineHit, 0)
	for _, part := range outline.Parts {
		partPath := outlinePath{PartID: part.ID, PartTitle: part.Title}
		if nodeType == "part" && idMatches(part.ID, id) {
			hits = append(hits, outlineHit{Type: "part", ID: part.ID, Title: part.Title, Summary: part.Summary, Path: partPath, Object: part})
		}
		for _, volume := range part.Volumes {
			volumePath := partPath
			volumePath.VolumeID = volume.ID
			volumePath.VolumeTitle = volume.Title
			if nodeType == "volume" && idMatches(volume.ID, id) {
				hits = append(hits, outlineHit{Type: "volume", ID: volume.ID, Title: volume.Title, Summary: volume.Summary, Path: volumePath, Object: volume})
			}
			for _, chapter := range volume.Chapters {
				chapterPath := volumePath
				chapterPath.ChapterID = chapter.ID
				chapterPath.ChapterTitle = chapter.Title
				if nodeType == "chapter" && idMatches(chapter.ID, id) {
					hits = append(hits, outlineHit{Type: "chapter", ID: chapter.ID, Title: chapter.Title, Summary: chapter.Summary, Path: chapterPath, Object: chapter})
				}
			}
		}
	}
	return toolResponse{OK: true, Section: "outline", Count: len(hits), Results: hits}
}

func queryChapter(ctx toolProjectContext, id string, includeContent bool) toolResponse {
	if strings.TrimSpace(id) == "" {
		return toolResponse{OK: false, Count: 0, Warnings: []string{"--id is required for chapter queries"}}
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
			hit.ContentPath, hit.Content = loadToolChapterContent(ctx.Root, chapter.Chapter.ID)
		}
		return toolResponse{OK: true, Count: 1, Results: []chapterHit{hit}}
	}
	return toolResponse{OK: true, Count: 0, Results: []chapterHit{}}
}

func queryOutlineRefs(outline *models.Outline, entityType, name string, chaptersOnly bool) toolResponse {
	hits := make([]outlineHit, 0)
	for _, chapter := range flattenChapters(outline) {
		reasons := chapterEntityReasons(chapter.Chapter, entityType, name)
		if len(reasons) == 0 {
			continue
		}
		hits = append(hits, outlineHit{
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
		return toolResponse{OK: true, Count: len(hits), Results: hits}
	}
	return toolResponse{OK: true, Count: len(hits), Results: hits}
}

func queryChaptersByEntity(ctx toolProjectContext, entityType, name string, includeContent bool) toolResponse {
	outlineResp := queryOutlineRefs(ctx.Outline, entityType, name, true)
	outlineHits, _ := outlineResp.Results.([]outlineHit)
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
			chapterHit.ContentPath, chapterHit.Content = loadToolChapterContent(ctx.Root, hit.ID)
		}
		chapters = append(chapters, chapterHit)
	}
	return toolResponse{OK: true, Count: len(chapters), Results: chapters}
}

func queryStorylines(ctx toolProjectContext, name string) toolResponse {
	type storylineHit struct {
		Source  string      `json:"source"`
		Name    string      `json:"name"`
		Object  interface{} `json:"object"`
		Chapter string      `json:"chapter_id,omitempty"`
		Title   string      `json:"chapter_title,omitempty"`
		Path    outlinePath `json:"path,omitempty"`
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
	return toolResponse{OK: true, Count: len(hits), Results: hits}
}

func queryEvents(outline *models.Outline, chapterID, volumeID, entityType, name string) toolResponse {
	hits := make([]eventHit, 0)
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
			hits = append(hits, eventHit{
				ChapterID:    chapter.Chapter.ID,
				ChapterTitle: chapter.Chapter.Title,
				EventIndex:   eventIndex,
				Path:         chapter.Path,
				Event:        event,
				Reasons:      reasons,
			})
		}
	}
	return toolResponse{OK: true, Count: len(hits), Results: hits}
}

type flatChapter struct {
	Chapter *models.Chapter
	Path    outlinePath
}

type flatVolume struct {
	Volume *models.Volume
	Path   outlinePath
}

func flattenVolumes(outline *models.Outline) []flatVolume {
	if outline == nil {
		return nil
	}
	result := make([]flatVolume, 0)
	for _, part := range outline.Parts {
		base := outlinePath{PartID: part.ID, PartTitle: part.Title}
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
		base := outlinePath{PartID: part.ID, PartTitle: part.Title}
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

func collectVolumeEvents(volume flatVolume) []eventHit {
	if volume.Volume == nil {
		return nil
	}
	hits := make([]eventHit, 0)
	for _, chapter := range volume.Volume.Chapters {
		chapterPath := volume.Path
		chapterPath.ChapterID = chapter.ID
		chapterPath.ChapterTitle = chapter.Title
		for eventIndex, event := range chapter.Events {
			hits = append(hits, eventHit{
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

func collectVolumeEntities(volume flatVolume) outlineVolumeEntities {
	if volume.Volume == nil {
		return outlineVolumeEntities{}
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
	return outlineVolumeEntities{
		Characters: clipToolStrings(compactStrings(characters), 25, 70),
		Items:      clipToolStrings(compactStrings(items), 25, 70),
		Locations:  clipToolStrings(compactStrings(locations), 25, 70),
		Storylines: clipToolStrings(compactStrings(storylines), 25, 70),
	}
}

func collectChapterEntities(chapter models.Chapter) outlineVolumeEntities {
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
	return outlineVolumeEntities{
		Characters: clipToolStrings(compactStrings(characters), 18, 70),
		Items:      clipToolStrings(compactStrings(items), 18, 70),
		Locations:  clipToolStrings(compactStrings(locations), 12, 70),
		Storylines: clipToolStrings(compactStrings(storylines), 12, 70),
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
	default:
		reasons := make([]string, 0)
		reasons = append(reasons, prefixedReasons("character", characterChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("item", itemChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("location", locationChapterReasons(chapter, name))...)
		return compactStrings(reasons)
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
	return compactStrings(reasons)
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
	return compactStrings(reasons)
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
	return compactStrings(reasons)
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

func loadToolChapterContent(root, chapterID string) (string, string) {
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

func repairToolLookupText(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	repaired := utils.RepairLikelyMojibakeText(trimmed)
	return repaired, repaired != trimmed
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeContextType(value string) string {
	switch normalizeKey(value) {
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
		return normalizeKey(value)
	}
}

func normalizeEntityType(value string) string {
	switch normalizeKey(value) {
	case "characters", "char":
		return "character"
	case "items":
		return "item"
	case "locations", "loc":
		return "location"
	case "storylines":
		return "storyline"
	default:
		return normalizeKey(value)
	}
}

func limitOutlineHitResults(results interface{}, max int) []outlineHit {
	values, _ := results.([]outlineHit)
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

func limitEventHitResults(results interface{}, max int) []eventHit {
	values, _ := results.([]eventHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func normalizeSetupType(value string) string {
	switch normalizeKey(value) {
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
	case "longform", "long_form_plan", "long-form":
		return "long-form-plan"
	default:
		return normalizeKey(value)
	}
}

func normalizeOutlineType(value string) string {
	switch normalizeKey(value) {
	case "parts":
		return "part"
	case "volumes":
		return "volume"
	case "chapters", "outline-chapter", "outline-chapters":
		return "chapter"
	case "reference", "references":
		return "refs"
	default:
		return normalizeKey(value)
	}
}

func normalizeCraftType(value string) string {
	switch normalizeKey(value) {
	case "characters", "char":
		return "character"
	case "items":
		return "item"
	case "locations", "loc":
		return "location"
	case "organizations", "org":
		return "organization"
	default:
		return normalizeKey(value)
	}
}

func normalizeChapterType(value string) string {
	switch normalizeKey(value) {
	case "reference", "references":
		return "refs"
	case "event":
		return "events"
	case "outline-chapter":
		return "outline"
	default:
		return normalizeKey(value)
	}
}

func prefixedReasons(prefix string, reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, prefix+"."+reason)
	}
	return out
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
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

func applyToolView(resp *toolResponse, view string) {
	if resp == nil {
		return
	}
	view = normalizeToolView(view)
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["view"] = view
	applyToolContextBudget(resp, view)
	if view == "full" || resp.Results == nil {
		return
	}
	switch view {
	case "index":
		resp.Results = indexToolResults(resp.Section, resp.Results)
	case "brief":
		resp.Results = briefToolResults(resp.Section, resp.Results)
	}
}

func applyToolContextBudget(resp *toolResponse, view string) {
	if resp == nil || normalizeKey(resp.Section) != "context" {
		return
	}
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	budget := map[string]interface{}{
		"strategy":           "index_first_then_brief",
		"current_view":       view,
		"avoid":              []string{"full_project_setup", "full_outline", "all_chapters", "source_code_search", "rpg_file_refresh_unless_next_actions_requires_it"},
		"max_extra_queries":  3,
		"prefer_commands_in": []string{"next_actions", "navigation", "workflow"},
	}
	switch view {
	case "index":
		budget["context_level"] = "route_only"
		budget["next_view"] = "brief"
		budget["upgrade_when"] = []string{
			"before composing a non-empty patch",
			"when check/navigation says target facts or excerpts are needed",
			"when current route identifies a specific entity that must be verified",
		}
		budget["stop_when"] = []string{
			"required focused check is clean",
			"next_actions do not request more context",
		}
		if contextIndexHasPatchTask(resp.Results) {
			budget["strategy"] = "patch_task_only"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["prefer_commands_in"] = []string{"patch_task", "next_actions"}
			budget["upgrade_when"] = []string{
				"do not fetch brief context; use patch_task.dry_run_command and patch_task.apply_command directly",
			}
			budget["stop_when"] = []string{
				"patch_task post_patch_check_query has been run",
				"next_actions request post_patch_check or return_final_json",
			}
		} else if contextIndexHasUnpatchableGlobalOnly(resp.Results) {
			budget["strategy"] = "route_only_classify_unpatchable"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 1
			budget["upgrade_when"] = []string{
				"do not fetch brief context unless a later focused check returns a patchable target",
			}
			budget["stop_when"] = []string{
				"issue_context has no patch_query plus patch_shape",
				"next_actions request return_final_json",
			}
		} else if contextIndexHasCleanFocusedCheck(resp.Results) {
			budget["strategy"] = "route_only_clean_focused_check"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["upgrade_when"] = []string{
				"do not fetch brief context when the scoped check returned zero issues",
			}
			budget["stop_when"] = []string{
				"focused check summary total is zero",
				"next_actions request return_final_json",
			}
		} else if contextIndexHasRefreshFirst(resp.Results) {
			budget["strategy"] = "route_only_refresh_derived_first"
			budget["next_view"] = "none"
			budget["max_extra_queries"] = 0
			budget["upgrade_when"] = []string{
				"do not fetch brief context before running the refresh and post-refresh check",
			}
			budget["stop_when"] = []string{
				"post-refresh focused check is clean",
				"next_actions request return_final_json",
			}
		}
	case "brief":
		budget["context_level"] = "focused_facts"
		budget["next_view"] = "targeted_query_only"
		budget["upgrade_when"] = []string{
			"only query exact chapter/entity/event IDs named by this bundle",
			"only use full view for one precise small object when brief is insufficient",
		}
		budget["stop_when"] = []string{
			"patch dry-run succeeds and workflow does not allow apply",
			"post-apply focused check is clean",
		}
	case "full":
		budget["context_level"] = "full_facts"
		budget["next_view"] = "none"
		budget["upgrade_when"] = []string{
			"do not broaden further; switch to patch/check or a narrower exact query",
		}
		budget["stop_when"] = []string{
			"target object is understood enough to patch or return final JSON",
		}
	default:
		budget["context_level"] = "focused_facts"
	}
	resp.Meta["context_budget"] = budget
}

func contextIndexHasPatchTask(results interface{}) bool {
	switch typed := results.(type) {
	case outlineGlobalRepairContext:
		return typed.PatchTask != nil
	case map[string]interface{}:
		task, ok := typed["patch_task"]
		if !ok || task == nil {
			return false
		}
		switch value := task.(type) {
		case *outlinePatchTask:
			return value != nil
		case outlinePatchTask:
			return strings.TrimSpace(value.PatchQuery) != ""
		case map[string]interface{}:
			return strings.TrimSpace(fmt.Sprint(value["patch_query"])) != "" && value["patch_shape"] != nil
		default:
			return true
		}
	default:
		return false
	}
}

func contextIndexHasRefreshFirst(results interface{}) bool {
	switch typed := results.(type) {
	case chapterRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case outlineRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case recapRepairContext:
		return toolCheckResultNeedsRefreshFirst(typed.Check)
	case map[string]interface{}:
		actions, ok := typed["next_actions"].([]toolNextAction)
		if !ok || len(actions) == 0 {
			return false
		}
		return actions[0].Action == "refresh_derived_dsl_first"
	default:
		return false
	}
}

func toolCheckResultNeedsRefreshFirst(check *toolCheckResult) bool {
	if check == nil || len(check.Issues) == 0 {
		return false
	}
	actions := toolCheckNextActions(*check)
	return len(actions) > 0 && actions[0].Action == "refresh_derived_dsl_first"
}

func contextIndexHasCleanFocusedCheck(results interface{}) bool {
	switch typed := results.(type) {
	case chapterRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case outlineRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case recapRepairContext:
		return toolCheckResultIsClean(typed.Check)
	case map[string]interface{}:
		raw, ok := typed["check"].(map[string]interface{})
		if !ok {
			return false
		}
		return toolCheckIndexMapIsClean(raw)
	default:
		return false
	}
}

func toolCheckResultIsClean(check *toolCheckResult) bool {
	if check == nil {
		return false
	}
	return check.Summary.Total == 0 && len(check.Issues) == 0
}

func toolCheckIndexMapIsClean(check map[string]interface{}) bool {
	if check == nil {
		return false
	}
	summary, ok := check["summary"].(map[string]interface{})
	if !ok {
		return false
	}
	return numericMapInt(summary, "total") == 0
}

func numericMapInt(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := strconv.Atoi(value.String())
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func contextIndexHasUnpatchableGlobalOnly(results interface{}) bool {
	switch typed := results.(type) {
	case outlineGlobalRepairContext:
		if len(typed.IssueContext) == 0 {
			return false
		}
		return !outlineRepairIssuesHavePatchRoute(typed.IssueContext)
	case map[string]interface{}:
		if _, ok := typed["issue_category"]; !ok {
			return false
		}
		issues, ok := typed["issue_context"]
		if !ok {
			return false
		}
		return !outlineIssueContextHasPatchRoute(issues)
	default:
		return false
	}
}

func outlineIssueContextHasPatchRoute(issues interface{}) bool {
	switch typed := issues.(type) {
	case []map[string]interface{}:
		if len(typed) == 0 {
			return true
		}
		for _, issue := range typed {
			patchQuery, _ := issue["patch_query"].(string)
			if strings.TrimSpace(patchQuery) != "" && issue["patch_shape"] != nil {
				return true
			}
		}
		return false
	case []interface{}:
		if len(typed) == 0 {
			return true
		}
		for _, raw := range typed {
			issue, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			patchQuery, _ := issue["patch_query"].(string)
			if strings.TrimSpace(patchQuery) != "" && issue["patch_shape"] != nil {
				return true
			}
		}
		return false
	case []outlineRepairIssue:
		return outlineRepairIssuesHavePatchRoute(typed)
	default:
		return true
	}
}

func applyToolFields(resp *toolResponse, rawFields string) {
	if resp == nil || resp.Results == nil {
		return
	}
	fields := parseToolFields(rawFields)
	if len(fields) == 0 {
		return
	}
	projected, ok := projectToolFields(resp.Results, fields)
	if !ok {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("no requested fields found: %s", strings.Join(fields, ",")))
	}
	resp.Results = projected
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["fields"] = fields
}

func parseToolFields(raw string) []string {
	seen := map[string]bool{}
	fields := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		field := normalizeToolFieldPath(item)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		fields = append(fields, field)
	}
	return fields
}

func normalizeToolFieldPath(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		key := normalizeKey(part)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return strings.Join(out, ".")
}

func projectToolFields(value interface{}, fields []string) (interface{}, bool) {
	data, err := json.Marshal(value)
	if err != nil {
		return value, false
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return value, false
	}
	return projectToolFieldValue(raw, fields)
}

func projectToolFieldValue(value interface{}, fields []string) (interface{}, bool) {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		foundAny := false
		for _, item := range typed {
			projected, found := projectToolFieldValue(item, fields)
			foundAny = foundAny || found
			out = append(out, projected)
		}
		return out, foundAny
	case map[string]interface{}:
		return projectToolFieldMap(typed, fields)
	default:
		return value, false
	}
}

func projectToolFieldMap(value map[string]interface{}, fields []string) (map[string]interface{}, bool) {
	out := map[string]interface{}{}
	for _, key := range []string{"type", "id", "title", "summary", "chapter_id", "chapter_title", "path", "reasons", "navigation", "stats"} {
		if v, ok := value[key]; ok {
			out[key] = v
		}
	}

	foundAny := false
	for _, field := range fields {
		if v, ok := getToolFieldPath(value, field); ok {
			out[toolFieldOutputKey(field)] = v
			foundAny = true
			continue
		}
		for _, nested := range []string{"object", "outline", "event", "target_volume", "previous_volume", "next_volume"} {
			child, ok := value[nested].(map[string]interface{})
			if !ok {
				continue
			}
			if v, ok := getToolFieldPath(child, field); ok {
				out[toolFieldOutputKey(field)] = v
				foundAny = true
				break
			}
		}
	}
	return out, foundAny
}

func getToolFieldPath(value map[string]interface{}, path string) (interface{}, bool) {
	current := interface{}(value)
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func toolFieldOutputKey(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func normalizeToolView(view string) string {
	switch normalizeKey(view) {
	case "", "brief", "summary":
		return "brief"
	case "full", "raw":
		return "full"
	case "index", "list":
		return "index"
	default:
		return "brief"
	}
}

func briefToolResults(section string, results interface{}) interface{} {
	switch normalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, false)
	case "outline":
		return briefOutlineResults(results, false)
	case "chapter", "chapters":
		return briefChapterResults(results, false)
	case "craft":
		return briefCraftResults(results, false)
	case "context":
		return briefContextResults(results, false)
	case "logs", "log":
		return briefLogResults(results, false)
	default:
		return results
	}
}

func indexToolResults(section string, results interface{}) interface{} {
	switch normalizeKey(section) {
	case "story-setup", "setup":
		return briefStorySetupResults(results, true)
	case "outline":
		return briefOutlineResults(results, true)
	case "chapter", "chapters":
		return briefChapterResults(results, true)
	case "craft":
		return briefCraftResults(results, true)
	case "context":
		return briefContextResults(results, true)
	case "logs", "log":
		return briefLogResults(results, true)
	default:
		return results
	}
}

func briefLogResults(results interface{}, indexOnly bool) interface{} {
	hits, ok := results.([]logHit)
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(hits))
	for _, hit := range hits {
		item := map[string]interface{}{
			"id":           hit.ID,
			"kind":         hit.Kind,
			"agent":        hit.Agent,
			"size_bytes":   hit.SizeBytes,
			"modified_at":  hit.ModifiedAt,
			"detail_query": hit.DetailQuery,
		}
		if len(hit.Summary) > 0 {
			item["summary"] = hit.Summary
		}
		if !indexOnly {
			if hit.Preview != "" {
				item["preview"] = clipToolString(hit.Preview, 1000)
			}
			if hit.Content != "" {
				item["content"] = hit.Content
			}
			if hit.Truncated {
				item["truncated"] = true
			}
		}
		out = append(out, item)
	}
	return out
}

func briefCraftResults(results interface{}, indexOnly bool) interface{} {
	switch typed := results.(type) {
	case []map[string]interface{}:
		out := make([]map[string]interface{}, 0, len(typed))
		for _, hit := range typed {
			if indexOnly {
				out = append(out, indexCraftHit(hit))
			} else {
				out = append(out, briefCraftHit(hit))
			}
		}
		return out
	case map[string]interface{}:
		if indexOnly {
			out := map[string]interface{}{}
			for section, value := range typed {
				out[section] = craftCollectionIndex(section, value)
			}
			return out
		}
		return results
	default:
		return results
	}
}

func briefCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range []string{"type", "key"} {
		if value, ok := hit[key]; ok {
			out[key] = value
		}
	}
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		out["object"] = compactCraftObject(raw)
	} else if object, ok := hit["object"]; ok {
		out["object"] = object
	}
	return out
}

func indexCraftHit(hit map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	key := fmt.Sprint(hit["key"])
	kind := normalizeCraftType(fmt.Sprint(hit["type"]))
	if key != "" {
		out["key"] = key
	}
	if kind != "" {
		out["type"] = kind
	}
	name := key
	if raw, ok := hit["object"].(map[string]interface{}); ok {
		if objectName := stringMapValue(raw, "name"); objectName != "" {
			name = objectName
			out["name"] = objectName
		}
		for _, field := range []string{"role_in_story", "rpg_role", "combat_role", "type", "category"} {
			if value, ok := raw[field]; ok {
				if text, ok := value.(string); ok {
					out[field] = clipToolString(text, 120)
				} else {
					out[field] = value
				}
			}
		}
	}
	if kind != "" && name != "" {
		out["detail_query"] = fmt.Sprintf("novelgen tool query craft --type %s --name %q --view brief", kind, name)
	}
	return out
}

func craftCollectionIndex(section string, value interface{}) interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]interface{}{"count": 0}
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]interface{}{"count": 0}
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return map[string]interface{}{
		"count":        len(keys),
		"keys":         keys,
		"detail_query": fmt.Sprintf("novelgen tool query craft --type %s --name <name> --view brief", craftCollectionType(section)),
	}
}

func craftCollectionType(section string) string {
	switch normalizeKey(section) {
	case "characters":
		return "character"
	case "items":
		return "item"
	case "locations":
		return "location"
	case "organizations", "orgs":
		return "organization"
	default:
		return normalizeKey(section)
	}
}

func briefContextResults(results interface{}, indexOnly bool) interface{} {
	if indexOnly {
		return indexContextResults(results)
	}
	switch typed := results.(type) {
	case chapterRepairContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case chapterWriteContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case outlineRepairContext:
		typed.ParentVolume = stripContextVolumePayoff(typed.ParentVolume)
		typed.Current = stripContextVolumePayoff(typed.Current)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case outlineGlobalRepairContext:
		typed.StorySetup = nil
		typed.Outline = nil
		if len(typed.MysteryThreads) > 12 {
			typed.MysteryThreads = typed.MysteryThreads[:12]
		}
		return typed
	case outlineVolumeContext:
		typed.TargetVolume = stripContextVolumePayoff(typed.TargetVolume)
		typed.PreviousVolume = stripContextVolumePayoff(typed.PreviousVolume)
		typed.NextVolume = stripContextVolumePayoff(typed.NextVolume)
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	case craftElementContext:
		typed.Events = stripContextEventPaths(typed.Events)
		return typed
	default:
		return results
	}
}

func indexContextResults(results interface{}) interface{} {
	switch typed := results.(type) {
	case chapterRepairContext:
		return map[string]interface{}{
			"chapter_id":     typed.ChapterID,
			"path":           typed.Path,
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   chapterRepairIndexNextActions(typed),
			"stats":          typed.Stats,
			"entity_index":   typed.EntityIndex,
		}
	case chapterWriteContext:
		return map[string]interface{}{
			"chapter_id":   typed.ChapterID,
			"path":         typed.Path,
			"navigation":   typed.Navigation,
			"workflow":     toolWorkflowIndex(typed.Workflow),
			"next_actions": chapterWriteIndexNextActions(typed),
			"stats":        typed.Stats,
			"entity_index": typed.EntityIndex,
		}
	case recapRepairContext:
		return map[string]interface{}{
			"chapter_id":   typed.ChapterID,
			"path":         typed.Path,
			"check":        toolCheckIndex(typed.Check),
			"navigation":   typed.Navigation,
			"workflow":     toolWorkflowIndex(typed.Workflow),
			"next_actions": recapRepairIndexNextActions(typed),
			"stats":        typed.Stats,
		}
	case outlineRepairContext:
		return map[string]interface{}{
			"scope":          typed.Scope,
			"id":             typed.ID,
			"path":           typed.Path,
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"issue_context":  outlineRepairIssueIndex(typed.IssueContext),
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   outlineRepairIndexNextActions(typed),
			"stats":          typed.Stats,
		}
	case outlineGlobalRepairContext:
		return map[string]interface{}{
			"issue_category": typed.IssueCategory,
			"check":          toolCheckIndex(typed.Check),
			"issue_context":  outlineGlobalRepairIssueIndex(typed.IssueContext),
			"patch_task":     typed.PatchTask,
			"navigation":     typed.Navigation,
			"workflow":       toolWorkflowIndex(typed.Workflow),
			"next_actions":   outlineGlobalRepairIndexNextActions(typed),
			"stats":          typed.Stats,
		}
	case outlineVolumeContext:
		return map[string]interface{}{
			"volume_id":    typed.VolumeID,
			"path":         typed.Path,
			"navigation":   typed.Navigation,
			"next_actions": typed.NextActions,
			"stats":        typed.Stats,
			"entity_index": indexOutlineVolumeEntities(typed.EntityIndex),
		}
	case craftElementContext:
		return map[string]interface{}{
			"target":       typed.Target,
			"name":         typed.Name,
			"navigation":   typed.Navigation,
			"next_actions": craftElementIndexNextActions(typed),
			"stats":        typed.Stats,
		}
	default:
		return results
	}
}

func chapterWriteIndexNextActions(ctx chapterWriteContext) []toolNextAction {
	out := []toolNextAction{{
		Step:    1,
		Action:  "query_brief_context",
		Purpose: "Fetch the focused write bundle before drafting; index view is only the route map.",
		Command: fmt.Sprintf("novelgen tool query context --type chapter-write --id %q --view brief", ctx.ChapterID),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func chapterRepairIndexNextActions(ctx chapterRepairContext) []toolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			fmt.Sprintf("novelgen tool check all --target chapter --scope chapter --id %q --category %s --min-priority low --max-issues 12", ctx.ChapterID, categoryOrAll(ctx.IssueCategory)),
			"The scoped final-chapter check returned zero issues; do not fetch brief repair context or patch prose.",
		)
	}
	if actions := refreshFirstIndexNextActions(ctx.Check); len(actions) > 0 {
		return actions
	}
	out := []toolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused repair bundle before patching; index view is only the route map and check summary.",
		Command: fmt.Sprintf("novelgen tool query context --type chapter-repair --id %q --name %q --view brief", ctx.ChapterID, categoryOrAll(ctx.IssueCategory)),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_repair_bundle" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func outlineRepairIndexNextActions(ctx outlineRepairContext) []toolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			fmt.Sprintf("novelgen tool check all --target outline --scope %s --id %q --category %s --min-priority low --max-issues 8", ctx.Scope, ctx.ID, categoryOrAll(ctx.IssueCategory)),
			"The scoped outline check returned zero issues; do not fetch brief repair context or patch outline.",
		)
	}
	if actions := refreshFirstIndexNextActions(ctx.Check); len(actions) > 0 {
		return actions
	}
	out := []toolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused outline repair bundle before patching; index view is only the route map and check summary.",
		Command: fmt.Sprintf("novelgen tool query context --type outline-repair --id %q --name %q --view brief", ctx.ID, categoryOrAll(ctx.IssueCategory)),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_repair_bundle" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func cleanFocusedCheckNextActions(checkCommand, purpose string) []toolNextAction {
	return []toolNextAction{{
		Step:    1,
		Action:  "focused_check_clean",
		Purpose: purpose,
		Command: checkCommand,
	}, {
		Step:    2,
		Action:  "return_final_json",
		Purpose: "Report that the focused check is clean and stop tool exploration.",
	}}
}

func refreshFirstIndexNextActions(check *toolCheckResult) []toolNextAction {
	if check == nil || len(check.Issues) == 0 {
		return nil
	}
	actions := toolCheckNextActions(*check)
	if len(actions) == 0 || actions[0].Action != "refresh_derived_dsl_first" {
		return nil
	}
	out := make([]toolNextAction, len(actions))
	copy(out, actions)
	for i := range out {
		out[i].Step = i + 1
	}
	return out
}

func outlineGlobalRepairIndexNextActions(ctx outlineGlobalRepairContext) []toolNextAction {
	if !outlineRepairIssuesHavePatchRoute(ctx.IssueContext) {
		out := []toolNextAction{{
			Step:    1,
			Action:  "classify_unpatchable_global_issue",
			Purpose: "No returned global issue has both patch_query and patch_shape; do not fetch brief context or invent a broad patch.",
		}}
		if category := strings.TrimSpace(ctx.IssueCategory); category != "" {
			out = append(out, toolNextAction{
				Step:    len(out) + 1,
				Action:  "focused_recheck",
				Purpose: "Confirm whether the global diagnostic still reproduces before routing to a smaller workflow.",
				Command: fmt.Sprintf("novelgen tool check all --target outline --scope all --category %s --min-priority low --max-issues 12", categoryOrAll(category)),
				When:    "Only when you need to confirm the diagnostic; otherwise return final JSON with remaining issue notes.",
			})
		}
		out = append(out, toolNextAction{
			Step:    len(out) + 1,
			Action:  "return_final_json",
			Purpose: "Report the issue as remaining/unpatchable from this route, or recommend a smaller targeted workflow.",
		})
		return out
	}
	if ctx.PatchTask == nil {
		out := []toolNextAction{{
			Step:    1,
			Action:  "query_brief_global_repair_context",
			Purpose: "Fetch the focused global repair bundle before patching because this index response has no complete patch_task.",
			Command: fmt.Sprintf("novelgen tool query context --type outline-global-repair --name %q --view brief", categoryOrAll(ctx.IssueCategory)),
		}}
		for _, action := range ctx.NextActions {
			if action.Action == "use_patch_task" || action.Action == "use_current_global_repair_bundle" {
				continue
			}
			action.Step = len(out) + 1
			out = append(out, action)
		}
		return out
	}
	out := []toolNextAction{{
		Step:    1,
		Action:  "use_patch_task",
		Purpose: "Use patch_task.dry_run_command and patch_task.apply_command from this index response directly. Do not query brief context unless patch_task is absent.",
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_patch_task" {
			continue
		}
		if ctx.PatchTask != nil {
			switch action.Action {
			case "patch_dry_run":
				action.Command = ctx.PatchTask.DryRunCommand
			case "patch_apply":
				action.Command = ctx.PatchTask.ApplyCommand
			}
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func outlineRepairIssuesHavePatchRoute(issues []outlineRepairIssue) bool {
	for _, issue := range issues {
		if strings.TrimSpace(issue.PatchQuery) == "" || issue.PatchShape == nil {
			continue
		}
		return true
	}
	return false
}

func recapRepairIndexNextActions(ctx recapRepairContext) []toolNextAction {
	if toolCheckResultIsClean(ctx.Check) {
		return cleanFocusedCheckNextActions(
			fmt.Sprintf("novelgen tool check quality --target recap --scope chapter --id %q --min-priority low --max-issues 8", ctx.ChapterID),
			"The scoped recap check returned zero issues; do not fetch brief repair context or patch recap.",
		)
	}
	out := []toolNextAction{{
		Step:    1,
		Action:  "query_brief_repair_context",
		Purpose: "Fetch the focused recap repair bundle before patching; index view is only the route map and check summary.",
		Command: fmt.Sprintf("novelgen tool query context --type recap-repair --id %q --view brief", ctx.ChapterID),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func craftElementIndexNextActions(ctx craftElementContext) []toolNextAction {
	out := []toolNextAction{{
		Step:    1,
		Action:  "query_brief_context",
		Purpose: "Fetch the focused craft bundle before patching; index view is only the route map and counts.",
		Command: fmt.Sprintf("novelgen tool query context --type craft-%s --name %q --view brief", ctx.Target, ctx.Name),
	}}
	for _, action := range ctx.NextActions {
		if action.Action == "use_current_context" {
			continue
		}
		action.Step = len(out) + 1
		out = append(out, action)
	}
	return out
}

func indexOutlineVolumeEntities(entities outlineVolumeEntities) outlineVolumeEntities {
	return outlineVolumeEntities{
		Characters: firstNStrings(entities.Characters, 12),
		Items:      firstNStrings(entities.Items, 12),
		Locations:  firstNStrings(entities.Locations, 10),
		Storylines: firstNStrings(entities.Storylines, 8),
	}
}

func outlineRepairIssueIndex(issues []outlineRepairIssue) []map[string]interface{} {
	if len(issues) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		out = append(out, map[string]interface{}{
			"category":               issue.Category,
			"target_id":              issue.TargetID,
			"target_name":            issue.TargetName,
			"priority":               issue.Priority,
			"focused_check_query":    issue.FocusedCheckQuery,
			"repair_context_query":   issue.RepairContextQuery,
			"patch_query":            issue.PatchQuery,
			"patch_shape":            issue.PatchShape,
			"post_patch_check_query": issue.PostPatchCheckQuery,
		})
	}
	return out
}

func outlineGlobalRepairIssueIndex(issues []outlineRepairIssue) []map[string]interface{} {
	if len(issues) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(issues))
	for _, issue := range issues {
		item := map[string]interface{}{
			"category":            issue.Category,
			"target_id":           issue.TargetID,
			"target_name":         issue.TargetName,
			"priority":            issue.Priority,
			"focused_check_query": issue.FocusedCheckQuery,
			"patch_query":         issue.PatchQuery,
			"patch_shape":         issue.PatchShape,
		}
		if strings.TrimSpace(issue.PatchQuery) != "" && issue.PatchShape != nil {
			item["repair_context_query"] = issue.RepairContextQuery
			item["post_patch_check_query"] = issue.PostPatchCheckQuery
		}
		out = append(out, item)
	}
	return out
}

func toolCheckIndex(check *toolCheckResult) interface{} {
	if check == nil {
		return nil
	}
	return map[string]interface{}{
		"kind":     check.Kind,
		"target":   check.Target,
		"scope":    check.Scope,
		"id":       check.ID,
		"ok":       check.OK,
		"blocking": check.Blocking,
		"score":    check.Score,
		"summary":  check.Summary,
	}
}

func toolWorkflowIndex(workflow map[string]interface{}) map[string]interface{} {
	if len(workflow) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	for key, value := range workflow {
		normalized := normalizeKey(key)
		if normalized == "steps" || normalized == "stop_rules" {
			continue
		}
		if strings.HasSuffix(normalized, "_query") ||
			strings.Contains(normalized, "query") ||
			normalized == "goal" ||
			normalized == "scope" ||
			normalized == "target_id" ||
			normalized == "target_chapter_id" ||
			normalized == "issue_category" ||
			normalized == "patch_shape" ||
			normalized == "patch_query" ||
			normalized == "parent_volume_id" ||
			normalized == "agent_rule" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stripContextVolumePayoff(value interface{}) interface{} {
	switch typed := value.(type) {
	case volumeBrief:
		typed.PayoffContract = nil
		return typed
	default:
		return value
	}
}

func stripContextEventPaths(value interface{}) interface{} {
	events, ok := value.([]eventHitBrief)
	if !ok {
		return value
	}
	out := make([]eventHitBrief, 0, len(events))
	for _, event := range events {
		event.Path = nil
		out = append(out, event)
	}
	return out
}

func briefStorySetupResults(results interface{}, indexOnly bool) interface{} {
	setup, ok := results.(*models.StorySetup)
	if !ok || setup == nil {
		return results
	}
	brief := storySetupBrief{
		ProjectName:    setup.ProjectName,
		Genres:         setup.Genres,
		TargetAudience: clipToolString(setup.TargetAudience, 160),
		Tone:           clipToolString(setup.Tone, 160),
		POVStyle:       setup.POVStyle,
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
	if !indexOnly {
		brief.Premise = clipToolString(setup.Premise, 900)
		brief.Theme = clipToolString(setup.Theme, 500)
		brief.Rules = clipToolStrings(setup.Rules, 8, 220)
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, false)
	} else {
		brief.LongFormPlan = makeLongFormPlanBrief(setup.LongFormPlan, true)
	}
	for _, cast := range setup.CoreCast {
		item := coreCastBrief{ID: cast.ID, Name: cast.Name, Role: cast.Role, Importance: cast.Importance, EntryPhase: cast.EntryPhase, StorylineRefs: cast.StorylineRefs}
		if !indexOnly {
			item.StoryFunction = clipToolString(cast.StoryFunction, 220)
			item.RelationshipToLead = clipToolString(cast.RelationshipToLead, 160)
			item.RelationshipArc = clipToolString(cast.RelationshipArc, 180)
			item.Payoff = clipToolString(cast.Payoff, 180)
		}
		brief.CoreCast = append(brief.CoreCast, item)
	}
	for _, storyline := range setup.Storylines {
		item := storylineBrief{Name: storyline.Name, Type: storyline.Type, Importance: storyline.Importance, Scope: storyline.Scope}
		if !indexOnly {
			item.Description = clipToolString(storyline.Description, 260)
			item.SetupRole = clipToolString(storyline.SetupRole, 180)
			item.RepeatablePressure = clipToolString(storyline.RepeatablePressure, 160)
			item.PayoffCadence = clipToolString(storyline.PayoffCadence, 140)
			item.FailureMode = clipToolString(storyline.FailureMode, 160)
			item.OpenQuestion = clipToolString(storyline.OpenQuestion, 160)
			item.AppealEngine = storyline.AppealEngine
		}
		brief.Storylines = append(brief.Storylines, item)
	}
	for _, premise := range setup.Premises {
		item := premiseBrief{Name: premise.Name, Category: premise.Category}
		if !indexOnly {
			item.Description = clipToolString(premise.Description, 260)
			item.Progression = makeProgressionBrief(premise.Progression)
		}
		brief.Premises = append(brief.Premises, item)
	}
	for _, entry := range setup.WorldTimeline {
		item := worldTimelineBrief{Year: entry.Year, Event: clipToolString(entry.Event, 180)}
		if !indexOnly {
			item.Impact = clipToolString(entry.Impact, 180)
			item.RelatedMystery = entry.RelatedMystery
		}
		brief.WorldTimeline = append(brief.WorldTimeline, item)
	}
	for _, resource := range setup.WorldResources {
		item := worldResourceBrief{Name: resource.Name, Category: resource.Category, Scarcity: resource.Scarcity}
		if !indexOnly {
			item.Description = clipToolString(resource.Description, 180)
		}
		brief.WorldResources = append(brief.WorldResources, item)
	}
	return brief
}

func makeLongFormPlanBrief(plan *models.LongFormPlan, indexOnly bool) interface{} {
	if plan == nil || plan.IsZero() {
		return nil
	}
	brief := longFormPlanBrief{
		TargetChapters: plan.TargetChapters,
		TargetVolumes:  plan.TargetVolumes,
		MainLoop:       clipToolString(plan.MainLoop, 180),
	}
	if indexOnly {
		return brief
	}
	brief.EscalationLadder = clipToolStrings(plan.EscalationLadder, 8, 120)
	brief.ReaderPromises = clipToolStrings(plan.ReaderPromises, 8, 120)
	brief.PayoffCadence = clipToolString(plan.PayoffCadence, 180)
	brief.VolumePattern = clipToolStrings(plan.VolumePattern, 10, 100)
	brief.MidpointMutation = clipToolString(plan.MidpointMutation, 180)
	brief.EndgamePromise = clipToolString(plan.EndgamePromise, 180)
	return brief
}

func makeProgressionBrief(stages []models.ProgressionStage) []progressionStageBrief {
	if len(stages) == 0 {
		return nil
	}
	out := make([]progressionStageBrief, 0, len(stages))
	for _, stage := range stages {
		out = append(out, progressionStageBrief{
			Level: stage.Level,
			Name:  clipToolString(stage.Name, 80),
		})
	}
	return out
}

func briefOutlineResults(results interface{}, indexOnly bool) interface{} {
	switch values := results.(type) {
	case *models.Outline:
		if values == nil {
			return results
		}
		out := outlineBrief{Navigation: map[string]interface{}{
			"detail_queries": []string{
				"novelgen tool query outline --type volume --id <volume_id>",
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		}}
		for _, part := range values.Parts {
			out.Parts = append(out.Parts, makePartBrief(part, indexOnly))
		}
		return out
	case []outlineHit:
		out := make([]outlineHit, 0, len(values))
		for _, hit := range values {
			hit.Object = briefOutlineObject(hit.Object, indexOnly)
			out = append(out, hit)
		}
		return out
	case []eventHit:
		out := make([]eventHitBrief, 0, len(values))
		for _, hit := range values {
			out = append(out, makeEventHitBrief(hit, indexOnly))
		}
		return out
	default:
		return results
	}
}

func briefOutlineObject(value interface{}, indexOnly bool) interface{} {
	switch typed := value.(type) {
	case models.Part:
		return makePartBrief(typed, indexOnly)
	case models.Volume:
		return makeVolumeBrief(typed, indexOnly)
	case models.Chapter:
		return makeChapterBrief(typed, indexOnly)
	case *models.Part:
		if typed != nil {
			return makePartBrief(*typed, indexOnly)
		}
	case *models.Volume:
		if typed != nil {
			return makeVolumeBrief(*typed, indexOnly)
		}
	case *models.Chapter:
		if typed != nil {
			return makeChapterBrief(*typed, indexOnly)
		}
	}
	return value
}

func makePartBrief(part models.Part, indexOnly bool) partBrief {
	brief := partBrief{ID: part.ID, Title: part.Title, Summary: clipToolString(part.Summary, 500)}
	for _, volume := range part.Volumes {
		brief.Volumes = append(brief.Volumes, makeVolumeBrief(volume, indexOnly))
	}
	return brief
}

func makeVolumeBrief(volume models.Volume, indexOnly bool) volumeBrief {
	brief := volumeBrief{
		ID:      volume.ID,
		Title:   volume.Title,
		Summary: clipToolString(volume.Summary, 700),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id>",
				"novelgen tool query outline --type events --chapter-id <chapter_id>",
			},
		},
	}
	if !indexOnly && !volume.PayoffContract.IsZero() {
		brief.PayoffContract = volume.PayoffContract
	}
	for _, chapter := range volume.Chapters {
		brief.Chapters = append(brief.Chapters, makeChapterBrief(chapter, true))
	}
	return brief
}

func makeContextVolumeBrief(volume models.Volume, includeChapters bool) volumeBrief {
	brief := volumeBrief{
		ID:      volume.ID,
		Title:   volume.Title,
		Summary: clipToolString(volume.Summary, 360),
		Navigation: map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !volume.PayoffContract.IsZero() {
		brief.PayoffContract = volume.PayoffContract
	}
	if !includeChapters {
		return brief
	}
	for _, chapter := range volume.Chapters {
		brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
	}
	return brief
}

func makeContextVolumeRouteMap(volume models.Volume, includeChapters bool) map[string]interface{} {
	brief := map[string]interface{}{
		"id":      volume.ID,
		"title":   volume.Title,
		"summary": clipToolString(volume.Summary, 180),
		"navigation": map[string]interface{}{
			"chapter_queries": []string{
				"novelgen tool query outline --type chapter --id <chapter_id> --view brief",
				"novelgen tool query outline --type chapter --id <chapter_id> --fields location,timeline,state_anchor --view brief",
				"novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			},
		},
	}
	if !includeChapters {
		return brief
	}
	chapters := make([]map[string]interface{}, 0, len(volume.Chapters))
	for _, chapter := range volume.Chapters {
		chapters = append(chapters, map[string]interface{}{
			"id":            chapter.ID,
			"title":         chapter.Title,
			"characters":    clipToolStrings(chapter.Characters, 6, 40),
			"location":      clipToolString(chapter.Location, 80),
			"event_count":   len(chapter.Events),
			"advance_count": len(chapter.StorylineAdvances),
		})
	}
	brief["chapters"] = chapters
	return brief
}

func makeRepairVolumeBrief(volume models.Volume, check *toolCheckResult) volumeBrief {
	brief := makeContextVolumeBrief(volume, false)
	chapterIDs := map[string]bool{}
	if check != nil {
		for _, issue := range check.Issues {
			if _, ok := toolVolumeIDFromChapterID(issue.TargetID); ok {
				chapterIDs[issue.TargetID] = true
			}
		}
	}
	for _, chapter := range volume.Chapters {
		if len(chapterIDs) == 0 || chapterIDs[chapter.ID] {
			brief.Chapters = append(brief.Chapters, makeContextChapterBrief(chapter))
		}
		if len(chapterIDs) == 0 && len(brief.Chapters) >= 8 {
			break
		}
	}
	return brief
}

func makeContextChapterBrief(chapter models.Chapter) chapterBrief {
	return chapterBrief{
		ID:           chapter.ID,
		Title:        chapter.Title,
		Summary:      clipToolString(chapter.Summary, 220),
		Characters:   clipToolStrings(chapter.Characters, 12, 50),
		Location:     clipToolString(chapter.Location, 90),
		StateChange:  clipToolString(chapter.StateChange, 120),
		EventCount:   len(chapter.Events),
		AdvanceCount: len(chapter.StorylineAdvances),
	}
}

func makeRepairChapterBrief(chapter models.Chapter) chapterBrief {
	brief := chapterBrief{
		ID:                chapter.ID,
		Title:             chapter.Title,
		Summary:           clipToolString(chapter.Summary, 360),
		Characters:        clipToolStrings(chapter.Characters, 12, 50),
		Location:          clipToolString(chapter.Location, 90),
		OpeningBeat:       clipToolString(chapter.OpeningBeat, 160),
		ClosingBeat:       clipToolString(chapter.ClosingBeat, 160),
		StateChange:       clipToolString(chapter.StateChange, 180),
		Conflict:          clipToolString(chapter.Conflict, 200),
		Pacing:            clipToolString(chapter.Pacing, 120),
		Timeline:          chapter.Timeline,
		StateAnchor:       chapter.StateAnchor,
		StorylineAdvances: compactStorylineAdvances(chapter.StorylineAdvances, 4),
		ChapterPayoff:     chapter.ChapterPayoff,
		EventCount:        len(chapter.Events),
		AdvanceCount:      len(chapter.StorylineAdvances),
	}
	return brief
}

func compactStorylineAdvances(advances []models.StorylineAdvance, maxItems int) []models.StorylineAdvance {
	if maxItems <= 0 || len(advances) == 0 {
		return nil
	}
	if len(advances) > maxItems {
		advances = advances[:maxItems]
	}
	result := make([]models.StorylineAdvance, 0, len(advances))
	for _, advance := range advances {
		result = append(result, models.StorylineAdvance{
			StorylineName: clipToolString(advance.StorylineName, 80),
			Stage:         clipToolString(advance.Stage, 50),
			Change:        clipToolString(advance.Change, 180),
			Consequence:   clipToolString(advance.Consequence, 180),
			Pressure:      clipToolString(advance.Pressure, 180),
		})
	}
	return result
}

func makeRecapBrief(value *models.ChapterRecap) interface{} {
	if value == nil {
		return nil
	}
	return models.ChapterRecap{
		ChapterID:       value.ChapterID,
		Title:           clipToolString(value.Title, 80),
		Location:        clipToolString(value.Location, 120),
		Time:            clipToolString(value.Time, 120),
		Present:         clipToolStrings(value.Present, 12, 50),
		PlotBeats:       clipToolStrings(value.PlotBeats, 5, 160),
		Decisions:       clipToolStrings(value.Decisions, 4, 140),
		Reveals:         clipToolStrings(value.Reveals, 4, 140),
		Unresolved:      clipToolStrings(value.Unresolved, 4, 140),
		Promises:        clipToolStrings(value.Promises, 4, 140),
		Items:           clipToolStrings(value.Items, 5, 140),
		Status:          clipToolStrings(value.Status, 5, 140),
		LastLine:        clipToolString(value.LastLine, 220),
		Cliffhanger:     clipToolString(value.Cliffhanger, 180),
		NextOpeningHint: clipToolString(value.NextOpeningHint, 220),
	}
}

func recapUsableForChapter(value *models.ChapterRecap, chapter *models.Chapter) (bool, string) {
	if value == nil || chapter == nil {
		return false, ""
	}
	recapID := strings.TrimSpace(value.ChapterID)
	chapterID := strings.TrimSpace(chapter.ID)
	if recapID != "" && chapterID != "" && !idMatches(recapID, chapterID) {
		return false, fmt.Sprintf("recap chapter_id %q does not match outline chapter %q", recapID, chapterID)
	}
	recapTitle := strings.TrimSpace(value.Title)
	chapterTitle := strings.TrimSpace(chapter.Title)
	if recapTitle != "" && chapterTitle != "" && !strings.EqualFold(recapTitle, chapterTitle) {
		return false, fmt.Sprintf("recap title %q does not match outline title %q for %s", recapTitle, chapterTitle, chapterID)
	}
	return true, ""
}

func loadUsableToolContextRecap(root string, store *recap.Store, chapter *models.Chapter) (*models.ChapterRecap, string) {
	if store == nil || chapter == nil {
		return nil, ""
	}
	loaded, err := store.Load(chapter.ID)
	if err != nil {
		return nil, ""
	}
	if ok, warning := recapUsableForChapter(loaded, chapter); !ok {
		return nil, warning
	}
	if ok, warning := recapFreshForChapterMarkdown(root, chapter.ID); !ok {
		return nil, warning
	}
	return loaded, ""
}

func recapFreshForChapterMarkdown(root, chapterID string) (bool, string) {
	root = strings.TrimSpace(root)
	chapterID = strings.TrimSpace(chapterID)
	if root == "" || chapterID == "" {
		return true, ""
	}
	recapPath := filepath.Join(root, "story", "recaps", chapterID+".json")
	chapterPath := firstExistingPath(candidateChapterMarkdownPaths(root, chapterID))
	if chapterPath == "" {
		return true, ""
	}
	recapInfo, recapErr := os.Stat(recapPath)
	chapterInfo, chapterErr := os.Stat(chapterPath)
	if recapErr != nil || chapterErr != nil {
		return true, ""
	}
	if recapInfo.ModTime().Before(chapterInfo.ModTime()) {
		return false, fmt.Sprintf("recap %s is older than chapter markdown %s", recapPath, chapterPath)
	}
	return true, ""
}

func makeChapterBrief(chapter models.Chapter, indexOnly bool) chapterBrief {
	brief := chapterBrief{
		ID:           chapter.ID,
		Title:        chapter.Title,
		Summary:      clipToolString(chapter.Summary, 500),
		Characters:   chapter.Characters,
		Location:     chapter.Location,
		StateChange:  clipToolString(chapter.StateChange, 180),
		EventCount:   len(chapter.Events),
		AdvanceCount: len(chapter.StorylineAdvances),
	}
	if indexOnly {
		return brief
	}
	brief.OpeningBeat = clipToolString(chapter.OpeningBeat, 180)
	brief.ClosingBeat = clipToolString(chapter.ClosingBeat, 180)
	brief.Conflict = clipToolString(chapter.Conflict, 220)
	brief.Pacing = clipToolString(chapter.Pacing, 180)
	brief.Timeline = chapter.Timeline
	brief.StateAnchor = chapter.StateAnchor
	brief.StorylineAdvances = chapter.StorylineAdvances
	brief.ChapterPayoff = chapter.ChapterPayoff
	for _, event := range chapter.Events {
		brief.Events = append(brief.Events, makeEventBrief(event))
	}
	return brief
}

func makeEventBrief(event models.Event) eventBrief {
	return eventBrief{
		Type:       event.Type,
		Characters: event.Characters,
		Subject:    clipToolString(event.Subject, 120),
		Change:     clipToolString(event.Change, 140),
		Details:    clipToolString(event.Details, 180),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
		Context:    clipToolString(event.Context, 140),
		Result:     clipToolString(event.Result, 160),
	}
}

func makeEventIndexBrief(event models.Event) eventBrief {
	return eventBrief{
		Type:       event.Type,
		Subject:    clipToolString(event.Subject, 80),
		Change:     clipToolString(event.Change, 80),
		Actor:      event.GetActor(),
		Action:     event.GetAction(),
		Target:     event.GetTarget(),
		TargetType: event.GetTargetType(),
	}
}

func makeEventHitBrief(hit eventHit, indexOnly bool) eventHitBrief {
	brief := eventHitBrief{
		ChapterID:    hit.ChapterID,
		ChapterTitle: hit.ChapterTitle,
		EventIndex:   hit.EventIndex,
		Reasons:      hit.Reasons,
	}
	if indexOnly {
		brief.Event = makeEventIndexBrief(hit.Event)
		return brief
	}
	brief.Path = &hit.Path
	brief.Event = makeEventBrief(hit.Event)
	return brief
}

func briefChapterResults(results interface{}, indexOnly bool) interface{} {
	values, ok := results.([]chapterHit)
	if !ok {
		return results
	}
	out := make([]map[string]interface{}, 0, len(values))
	for _, hit := range values {
		item := map[string]interface{}{
			"id":      hit.ID,
			"title":   hit.Title,
			"summary": hit.Summary,
			"path":    hit.Path,
		}
		if len(hit.Reasons) > 0 {
			item["reasons"] = hit.Reasons
		}
		if hit.Outline != nil {
			brief := makeChapterBrief(*hit.Outline, indexOnly)
			item["outline"] = brief
		}
		if hit.ContentPath != "" && !indexOnly {
			item["content_path"] = hit.ContentPath
		}
		out = append(out, item)
	}
	return out
}

func clipToolStrings(values []string, maxItems, maxRunes int) []string {
	if len(values) == 0 {
		return nil
	}
	if maxItems > 0 && len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if clipped := clipToolString(value, maxRunes); clipped != "" {
			out = append(out, clipped)
		}
	}
	return out
}

func firstNStrings(values []string, maxItems int) []string {
	if maxItems > 0 && len(values) > maxItems {
		return values[:maxItems]
	}
	return values
}

func loadChapterExcerptForTool(root, chapterID string) (map[string]string, []string) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(chapterID) == "" {
		return nil, []string{"project root or chapter id missing; chapter excerpt unavailable"}
	}
	candidates := []string{
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterID)),
		filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", extractChapterNumber(chapterID))),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		return map[string]string{
			"content_path": path,
			"opening":      clipToolString(content, 320),
			"closing":      clipToolTailString(content, 420),
			"last_line":    clipToolString(lastNonEmptyLine(content), 220),
		}, nil
	}
	return nil, []string{fmt.Sprintf("chapter manuscript not found for %s", chapterID)}
}

func clipToolString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func clipToolTailString(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return "..." + string(runes[len(runes)-maxRunes:])
}

func lastNonEmptyLine(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func applyToolLimit(resp *toolResponse, limit int) {
	if resp == nil || limit <= 0 || resp.Results == nil {
		return
	}
	data, err := json.Marshal(resp.Results)
	if err != nil {
		return
	}
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil || len(values) <= limit {
		return
	}
	resp.Results = values[:limit]
	resp.Warnings = append(resp.Warnings, fmt.Sprintf("results truncated to --limit=%d", limit))
}

func writeToolJSON(cmd *cobra.Command, resp toolResponse) error {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func init() {
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.Type, "type", "", "Section-specific query type")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.ID, "id", "", "ID for part, volume, or chapter queries")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.Name, "name", "", "Name or substring to search")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.EntityType, "entity-type", "", "Entity type for ref queries: character, item, location, storyline")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.ChapterID, "chapter-id", "", "Limit event queries to a chapter ID")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.VolumeID, "volume-id", "", "Limit event queries to a volume ID")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.View, "view", "brief", "Output detail level: brief, index, or full")
	toolQueryCmd.Flags().StringVar(&toolQueryFlags.Fields, "fields", "", "Comma-separated JSON fields to return from each result, e.g. storyline_advances,chapter_payoff,conflict")
	toolQueryCmd.Flags().BoolVar(&toolQueryFlags.IncludeContent, "content", false, "Include chapter text content when available")
	toolQueryCmd.Flags().IntVar(&toolQueryFlags.Limit, "limit", 0, "Maximum number of results to return")
	toolCheckCmd.Flags().StringVar(&toolCheckFlags.Target, "target", "outline", "Check target: outline, setup, craft, chapter, or recap")
	toolCheckCmd.Flags().StringVar(&toolCheckFlags.Scope, "scope", "all", "Check scope: outline all/volume/chapter, craft character/item/location/organization, chapter chapter, or recap chapter")
	toolCheckCmd.Flags().StringVar(&toolCheckFlags.ID, "id", "", "Volume/chapter ID, craft element name, final chapter ID, or recap chapter ID for scoped checks")
	toolCheckCmd.Flags().StringVar(&toolCheckFlags.MinPriority, "min-priority", "", "Only return issues at or above priority: low, medium, high, critical")
	toolCheckCmd.Flags().StringVar(&toolCheckFlags.Category, "category", "", "Only return issues in comma-separated categories, e.g. logic,plot,structure")
	toolCheckCmd.Flags().IntVar(&toolCheckFlags.MaxIssues, "max-issues", 0, "Maximum number of issues to return without changing the summary")
	toolCheckCmd.Flags().IntVar(&toolCheckFlags.TargetWords, "target-words", 0, "Override target narrative units for final chapter length checks")
	toolPatchCmd.Flags().StringVar(&toolPatchFlags.Target, "target", "chapter", "Patch target: outline chapter/volume or craft character/item/location/organization")
	toolPatchCmd.Flags().StringVar(&toolPatchFlags.ID, "id", "", "Target outline ID, final chapter ID, recap chapter ID, or craft element name/key")
	toolPatchCmd.Flags().StringVar(&toolPatchFlags.PatchJSON, "patch-json", "", "Patch JSON string; stdin is used when omitted")
	toolPatchCmd.Flags().StringVar(&toolPatchFlags.PatchBuffer, "patch-buffer", "", "Read patch content from a novelgen-managed agent patch buffer")
	toolPatchCmd.Flags().StringVar(&toolPatchFlags.Task, "task", "", "Use a validated project patch task instead of stdin or --patch-json")
	toolPatchCmd.Flags().BoolVar(&toolPatchFlags.Apply, "apply", false, "Apply the patch and save project files")
	toolPatchCmd.Flags().BoolVar(&toolPatchFlags.DryRun, "dry-run", false, "Force dry-run mode even when --apply is present")
	toolPatchCmd.Flags().BoolVar(&toolPatchFlags.RefreshDerived, "refresh-derived", false, "For chapter --apply, regenerate derived RPG DSL and return a post-refresh all check")
	toolPatchCmd.Flags().IntVar(&toolPatchFlags.TargetWords, "target-words", 0, "Override target narrative units for chapter patch length checks")
	toolPatchBufferCmd.Flags().StringVar(&toolPatchBufferFlags.ID, "id", "", "Patch buffer id")
	toolPatchBufferCmd.Flags().StringVar(&toolPatchBufferFlags.Text, "text", "", "Text chunk to append; stdin is used when omitted")
	toolPatchBufferCmd.Flags().BoolVar(&toolPatchBufferFlags.Stdin, "stdin", false, "Read text chunk from stdin explicitly")
	toolPatchBufferCmd.Flags().IntVar(&toolPatchBufferFlags.MaxChars, "max-chars", 400, "Preview character limit for show")
	toolRefreshCmd.Flags().StringVar(&toolRefreshFlags.ID, "id", "", "Target chapter ID for scoped refresh")
	toolRefreshCmd.Flags().IntVar(&toolRefreshFlags.BatchSize, "batch-size", 10, "Chapter markdown batch size for AI -> RPG DSL conversion")
	toolCmd.AddCommand(toolQueryCmd)
	toolCmd.AddCommand(toolCheckCmd)
	toolCmd.AddCommand(toolPatchCmd)
	toolCmd.AddCommand(toolPatchBufferCmd)
	toolCmd.AddCommand(toolRefreshCmd)
	RegisterCommand(func() *cobra.Command {
		return toolCmd
	})
}
