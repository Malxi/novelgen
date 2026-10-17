// The setup and outline target checks and the scope filter they share. Both run the deterministic
// internal/fiction/domain/quality gates over saved project state.

package check

import (
	"fmt"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/quality"
	"novelgen/internal/platform/store"
)

func RunToolSetupCheck(kind string, setup *models.StorySetup) (*ToolCheckResult, error) {
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		gate = quality.RunSetupQualityGate(setup)
	case "simulation":
		gate = quality.RunSetupSimulationGate(setup)
	case "all":
		gate = quality.RunSetupQualityGate(setup)
		sim := quality.RunSetupSimulationGate(setup)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	default:
		return nil, fmt.Errorf("unsupported setup check kind %q", kind)
	}
	return MakeToolCheckResult(kind, "setup", "all", "", gate), nil
}

// RunToolOutlineCheck is the model-only compatibility form. It has no project
// root, so simulation runs without craft elements; callers that can reach the
// project should use RunToolOutlineCheckAt.
func RunToolOutlineCheck(kind string, setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	return RunToolOutlineCheckAt(nil, kind, setup, outline, scope, id)
}

// RunToolOutlineCheckAt runs the outline check against an explicit storage root.
func RunToolOutlineCheckAt(root *store.Root, kind string, setup *models.StorySetup, outline *models.Outline, scope, id string) (*ToolCheckResult, error) {
	scope = NormalizeScope(scope)
	id = strings.TrimSpace(id)
	if scope != "all" && id == "" {
		return nil, fmt.Errorf("--id is required for scope %q", scope)
	}
	scoped, err := scopedOutline(outline, scope, id)
	if err != nil {
		return nil, err
	}
	var gate quality.QualityGateResult
	switch kind {
	case "quality":
		if scope == "all" {
			gate = quality.RunOutlineQualityGate(setup, scoped)
		} else {
			gate = quality.RunScopedOutlineQualityGate(setup, outline)
		}
	case "simulation":
		gate = quality.RunOutlineSimulationGate(root, setup, outline)
	case "all":
		if scope == "all" {
			gate = quality.RunOutlineQualityGate(setup, scoped)
		} else {
			gate = quality.RunScopedOutlineQualityGate(setup, outline)
		}
		sim := quality.RunOutlineSimulationGate(root, setup, outline)
		gate.Add(sim.Suggestions...)
		gate.Dedup()
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	if scope != "all" {
		gate.Suggestions = FilterToolScopedSuggestions(gate.Suggestions, scoped, scope, id)
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	}
	return MakeToolCheckResult(kind, "outline", scope, id, gate), nil
}

func FilterToolScopedSuggestions(suggestions []models.ReviewSuggestion, outline *models.Outline, scope, id string) []models.ReviewSuggestion {
	if len(suggestions) == 0 {
		return nil
	}
	allowed := toolScopedTargetIDs(outline, scope, id)
	var filtered []models.ReviewSuggestion
	for _, suggestion := range suggestions {
		targetID := strings.TrimSpace(suggestion.TargetID)
		if targetID == "" {
			continue
		}
		if allowed[targetID] || allowed[strings.ToLower(targetID)] {
			filtered = append(filtered, suggestion)
			continue
		}
		if scope == "volume" && strings.HasPrefix(targetID, id+"-C") {
			filtered = append(filtered, suggestion)
		}
	}
	return filtered
}

func toolScopedTargetIDs(outline *models.Outline, scope, id string) map[string]bool {
	allowed := map[string]bool{
		id:                  true,
		strings.ToLower(id): true,
	}
	if outline == nil {
		return allowed
	}
	switch scope {
	case "volume":
		volume := outline.GetVolumeByID(id)
		if volume == nil && len(outline.Parts) == 1 && len(outline.Parts[0].Volumes) == 1 {
			volume = &outline.Parts[0].Volumes[0]
		}
		if volume != nil {
			for _, chapter := range volume.Chapters {
				if strings.TrimSpace(chapter.ID) != "" {
					allowed[chapter.ID] = true
					allowed[strings.ToLower(chapter.ID)] = true
				}
			}
		}
		allowed["payoff_contract"] = true
	case "chapter":
		allowed["chapter"] = true
	}
	return allowed
}

func NormalizeScope(scope string) string {
	switch NormalizeKey(scope) {
	case "", "all", "outline":
		return "all"
	case "vol", "volume":
		return "volume"
	case "chap", "chapter":
		return "chapter"
	default:
		return NormalizeKey(scope)
	}
}

func scopedOutline(outline *models.Outline, scope, id string) (*models.Outline, error) {
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	if scope == "all" {
		return CloneOutline(outline), nil
	}
	for _, part := range outline.Parts {
		for _, volume := range part.Volumes {
			if scope == "volume" && volume.ID == id {
				partCopy := part
				volumeCopy := volume
				partCopy.Volumes = []models.Volume{volumeCopy}
				return &models.Outline{Parts: []models.Part{partCopy}}, nil
			}
			if scope == "chapter" {
				for _, chapter := range volume.Chapters {
					if chapter.ID == id {
						partCopy := part
						volumeCopy := volume
						partCopy.Volumes = []models.Volume{volumeCopy}
						return &models.Outline{Parts: []models.Part{partCopy}}, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("%s %q not found in outline", scope, id)
}
