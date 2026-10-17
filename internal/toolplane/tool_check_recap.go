// The recap target check: saved story/recaps/<chapter_id>.json continuity anchors.

package toolplane

import (
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/store"
)

// RunToolRecapCheck is the string-root entry point for callers that hold a
// resolved project directory.
func RunToolRecapCheck(rootDir, kind, scope, id string) (*ToolCheckResult, error) {
	return runToolRecapCheckAt(store.At(rootDir), kind, scope, id)
}

func runToolRecapCheckAt(root *store.Root, kind, scope, id string) (*ToolCheckResult, error) {
	kind = NormalizeKey(kind)
	if kind == "" {
		kind = "all"
	}
	switch kind {
	case "quality", "all", "schema":
	default:
		return nil, fmt.Errorf("unsupported recap check kind %q", kind)
	}
	scope = normalizeRecapCheckScope(scope)
	id = strings.TrimSpace(id)
	if scope != "chapter" {
		return nil, fmt.Errorf("recap checks currently support only --scope chapter")
	}
	if id == "" {
		return nil, fmt.Errorf("--id is required for recap chapter check")
	}

	recapStore := recap.NewStoreAt(root)
	recapData, err := recapStore.Load(id)
	gate := quality.QualityGateResult{}
	if err != nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   id,
			TargetName: id,
			Issue:      fmt.Sprintf("recap file is not available: %v", err),
			Suggestion: fmt.Sprintf("Generate recap for chapter %s, then rerun tool check quality --target recap --scope chapter --id %q.", id, id),
			Priority:   models.PriorityHigh,
		})
		gate.Blocking = true
		return MakeToolCheckResult(kind, "recap", scope, id, gate), nil
	}
	var outlineChapter *models.Chapter
	if outline, err := root.LoadOutline(); err == nil && outline != nil {
		outlineChapter = outline.GetChapterByID(id)
	}
	return runToolRecapCheckForValue(kind, scope, id, recapData, outlineChapter), nil
}

func runToolRecapCheckForValue(kind, scope, id string, recapData *models.ChapterRecap, outlineChapter *models.Chapter) *ToolCheckResult {
	gate := quality.QualityGateResult{}
	validateRecapForToolCheck(&gate, recapData, id, outlineChapter)
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return MakeToolCheckResult(kind, "recap", scope, id, gate)
}

func normalizeRecapCheckScope(scope string) string {
	switch NormalizeKey(scope) {
	case "", "all", "chapter", "chap":
		return "chapter"
	default:
		return NormalizeKey(scope)
	}
}

func validateRecapForToolCheck(gate *quality.QualityGateResult, recapData *models.ChapterRecap, expectedID string, outlineChapter *models.Chapter) {
	if gate == nil {
		return
	}
	if recapData == nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: expectedID,
			Issue:      "recap is empty",
			Suggestion: "Regenerate the recap from the final chapter text.",
			Priority:   models.PriorityHigh,
		})
		return
	}
	if strings.TrimSpace(recapData.ChapterID) != "" && strings.TrimSpace(expectedID) != "" && recapData.ChapterID != expectedID {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: expectedID,
			Issue:      fmt.Sprintf("recap chapter_id %q does not match requested chapter %q", recapData.ChapterID, expectedID),
			Suggestion: "Regenerate or patch the recap so chapter_id matches the checked chapter.",
			Priority:   models.PriorityHigh,
		})
	}
	if outlineChapter != nil && strings.TrimSpace(outlineChapter.Title) != "" && strings.TrimSpace(recapData.Title) != "" && strings.TrimSpace(recapData.Title) != strings.TrimSpace(outlineChapter.Title) {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "recap",
			TargetID:   expectedID,
			TargetName: recapData.Title,
			Issue:      fmt.Sprintf("recap title %q does not match outline title %q", recapData.Title, outlineChapter.Title),
			Suggestion: "Regenerate the recap from the current final chapter and outline title so recap.title matches the chapter being checked.",
			Priority:   models.PriorityMedium,
		})
	}
	if ok, reasons := recap.ValidateMinimal(recapData); !ok {
		for _, reason := range reasons {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "recap",
				TargetID:   expectedID,
				TargetName: recapData.Title,
				Issue:      reason,
				Suggestion: "Regenerate the recap from final chapter text and preserve location, present characters, last_line, and next_opening_hint.",
				Priority:   models.PriorityHigh,
			})
		}
	}
	if ok, reasons := recap.ValidateConsistency(recapData); !ok {
		for _, reason := range reasons {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "recap",
				TargetID:   expectedID,
				TargetName: recapData.Title,
				Issue:      reason,
				Suggestion: "Regenerate recap with a concise next_opening_hint that visibly continues from last_line.",
				Priority:   models.PriorityMedium,
			})
		}
	}
}

func normalizeRecapPatchIdentity(value *models.ChapterRecap, id string, outlineChapter *models.Chapter, patchBytes []byte) []string {
	if value == nil {
		return nil
	}
	var patch map[string]interface{}
	_ = json.Unmarshal(patchBytes, &patch)
	warnings := []string{}
	if raw, ok := patch["chapter_id"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" && strings.TrimSpace(fmt.Sprint(raw)) != id {
		warnings = append(warnings, fmt.Sprintf("ignored patch chapter_id %q; recap identity is fixed to %q", fmt.Sprint(raw), id))
	}
	value.ChapterID = id
	if outlineChapter != nil && strings.TrimSpace(outlineChapter.Title) != "" {
		if raw, ok := patch["title"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" && strings.TrimSpace(fmt.Sprint(raw)) != strings.TrimSpace(outlineChapter.Title) {
			warnings = append(warnings, fmt.Sprintf("ignored patch title %q; recap title is fixed to outline title %q", fmt.Sprint(raw), outlineChapter.Title))
		}
		value.Title = outlineChapter.Title
	}
	return warnings
}
