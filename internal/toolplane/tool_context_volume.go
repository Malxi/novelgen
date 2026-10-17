// The outline-volume context bundle used before improving one volume.

package toolplane

import (
	"fmt"
	"novelgen/internal/toolcmd"
	"strings"
)

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
		return ToolResponse{OK: false, Section: "context", Count: 0, Results: nil, Warnings: []string{fmt.Sprintf("outline volume not found: id=%q name=%q", id, name)}}
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
				toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: target.Volume.ID, View: "brief"}),
				toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: target.Volume.ID, MinPriority: "medium", MaxIssues: 12}),
				toolcmd.PatchOutlineVolume(target.Volume.ID),
			},
			"chapter_detail_query":        "novelgen tool query outline --type chapter --id <chapter_id> --view brief",
			"chapter_field_query":         "novelgen tool query outline --type chapter --id <chapter_id> --fields storyline_advances,chapter_payoff,conflict --view brief",
			"chapter_events_query":        "novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
			"chapter_event_field_query":   "novelgen tool query outline --type events --chapter-id <chapter_id> --fields result,details,target,target_type --view brief",
			"setup_search_query":          "novelgen tool query story-setup --type search --name <keyword> --view brief",
			"entity_reference_query":      "novelgen tool query outline --type refs --entity-type character|item|location --name <name> --view brief",
			"patch_query":                 toolcmd.PatchOutlineVolume(target.Volume.ID),
			"patch_shape":                 map[string]interface{}{"summary": "<optional new summary>", "changed_chapters": []map[string]string{{"id": "<chapter_id>"}}},
			"post_patch_check_query":      toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: target.Volume.ID, MinPriority: "low", MaxIssues: 12}),
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
			Command: toolcmd.QueryContext(toolcmd.QueryOptions{Type: "outline-volume", ID: volumeID, View: "brief"}),
			Purpose: "Fetch the focused volume bundle before generating, improving, or patching; index view is only the route map and counts.",
		},
		{
			Step:    2,
			Action:  "focused_check",
			Command: toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: volumeID, MinPriority: "medium", MaxIssues: 12}),
			Purpose: "Check the volume before deciding which chapters or contracts need repair.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: toolcmd.PatchOutlineVolume(volumeID),
			Purpose: "Validate a minimal volume patch; chapter repairs must use changed_chapters under this parent volume.",
			When:    "The workflow allows outline patching or you need to test a candidate volume repair.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: volumeID, MinPriority: "low", MaxIssues: 12}),
			Purpose: "Run after patch apply to catch medium+ regressions in the patched volume.",
		},
	}
}
