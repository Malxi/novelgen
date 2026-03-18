package models

import (
	"encoding/json"
	"os"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string      `json:"project_name"`
	Genres         []string    `json:"genres"`
	Premise        string      `json:"premise"`
	Theme          string      `json:"theme"`
	Rules          []string    `json:"rules"`
	TargetAudience string      `json:"target_audience"`
	Tone           string      `json:"tone"`
	Tense          string      `json:"tense"`
	POVStyle       string      `json:"pov_style"`
	Storylines     []Storyline `json:"storylines,omitempty"`
	Premises       []Premise   `json:"premises,omitempty"`
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`       // main, subplot, character_arc, etc.
	Importance  int    `json:"importance"` // 1-10, 10 being most important
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"`    // 机甲, 基因, 飞船, 魔法, etc.
	Progression []ProgressionStage `json:"progression"` // 升级体系
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Requirements string `json:"requirements,omitempty"`
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
