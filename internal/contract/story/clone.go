package models

import "encoding/json"

// CloneOutline returns a JSON round-tripped copy of the outline. Mutation
// services snapshot the pre-write value with this helper.
func CloneOutline(outline *Outline) *Outline {
	if outline == nil {
		return nil
	}
	data, _ := json.Marshal(outline)
	var cloned Outline
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}

// CloneStorySetup returns a JSON round-tripped copy of the story setup.
func CloneStorySetup(setup *StorySetup) StorySetup {
	if setup == nil {
		return StorySetup{}
	}
	data, _ := json.Marshal(setup)
	var cloned StorySetup
	_ = json.Unmarshal(data, &cloned)
	return cloned
}
