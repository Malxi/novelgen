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

				normalizeStateAnchor(chapter, &report)
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

func normalizeStateAnchor(chapter *Chapter, report *OutlineNormalizationReport) {
	normalizeStateAnchorAllies(chapter, report)
	normalizeStateAnchorInjuries(chapter, report)
}

func normalizeStateAnchorAllies(chapter *Chapter, report *OutlineNormalizationReport) {
	seen := make(map[string]bool, len(chapter.StateAnchor.Allies))
	allies := chapter.StateAnchor.Allies[:0]
	for _, ally := range chapter.StateAnchor.Allies {
		original := strings.TrimSpace(ally)
		if original == "" {
			continue
		}

		canonical, _, ok := splitTrailingQualifier(original)
		if ok {
			appendStateAnchorNote(&chapter.StateAnchor, "盟友备注："+original)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   chapter.ID + ".state_anchor.allies",
				Action: "canonicalize_ally_name",
				Value:  original + " -> " + canonical,
			})
		} else {
			canonical = original
		}

		if canonical == "" || seen[canonical] {
			continue
		}
		seen[canonical] = true
		allies = append(allies, canonical)
	}
	chapter.StateAnchor.Allies = allies
}

func normalizeStateAnchorInjuries(chapter *Chapter, report *OutlineNormalizationReport) {
	seen := make(map[string]bool, len(chapter.StateAnchor.Injuries))
	injuries := chapter.StateAnchor.Injuries[:0]
	for _, injury := range chapter.StateAnchor.Injuries {
		original := strings.TrimSpace(injury)
		if original == "" {
			continue
		}

		canonical := original
		qualifier := ""
		if base, q, ok := splitTrailingQualifier(original); ok {
			canonical = base
			qualifier = q
			appendStateAnchorNote(&chapter.StateAnchor, "伤势备注："+original)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   chapter.ID + ".state_anchor.injuries",
				Action: "canonicalize_injury",
				Value:  original + " -> " + canonical,
			})
		}

		if injuryStatusResolved(original) || injuryStatusResolved(qualifier) {
			appendStateAnchorNote(&chapter.StateAnchor, "伤势已恢复："+original)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   chapter.ID + ".state_anchor.injuries",
				Action: "remove_resolved_injury",
				Value:  original,
			})
			continue
		}

		if canonical == "" || seen[canonical] {
			continue
		}
		seen[canonical] = true
		injuries = append(injuries, canonical)
	}
	chapter.StateAnchor.Injuries = injuries
}

func splitTrailingQualifier(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "）") {
		start := strings.LastIndex(value, "（")
		if start > 0 {
			base := strings.TrimSpace(value[:start])
			qualifier := strings.TrimSpace(strings.TrimSuffix(value[start+len("（"):], "）"))
			return base, qualifier, base != "" && qualifier != ""
		}
	}
	if strings.HasSuffix(value, ")") {
		start := strings.LastIndex(value, "(")
		if start > 0 {
			base := strings.TrimSpace(value[:start])
			qualifier := strings.TrimSpace(strings.TrimSuffix(value[start+1:], ")"))
			return base, qualifier, base != "" && qualifier != ""
		}
	}
	return value, "", false
}

func injuryStatusResolved(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	resolvedMarkers := []string{"已愈合", "已恢复", "已痊愈", "已康复", "基本愈合", "healed", "recovered"}
	for _, marker := range resolvedMarkers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func appendStateAnchorNote(anchor *StateAnchor, note string) {
	note = strings.TrimSpace(note)
	if note == "" || strings.Contains(anchor.Notes, note) {
		return
	}
	if strings.TrimSpace(anchor.Notes) == "" {
		anchor.Notes = note
		return
	}
	anchor.Notes = strings.TrimSpace(anchor.Notes) + "；" + note
}
