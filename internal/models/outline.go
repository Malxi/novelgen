package models

import (
	"encoding/json"
	"os"
)

// Outline represents the complete story outline with 3-level structure (parts → volumes → chapters)
type Outline struct {
	Parts []Part `json:"parts"`
}

// Part represents a major section of the story
type Part struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	Volumes  []Volume  `json:"volumes"`
}

// Volume represents a subdivision of a part
type Volume struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	Chapters []Chapter `json:"chapters"`
}

// Chapter represents a single chapter in the story
type Chapter struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Beats    []string `json:"beats"`
	Conflict string   `json:"conflict"`
	Pacing   string   `json:"pacing"`
}

// Save writes the outline to a file
func (o *Outline) Save(path string) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadOutline reads the outline from a file
func LoadOutline(path string) (*Outline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var outline Outline
	if err := json.Unmarshal(data, &outline); err != nil {
		return nil, err
	}
	return &outline, nil
}

// GetChapterByID finds a chapter by its ID
func (o *Outline) GetChapterByID(id string) *Chapter {
	for _, part := range o.Parts {
		for _, volume := range part.Volumes {
			for i := range volume.Chapters {
				if volume.Chapters[i].ID == id {
					return &volume.Chapters[i]
				}
			}
		}
	}
	return nil
}

// GetVolumeByID finds a volume by its ID
func (o *Outline) GetVolumeByID(id string) *Volume {
	for _, part := range o.Parts {
		for i := range part.Volumes {
			if part.Volumes[i].ID == id {
				return &part.Volumes[i]
			}
		}
	}
	return nil
}

// GetPartByID finds a part by its ID
func (o *Outline) GetPartByID(id string) *Part {
	for i := range o.Parts {
		if o.Parts[i].ID == id {
			return &o.Parts[i]
		}
	}
	return nil
}
