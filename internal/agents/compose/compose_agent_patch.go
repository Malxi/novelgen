// Code in this file is part of the ComposeAgent implementation.
// Split from compose_agent.go; keep the stage concern in one file.

package compose

import (
	"novelgen/internal/toolcmd"
	"strings"

	"novelgen/internal/models"
)

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

func composeGlobalRepairRequiredPostPatchChecks(review models.ReviewResult) []string {
	for _, suggestion := range review.Suggestions {
		category := composeAgentSDKCheckCategory(suggestion.Category)
		if category == "" {
			continue
		}
		targetID := strings.TrimSpace(suggestion.TargetID)
		if category == "faction_tier" && targetID != "" && !strings.EqualFold(targetID, "global") {
			return []string{toolcmd.Check(toolcmd.CheckOptions{Target: "setup", Category: category, MinPriority: "low", MaxIssues: 12})}
		}
		return []string{toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "all", Category: category, MinPriority: "low", MaxIssues: 12})}
	}
	return nil
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
