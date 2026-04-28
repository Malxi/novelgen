package models

import "strings"

// OutlineNormalizationReport records deterministic cleanups applied to an outline.
type OutlineNormalizationReport struct {
	Changes []OutlineNormalizationChange `json:"changes"`
}

// OutlineNormalizationChange describes one deterministic outline cleanup.
type OutlineNormalizationChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Value  string `json:"value"`
}

// Changed reports whether normalization modified the outline.
func (r OutlineNormalizationReport) Changed() bool {
	return len(r.Changes) > 0
}

// NormalizeOutline applies deterministic, schema-preserving repairs that do not
// require creative judgment. It keeps chapter character lists in sync with
// event and scene appearances so validators and downstream agents see one
// canonical appearance list.
func NormalizeOutline(outline *Outline) OutlineNormalizationReport {
	report := OutlineNormalizationReport{Changes: []OutlineNormalizationChange{}}
	if outline == nil {
		return report
	}

	for pi := range outline.Parts {
		for vi := range outline.Parts[pi].Volumes {
			for ci := range outline.Parts[pi].Volumes[vi].Chapters {
				chapter := &outline.Parts[pi].Volumes[vi].Chapters[ci]
				seen := make(map[string]bool, len(chapter.Characters))

				characters := chapter.Characters[:0]
				for _, char := range chapter.Characters {
					char = strings.TrimSpace(char)
					if char == "" || seen[char] {
						continue
					}
					seen[char] = true
					characters = append(characters, char)
				}
				chapter.Characters = characters

				for _, event := range chapter.Events {
					for _, char := range event.Characters {
						addChapterCharacter(chapter, seen, char, "sync_event_character", &report)
					}
				}

				for _, scene := range chapter.Scenes {
					for _, char := range scene.Characters {
						addChapterCharacter(chapter, seen, char, "sync_scene_character", &report)
					}
				}
			}
		}
	}

	return report
}

func addChapterCharacter(chapter *Chapter, seen map[string]bool, char string, action string, report *OutlineNormalizationReport) {
	char = strings.TrimSpace(char)
	if char == "" || seen[char] {
		return
	}

	chapter.Characters = append(chapter.Characters, char)
	seen[char] = true
	report.Changes = append(report.Changes, OutlineNormalizationChange{
		Path:   chapter.ID + ".characters",
		Action: action,
		Value:  char,
	})
}
