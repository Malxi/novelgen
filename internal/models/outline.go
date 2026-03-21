package models

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Outline represents the complete story outline with 3-level structure (parts → volumes → chapters)
type Outline struct {
	Parts []Part `json:"parts" md:"parts"`
}

// Part represents a major section of the story
type Part struct {
	ID      string   `json:"id" md:"-"` // ID not shown in markdown
	Title   string   `json:"title" md:"title"`
	Summary string   `json:"summary" md:"heading"`
	Volumes []Volume `json:"volumes" md:"volumes"`
}

// Volume represents a subdivision of a part
type Volume struct {
	ID       string    `json:"id" md:"-"` // ID not shown in markdown
	Title    string    `json:"title" md:"title"`
	Summary  string    `json:"summary" md:"heading"`
	Chapters []Chapter `json:"chapters" md:"chapters"`
}

// Chapter represents a single chapter in the story
type Chapter struct {
	ID         string   `json:"id" md:"-"` // ID not shown in markdown
	Title      string   `json:"title" md:"title"`
	Summary    string   `json:"summary" md:"heading"`       // 格式: 角色 在 什么地方 发生了 什么事
	Characters []string `json:"characters" md:"characters"` // 本章出现的角色名列表
	Location   string   `json:"location" md:"location"`     // 事情发生的地点
	Events     []Event  `json:"events" md:"events"`         // 本章发生的事件
	Beats      []string `json:"beats" md:"beats"`
	Conflict   string   `json:"conflict" md:"conflict"`
	Pacing     string   `json:"pacing" md:"pacing"`
}

// Event represents a story event that changes state
type Event struct {
	Type       string   `json:"type" md:"type"`                 // relationship, goal, item, premise, storyline
	Characters []string `json:"characters" md:"characters"`     // 涉及的角色
	Subject    string   `json:"subject" md:"subject"`           // 目标角色/物品/体系/故事线
	Change     string   `json:"change" md:"change"`             // 变化描述 (started, progressed, completed, etc.)
	Details    string   `json:"details,omitempty" md:"details"` // 额外详情，用于 storyline 进度描述等
}

// Event type constants
const (
	EventTypeRelationship = "relationship" // (relationship, characterA, characterB, change) 角色关系变化
	EventTypeGoal         = "goal"         // (goal, character, change) 角色目标更新
	EventTypeItem         = "item"         // (item, character, get/lost) 角色物品更新
	EventTypePremise      = "premise"      // (premise, character, change) 角色体系更新
	EventTypeStoryline    = "storyline"    // (storyline, change) 故事线更新
)

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

// ToMarkdown converts the outline to markdown format using reflection
func (o *Outline) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Story Outline\n\n")

	for _, part := range o.Parts {
		sb.WriteString(part.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts part to markdown
func (p *Part) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", p.Title))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", p.Summary))

	for _, volume := range p.Volumes {
		sb.WriteString(volume.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts volume to markdown
func (v *Volume) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", v.Title))
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", v.Summary))

	for _, chapter := range v.Chapters {
		sb.WriteString(chapter.ToMarkdown())
	}

	return sb.String()
}

// ToMarkdown converts chapter to markdown
func (c *Chapter) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#### %s\n\n", c.Title))

	// Summary
	sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", c.Summary))

	// Characters
	if len(c.Characters) > 0 {
		sb.WriteString(fmt.Sprintf("**Characters:** %s\n\n", strings.Join(c.Characters, ", ")))
	}

	// Location
	if c.Location != "" {
		sb.WriteString(fmt.Sprintf("**Location:** %s\n\n", c.Location))
	}

	// Events
	if len(c.Events) > 0 {
		sb.WriteString("**Events:**\n")
		for _, event := range c.Events {
			sb.WriteString(event.ToMarkdown())
		}
		sb.WriteString("\n")
	}

	// Beats
	if len(c.Beats) > 0 {
		sb.WriteString("**Beats:**\n")
		for i, beat := range c.Beats {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, beat))
		}
		sb.WriteString("\n")
	}

	// Conflict
	if c.Conflict != "" {
		sb.WriteString(fmt.Sprintf("**Conflict:** %s\n\n", c.Conflict))
	}

	// Pacing
	if c.Pacing != "" {
		sb.WriteString(fmt.Sprintf("**Pacing:** %s\n\n", c.Pacing))
	}

	sb.WriteString("---\n\n")

	return sb.String()
}

// ToMarkdown converts event to markdown
func (e *Event) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- **%s**", e.Type))

	if len(e.Characters) > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(e.Characters, ", ")))
	}

	if e.Subject != "" {
		sb.WriteString(fmt.Sprintf(" [%s]", e.Subject))
	}

	if e.Change != "" {
		sb.WriteString(fmt.Sprintf(": %s", e.Change))
	}

	if e.Details != "" {
		sb.WriteString(fmt.Sprintf(" - %s", e.Details))
	}

	sb.WriteString("\n")

	return sb.String()
}
