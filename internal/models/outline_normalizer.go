package models

import (
	"fmt"
	"strings"
)

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
			normalizeVolumeIdentity(&outline.Parts[pi], vi, &report)
			for ci := range outline.Parts[pi].Volumes[vi].Chapters {
				chapter := &outline.Parts[pi].Volumes[vi].Chapters[ci]
				normalizeChapterIdentity(&outline.Parts[pi].Volumes[vi], ci, &report)
				normalizeChapterScenes(chapter, &report)
				normalizeChapterTimeline(chapter, ci, &report)
				normalizeChapterEvents(chapter, &report)
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

func normalizeVolumeIdentity(part *Part, volumeIndex int, report *OutlineNormalizationReport) {
	if part == nil || volumeIndex < 0 || volumeIndex >= len(part.Volumes) {
		return
	}
	partID := strings.TrimSpace(part.ID)
	if partID == "" {
		partID = fmt.Sprintf("P%d", 1)
	}
	expectedID := fmt.Sprintf("%s-V%d", partID, volumeIndex+1)
	volume := &part.Volumes[volumeIndex]
	currentID := strings.TrimSpace(volume.ID)
	if currentID == expectedID {
		return
	}
	if currentID == "" || strings.EqualFold(currentID, expectedID) || isLegacyVolumeID(currentID) {
		volume.ID = expectedID
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   fmt.Sprintf("%s.volumes[%d].id", partID, volumeIndex),
			Action: "canonicalize_volume_id",
			Value:  currentID + " -> " + expectedID,
		})
	}
}

func normalizeChapterIdentity(volume *Volume, chapterIndex int, report *OutlineNormalizationReport) {
	if volume == nil || chapterIndex < 0 || chapterIndex >= len(volume.Chapters) {
		return
	}
	volumeID := strings.TrimSpace(volume.ID)
	if volumeID == "" {
		return
	}
	expectedID := fmt.Sprintf("%s-C%d", volumeID, chapterIndex+1)
	chapter := &volume.Chapters[chapterIndex]
	currentID := strings.TrimSpace(chapter.ID)
	if currentID == expectedID {
		return
	}
	if currentID == "" || strings.EqualFold(currentID, expectedID) {
		chapter.ID = expectedID
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   fmt.Sprintf("%s.chapters[%d].id", volumeID, chapterIndex),
			Action: "canonicalize_chapter_id",
			Value:  currentID + " -> " + expectedID,
		})
	}
}

func isLegacyVolumeID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if !strings.HasPrefix(id, "part") {
		return false
	}
	return strings.Contains(id, "-volume")
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
	normalizeStateAnchorLocation(chapter, report)
	normalizeStateAnchorAllies(chapter, report)
	normalizeStateAnchorInjuries(chapter, report)
}

func normalizeChapterScenes(chapter *Chapter, report *OutlineNormalizationReport) {
	if len(chapter.Scenes) == 0 {
		beats := chapter.GetBeats()
		if len(beats) == 0 {
			beats = fallbackChapterBeats(chapter)
		}
		if len(beats) > 0 {
			chapter.Scenes = synthesizeScenes(chapter, beats)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   chapter.ID + ".scenes",
				Action: "synthesize_scenes_from_beats",
				Value:  fmt.Sprintf("%d scene(s)", len(chapter.Scenes)),
			})
		}
	}

	for i := range chapter.Scenes {
		scene := &chapter.Scenes[i]
		if scene.Order != i+1 {
			scene.Order = i + 1
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   fmt.Sprintf("%s.scenes[%d].order", chapter.ID, i),
				Action: "fix_scene_order",
				Value:  fmt.Sprintf("%d", i+1),
			})
		}
		if strings.TrimSpace(scene.POV) == "" {
			scene.POV = defaultChapterPOV(chapter)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   fmt.Sprintf("%s.scenes[%d].pov", chapter.ID, i),
				Action: "fill_scene_pov",
				Value:  scene.POV,
			})
		}
		if strings.TrimSpace(scene.Goal) == "" {
			scene.Goal = sceneGoal(chapter, scene.Beats, i)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   fmt.Sprintf("%s.scenes[%d].goal", chapter.ID, i),
				Action: "fill_scene_goal",
				Value:  scene.Goal,
			})
		}
		if strings.TrimSpace(scene.Location) == "" {
			if location := strings.TrimSpace(chapter.Location); location != "" {
				scene.Location = location
				report.Changes = append(report.Changes, OutlineNormalizationChange{
					Path:   fmt.Sprintf("%s.scenes[%d].location", chapter.ID, i),
					Action: "fill_scene_location",
					Value:  scene.Location,
				})
			}
		}
		if len(scene.Characters) == 0 && len(chapter.Characters) > 0 {
			scene.Characters = append(scene.Characters, chapter.Characters...)
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   fmt.Sprintf("%s.scenes[%d].characters", chapter.ID, i),
				Action: "fill_scene_characters",
				Value:  strings.Join(scene.Characters, ", "),
			})
		}
		if len(scene.Beats) == 0 {
			if beats := fallbackChapterBeats(chapter); len(beats) > 0 {
				scene.Beats = beats
				report.Changes = append(report.Changes, OutlineNormalizationChange{
					Path:   fmt.Sprintf("%s.scenes[%d].beats", chapter.ID, i),
					Action: "fill_scene_beats",
					Value:  fmt.Sprintf("%d beat(s)", len(scene.Beats)),
				})
			}
		}
	}
}

func synthesizeScenes(chapter *Chapter, beats []string) []OutlineScene {
	beats = compactStrings(beats)
	if len(beats) == 0 {
		return nil
	}

	sceneCount := 2
	if len(beats) >= 5 {
		sceneCount = 3
	}
	if len(beats) == 1 {
		beats = append(beats, fallbackClosingBeat(chapter))
	}

	scenes := make([]OutlineScene, 0, sceneCount)
	for i := 0; i < sceneCount; i++ {
		start := i * len(beats) / sceneCount
		end := (i + 1) * len(beats) / sceneCount
		if end <= start {
			end = start + 1
		}
		if end > len(beats) {
			end = len(beats)
		}
		sceneBeats := append([]string{}, beats[start:end]...)
		scenes = append(scenes, OutlineScene{
			Order:      i + 1,
			POV:        defaultChapterPOV(chapter),
			Goal:       sceneGoal(chapter, sceneBeats, i),
			Location:   strings.TrimSpace(chapter.Location),
			Characters: append([]string{}, chapter.Characters...),
			Words:      1500,
			Tone:       chapter.Pacing,
			Beats:      sceneBeats,
		})
	}
	return scenes
}

func normalizeChapterTimeline(chapter *Chapter, chapterIndex int, report *OutlineNormalizationReport) {
	if strings.TrimSpace(chapter.Timeline.Anchor) == "" {
		chapter.Timeline.Anchor = fmt.Sprintf("第%d章", chapterIndex+1)
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   chapter.ID + ".timeline.anchor",
			Action: "fill_timeline_anchor",
			Value:  chapter.Timeline.Anchor,
		})
	}
}

func normalizeChapterEvents(chapter *Chapter, report *OutlineNormalizationReport) {
	for i := range chapter.Events {
		normalizeEventShape(chapter, &chapter.Events[i], i, report)
	}

	if len(chapter.Events) >= 3 {
		return
	}

	if strings.TrimSpace(chapter.StateChange) != "" && !chapterHasEventChange(chapter, chapter.StateChange) {
		chapter.Events = append(chapter.Events, Event{
			Type:       "status",
			Characters: protagonistCharacters(chapter),
			Subject:    defaultChapterPOV(chapter),
			Change:     strings.TrimSpace(chapter.StateChange),
			Details:    "由章节 state_change 迁移生成的结构事件。",
			Actor:      defaultChapterPOV(chapter),
			Action:     ActionTransform,
			Target:     "状态变化",
			TargetType: TargetTypeStatus,
			Context:    strings.TrimSpace(chapter.Location),
			Result:     strings.TrimSpace(chapter.StateChange),
		})
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   chapter.ID + ".events",
			Action: "add_state_change_event",
			Value:  trimForReport(chapter.StateChange),
		})
	}

	if len(chapter.Events) >= 3 {
		return
	}
	if strings.TrimSpace(chapter.Conflict) != "" && !chapterHasEventChange(chapter, chapter.Conflict) {
		chapter.Events = append(chapter.Events, Event{
			Type:       "gate",
			Characters: protagonistCharacters(chapter),
			Subject:    "章节冲突",
			Change:     strings.TrimSpace(chapter.Conflict),
			Details:    "由章节 conflict 迁移生成的结构事件。",
			Actor:      defaultChapterPOV(chapter),
			Action:     ActionDefend,
			Target:     "章节冲突",
			TargetType: TargetTypeGoal,
			Context:    strings.TrimSpace(chapter.Location),
			Result:     strings.TrimSpace(chapter.Conflict),
		})
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   chapter.ID + ".events",
			Action: "add_conflict_event",
			Value:  trimForReport(chapter.Conflict),
		})
	}

	if len(chapter.Events) >= 3 {
		return
	}
	if strings.TrimSpace(chapter.Summary) != "" && !chapterHasEventChange(chapter, chapter.Summary) {
		chapter.Events = append(chapter.Events, Event{
			Type:       "storyline",
			Characters: protagonistCharacters(chapter),
			Subject:    "章节推进",
			Change:     strings.TrimSpace(chapter.Summary),
			Details:    "由章节 summary 迁移生成的结构事件。",
			Actor:      defaultChapterPOV(chapter),
			Action:     ActionProgress,
			Target:     "章节推进",
			TargetType: TargetTypeStoryline,
			Context:    strings.TrimSpace(chapter.Location),
			Result:     strings.TrimSpace(chapter.Summary),
		})
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   chapter.ID + ".events",
			Action: "add_summary_event",
			Value:  trimForReport(chapter.Summary),
		})
	}
}

func normalizeEventShape(chapter *Chapter, event *Event, index int, report *OutlineNormalizationReport) {
	if event == nil {
		return
	}
	path := fmt.Sprintf("%s.events[%d]", chapter.ID, index)

	if strings.TrimSpace(event.Actor) == "" {
		actor := strings.TrimSpace(event.GetActor())
		if actor == "" {
			actor = defaultChapterPOV(chapter)
		}
		event.Actor = actor
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   path + ".actor",
			Action: "fill_event_actor",
			Value:  actor,
		})
	}

	if strings.TrimSpace(event.Action) == "" {
		action := strings.TrimSpace(event.GetAction())
		if action == "" {
			action = ActionDiscover
		}
		event.Action = action
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   path + ".action",
			Action: "fill_event_action",
			Value:  action,
		})
	}

	if strings.TrimSpace(event.Target) == "" {
		target := normalizeEventTarget(chapter, event)
		event.Target = target
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   path + ".target",
			Action: "fill_event_target",
			Value:  target,
		})
	}

	if strings.TrimSpace(event.TargetType) == "" {
		targetType := strings.TrimSpace(event.GetTargetType())
		if targetType == "" {
			targetType = TargetTypeKnowledge
		}
		event.TargetType = targetType
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   path + ".target_type",
			Action: "fill_event_target_type",
			Value:  targetType,
		})
	}

	if strings.TrimSpace(event.Context) == "" && strings.TrimSpace(chapter.Location) != "" {
		event.Context = strings.TrimSpace(chapter.Location)
		report.Changes = append(report.Changes, OutlineNormalizationChange{
			Path:   path + ".context",
			Action: "fill_event_context",
			Value:  event.Context,
		})
	}

	if strings.TrimSpace(event.Result) == "" {
		result := strings.TrimSpace(event.Change)
		if result == "" {
			result = strings.TrimSpace(event.Details)
		}
		if result != "" {
			event.Result = result
			report.Changes = append(report.Changes, OutlineNormalizationChange{
				Path:   path + ".result",
				Action: "fill_event_result",
				Value:  trimForReport(result),
			})
		}
	}
}

func normalizeEventTarget(chapter *Chapter, event *Event) string {
	if target := strings.TrimSpace(event.GetTarget()); target != "" {
		return target
	}
	switch strings.TrimSpace(event.Type) {
	case EventTypeStatus:
		return "状态变化"
	case EventTypeGate:
		return "章节冲突"
	case EventTypeStoryline:
		return "章节推进"
	case EventTypePremise:
		return "设定线索"
	case EventTypeGoal:
		return "角色目标"
	case EventTypeRelationship:
		return "角色关系"
	}
	if strings.TrimSpace(chapter.Summary) != "" {
		return "章节事件"
	}
	return "未知目标"
}

func normalizeStateAnchorLocation(chapter *Chapter, report *OutlineNormalizationReport) {
	if strings.TrimSpace(chapter.StateAnchor.Location) != "" || strings.TrimSpace(chapter.Location) == "" {
		return
	}
	chapter.StateAnchor.Location = strings.TrimSpace(chapter.Location)
	report.Changes = append(report.Changes, OutlineNormalizationChange{
		Path:   chapter.ID + ".state_anchor.location",
		Action: "fill_state_anchor_location",
		Value:  chapter.StateAnchor.Location,
	})
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

func fallbackChapterBeats(chapter *Chapter) []string {
	var beats []string
	if strings.TrimSpace(chapter.OpeningBeat) != "" {
		beats = append(beats, strings.TrimSpace(chapter.OpeningBeat))
	}
	if strings.TrimSpace(chapter.Summary) != "" {
		beats = append(beats, strings.TrimSpace(chapter.Summary))
	}
	if strings.TrimSpace(chapter.ClosingBeat) != "" {
		beats = append(beats, strings.TrimSpace(chapter.ClosingBeat))
	} else if strings.TrimSpace(chapter.StateChange) != "" {
		beats = append(beats, strings.TrimSpace(chapter.StateChange))
	}
	return compactStrings(beats)
}

func fallbackClosingBeat(chapter *Chapter) string {
	for _, value := range []string{chapter.ClosingBeat, chapter.StateChange, chapter.Conflict, chapter.Summary} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "本章完成既定情节推进，并留下下一章承接点。"
}

func defaultChapterPOV(chapter *Chapter) string {
	if len(chapter.Characters) > 0 && strings.TrimSpace(chapter.Characters[0]) != "" {
		return strings.TrimSpace(chapter.Characters[0])
	}
	return "林砚"
}

func protagonistCharacters(chapter *Chapter) []string {
	pov := defaultChapterPOV(chapter)
	if pov == "" {
		return nil
	}
	return []string{pov}
}

func sceneGoal(chapter *Chapter, beats []string, index int) string {
	if len(beats) > 0 && strings.TrimSpace(beats[0]) != "" {
		return "推进：" + trimForReport(beats[0])
	}
	if index == 0 && strings.TrimSpace(chapter.Summary) != "" {
		return "建立本章局面：" + trimForReport(chapter.Summary)
	}
	if strings.TrimSpace(chapter.StateChange) != "" {
		return "完成状态变化：" + trimForReport(chapter.StateChange)
	}
	return "推进本章核心冲突"
}

func compactStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := values[:0]
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

func chapterHasEventChange(chapter *Chapter, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	for _, event := range chapter.Events {
		if strings.TrimSpace(event.Change) == value || strings.TrimSpace(event.Details) == value {
			return true
		}
	}
	return false
}

func trimForReport(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 48 {
		return value
	}
	return string(runes[:48]) + "..."
}
