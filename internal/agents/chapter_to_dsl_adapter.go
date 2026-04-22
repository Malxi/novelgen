package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// NovelgenProjectAdapter adapts rpg.NovelgenProject to agent input format
type NovelgenProjectAdapter struct {
	project *rpg.NovelgenProject
}

// NewNovelgenProjectAdapter creates a new adapter
func NewNovelgenProjectAdapter(project *rpg.NovelgenProject) *NovelgenProjectAdapter {
	return &NovelgenProjectAdapter{project: project}
}

// LoadNovelgenProject loads a novelgen project and returns it wrapped in adapter
func LoadNovelgenProject(bookName string) (*NovelgenProjectAdapter, error) {
	// 使用当前目录作为项目路径
	project, err := rpg.LoadNovelgenProject(".", bookName)
	if err != nil {
		return nil, err
	}

	return NewNovelgenProjectAdapter(project), nil
}

// GetStorySetup creates a StorySetup from setup.json or outline
func (a *NovelgenProjectAdapter) GetStorySetup() *models.StorySetup {
	// 优先尝试从 setup.json 加载
	setupPath := filepath.Join("books", a.project.BookName, "story", "setup.json")
	if setup, err := models.LoadStorySetup(setupPath); err == nil {
		// 成功加载 setup.json
		return setup
	}

	// 回退：从 outline 创建基本设定
	setup := &models.StorySetup{
		ProjectName:    a.project.BookName,
		Genres:         []string{},
		Premise:        "",
		Theme:          "",
		Rules:          []string{},
		TargetAudience: "",
		Tone:           "",
		Tense:          "过去时",
		POVStyle:       "第三人称有限视角",
	}

	// 尝试从 outline 提取信息
	if len(a.project.Outline.Parts) > 0 {
		part := a.project.Outline.Parts[0]
		if part.Title != "" {
			setup.ProjectName = part.Title
		}
		setup.Premise = part.Summary
	}

	return setup
}

// GetCharacters converts NovelgenCharacter to models.Character
func (a *NovelgenProjectAdapter) GetCharacters() map[string]models.Character {
	characters := make(map[string]models.Character)

	for id, nc := range a.project.Characters {
		mc := models.Character{
			Name:         nc.Name,
			Aliases:      nc.Aliases,
			Age:          nc.Age,
			Gender:       nc.Gender,
			Race:         nc.Race,
			Appearance:   nc.Appearance,
			Personality:  nc.Personality,
			Background:   nc.Background,
			Motivation:   nc.Motivation,
			Skills:       nc.Skills,
			Abilities:    nc.Abilities,
			Affiliations: nc.Affiliations,
			RoleInStory:  nc.RoleInStory,
			Voice:        nc.Voice,
			Notes:        nc.Notes,
		}

		characters[id] = mc
	}

	return characters
}

// GetLocations converts NovelgenLocation to models.Location
func (a *NovelgenProjectAdapter) GetLocations() map[string]models.Location {
	locations := make(map[string]models.Location)

	for id, nl := range a.project.Locations {
		ml := models.Location{
			Name:               nl.Name,
			Description:        nl.Description,
			Appearance:         nl.Appearance,
			Atmosphere:         nl.Atmosphere,
			History:            nl.History,
			Inhabitants:        nl.Inhabitants,
			ConnectedLocations: nl.ConnectedLocs,
			Events:             nl.Events,
			Secrets:            nl.Secrets,
			Notes:              nl.Notes,
		}
		// 处理 sensory details
		if len(nl.SensoryDetails) > 0 {
			ml.SensoryDetails = &models.SensoryDetails{}
			for sense, details := range nl.SensoryDetails {
				switch sense {
				case "visual", "sights":
					ml.SensoryDetails.Sights = details
				case "auditory", "sounds":
					ml.SensoryDetails.Sounds = details
				case "olfactory", "smells":
					ml.SensoryDetails.Smells = details
				case "tactile", "textures":
					ml.SensoryDetails.Textures = details
				}
			}
		}
		locations[id] = ml
	}

	return locations
}

// GetOutline returns the story outline
func (a *NovelgenProjectAdapter) GetOutline() *rpg.StoryOutline {
	return &a.project.Outline
}

// FindChapterRecaps returns all chapter recap file paths
func (a *NovelgenProjectAdapter) FindChapterRecaps() ([]string, error) {
	recapsDir := filepath.Join("books", a.project.BookName, "story", "recaps")
	
	pattern := filepath.Join(recapsDir, "P*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find chapter recaps: %w", err)
	}

	return files, nil
}

// LoadChapterRecap loads a single chapter recap
func (a *NovelgenProjectAdapter) LoadChapterRecap(filePath string) (*ChapterData, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var chapter ChapterData
	if err := json.Unmarshal(content, &chapter); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &chapter, nil
}
