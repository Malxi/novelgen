package models

import (
	"encoding/json"
	"os"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string   `json:"project_name"`
	Genres         []string `json:"genres"`
	Premise        string   `json:"premise"`
	Theme          string   `json:"theme"`
	Rules          []string `json:"rules"`
	TargetAudience string   `json:"target_audience"`
	Tone           string   `json:"tone"`
	Tense          string   `json:"tense"`
	POVStyle       string   `json:"pov_style"`
}

// Save writes the story setup to a file
func (s *StorySetup) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStorySetup reads the story setup from a file
func LoadStorySetup(path string) (*StorySetup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var setup StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil, err
	}
	return &setup, nil
}
