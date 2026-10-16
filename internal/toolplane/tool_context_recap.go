// The recap-repair context bundle used by recap extraction and repair workflows.

package toolplane

import (
	"fmt"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/toolcmd"
	"strings"
)

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

	check, err := runToolRecapCheckAt(ctx.Root, "quality", "chapter", target.Chapter.ID)
	if err != nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: []string{err.Error()}}
	}
	_ = ApplyToolCheckIssueFilters(check, "low", "recap", 8)

	var current *models.ChapterRecap
	if ctx.Root != nil {
		if loaded, err := recap.NewStoreAt(ctx.Root).Load(target.Chapter.ID); err == nil {
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
			"focused_check_query":       toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: target.Chapter.ID, MinPriority: "low", MaxIssues: 8}),
			"chapter_outline_query":     toolcmd.QueryOutline(toolcmd.QueryOptions{Type: "chapter", ID: target.Chapter.ID, View: "brief"}),
			"chapter_content_query":     toolcmd.QueryChapter(toolcmd.QueryOptions{ID: target.Chapter.ID, Content: true, View: "brief"}),
			"patch_query":               toolcmd.PatchRecap(target.Chapter.ID),
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
		"patch_query":                  toolcmd.PatchRecap(chapterID),
		"patch_shape":                  map[string]interface{}{"location": "<scene anchor>", "present": []string{"<character>"}, "last_line": "<final visible line>", "next_opening_hint": "<continuation from last_line>"},
		"post_save_check_query":        toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: chapterID, MinPriority: "low", MaxIssues: 8}),
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
			Command: toolcmd.PatchRecap(chapterID),
			Purpose: "Validate a minimal recap patch without writing.",
			When:    "The workflow allows recap patching or you need to test a candidate recap.",
		},
		{
			Step:    3,
			Action:  "post_save_check",
			Command: toolcmd.Check(toolcmd.CheckOptions{Kind: "quality", Target: "recap", Scope: "chapter", ID: chapterID, MinPriority: "low", MaxIssues: 8}),
			Purpose: "Confirm title, last_line, next_opening_hint, and minimum recap fields after Go saves or patch apply succeeds.",
		},
	}
}
