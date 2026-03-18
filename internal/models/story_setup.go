package models

import (
	"encoding/json"
	"os"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string      `json:"project_name" prompt:"Project Name"`
	Genres         []string    `json:"genres" prompt:"Genres"`
	Premise        string      `json:"premise" prompt:"Premise"`
	Theme          string      `json:"theme" prompt:"Theme"`
	Rules          []string    `json:"rules" prompt:"Rules"`
	TargetAudience string      `json:"target_audience" prompt:"Target Audience"`
	Tone           string      `json:"tone" prompt:"Tone"`
	Tense          string      `json:"tense" prompt:"Tense"`
	POVStyle       string      `json:"pov_style" prompt:"POV Style"`
	Storylines     []Storyline `json:"storylines,omitempty" prompt:"Storylines"`
	Premises       []Premise   `json:"premises,omitempty" prompt:"Premises"`
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name        string `json:"name" prompt:"Name"`
	Description string `json:"description" prompt:"Description"`
	Type        string `json:"type" prompt:"Type"`             // main, subplot, character_arc, etc.
	Importance  int    `json:"importance" prompt:"Importance"` // 1-10, 10 being most important
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name        string             `json:"name" prompt:"Name"`
	Description string             `json:"description" prompt:"Description"`
	Category    string             `json:"category" prompt:"Category"`       // 机甲, 基因, 飞船, 魔法, etc.
	Progression []ProgressionStage `json:"progression" prompt:"Progression"` // 升级体系
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level" prompt:"Level"`
	Name         string `json:"name" prompt:"Name"`
	Description  string `json:"description" prompt:"Description"`
	Requirements string `json:"requirements,omitempty" prompt:"Requirements"`
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
