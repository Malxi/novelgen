// Outline flattening, entity/text matching and match-reason reporting used by the outline, chapter
// and refs/events queries.

package toolplane

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/utils"
	"strings"
)

type flatChapter struct {
	Chapter *models.Chapter
	Path    OutlinePath
}

type flatVolume struct {
	Volume *models.Volume
	Path   OutlinePath
}

func flattenVolumes(outline *models.Outline) []flatVolume {
	if outline == nil {
		return nil
	}
	result := make([]flatVolume, 0)
	for _, part := range outline.Parts {
		base := OutlinePath{PartID: part.ID, PartTitle: part.Title}
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
		base := OutlinePath{PartID: part.ID, PartTitle: part.Title}
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

func collectVolumeEvents(volume flatVolume) []EventHit {
	if volume.Volume == nil {
		return nil
	}
	hits := make([]EventHit, 0)
	for _, chapter := range volume.Volume.Chapters {
		chapterPath := volume.Path
		chapterPath.ChapterID = chapter.ID
		chapterPath.ChapterTitle = chapter.Title
		for eventIndex, event := range chapter.Events {
			hits = append(hits, EventHit{
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

func collectVolumeEntities(volume flatVolume) OutlineVolumeEntities {
	if volume.Volume == nil {
		return OutlineVolumeEntities{}
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
	return OutlineVolumeEntities{
		Characters: clipToolStrings(utils.CompactStrings(characters), 25, 70),
		Items:      clipToolStrings(utils.CompactStrings(items), 25, 70),
		Locations:  clipToolStrings(utils.CompactStrings(locations), 25, 70),
		Storylines: clipToolStrings(utils.CompactStrings(storylines), 25, 70),
	}
}

func collectChapterEntities(chapter models.Chapter) OutlineVolumeEntities {
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
	return OutlineVolumeEntities{
		Characters: clipToolStrings(utils.CompactStrings(characters), 18, 70),
		Items:      clipToolStrings(utils.CompactStrings(items), 18, 70),
		Locations:  clipToolStrings(utils.CompactStrings(locations), 12, 70),
		Storylines: clipToolStrings(utils.CompactStrings(storylines), 12, 70),
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
	case "storyline":
		return storylineChapterReasons(chapter, name)
	default:
		reasons := make([]string, 0)
		reasons = append(reasons, prefixedReasons("character", characterChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("item", itemChapterReasons(chapter, name))...)
		reasons = append(reasons, prefixedReasons("location", locationChapterReasons(chapter, name))...)
		return utils.CompactStrings(reasons)
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
	return utils.CompactStrings(reasons)
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
	return utils.CompactStrings(reasons)
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
	return utils.CompactStrings(reasons)
}

// storylineChapterReasons reports where a chapter advances a named storyline.
func storylineChapterReasons(chapter *models.Chapter, name string) []string {
	var reasons []string
	for idx, advance := range chapter.StorylineAdvances {
		if textMatches(advance.StorylineName, name) {
			reasons = append(reasons, fmt.Sprintf("storyline_advances[%d]", idx))
		}
	}
	if len(reasons) > 0 {
		return utils.CompactStrings(reasons)
	}
	// Fall back to free-text matching against the advance payload fields,
	// so a query with no exact storyline name can still surface mentions.
	for idx, advance := range chapter.StorylineAdvances {
		payload := advance.StorylineName + " " + advance.Stage + " " + advance.Change + " " + advance.Consequence + " " + advance.Pressure
		if textMatches(payload, name) {
			reasons = append(reasons, fmt.Sprintf("storyline_advances[%d].text", idx))
		}
	}
	return utils.CompactStrings(reasons)
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
