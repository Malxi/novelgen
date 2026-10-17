// The craft-* context bundles used before generating or repairing a single craft element.

package toolplane

import (
	"fmt"
	"novelgen/internal/contract/toolcmd"
	"strings"
)

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
		toolcmd.QueryStorySetup(toolcmd.QueryOptions{Type: "search", Name: name, View: "brief"}),
		toolcmd.QueryCraft(toolcmd.QueryOptions{Type: target, Name: name, View: "brief"}),
	}
	if target == "character" || target == "item" || target == "location" {
		detailQueries = append(detailQueries,
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "refs", EntityType: target, Name: name, View: "brief"}),
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", EntityType: target, Name: name, View: "brief"}),
		)
	} else {
		detailQueries = append(detailQueries,
			toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "events", Name: name, Fields: "result,details,target,target_type,actor,action", View: "brief", Limit: 12}),
		)
	}
	return map[string]interface{}{
		"detail_queries":              detailQueries,
		"chapter_detail_query":        "novelgen tool query outline --type chapter --id <chapter_id> --view brief",
		"chapter_events_query":        "novelgen tool query outline --type events --chapter-id <chapter_id> --view brief",
		"schema_check_query":          toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: name}),
		"patch_query":                 toolcmd.PatchCraft(target, name),
		"patch_shape":                 map[string]interface{}{"<field_to_change>": "<new value>"},
		"post_patch_check_query":      toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: name}),
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
			Command: toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: name}),
			Purpose: "Check whether the current or candidate craft object satisfies the typed schema.",
		},
		{
			Step:    3,
			Action:  "patch_dry_run",
			Command: toolcmd.PatchCraft(target, name),
			Purpose: "Validate a minimal craft patch without writing.",
			When:    "The workflow allows craft patching or you need to test a candidate object.",
		},
		{
			Step:    4,
			Action:  "post_patch_check",
			Command: toolcmd.Check(toolcmd.CheckOptions{Kind: "schema", Target: "craft", Scope: target, ID: name}),
			Purpose: "Run after patch apply to catch schema, text-safety, and RPG metadata regressions.",
		},
	}
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
