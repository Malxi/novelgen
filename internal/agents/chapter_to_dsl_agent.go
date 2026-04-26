package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

// ChapterToDSLInput is the input for converting chapters to DSL
type ChapterToDSLInput struct {
	BookName   string                      `json:"book_name" md:"book_name" desc:"Book name"`
	StorySetup models.StorySetup           `json:"story_setup" md:"story_setup" desc:"Story setup with premise, genres, themes"`
	Characters map[string]models.Character `json:"characters" md:"characters" desc:"Characters from craft"`
	Locations  map[string]models.Location  `json:"locations" md:"locations" desc:"Locations from craft"`
	Chapters   []ChapterData               `json:"chapters" md:"chapters" desc:"Chapter full text with optional recap data"`
}

// ChapterData represents a chapter's full text plus optional recap summary.
type ChapterData struct {
	ChapterID string        `json:"chapter_id"`
	Title     string        `json:"title"`
	Content   string        `json:"content,omitempty" md:"content" desc:"Full chapter markdown text; primary source for DSL extraction"`
	Location  string        `json:"location,omitempty"`
	Time      string        `json:"time,omitempty"`
	Present   []string      `json:"present,omitempty"`
	PlotBeats []string      `json:"plot_beats,omitempty"`
	Recap     *ChapterRecap `json:"recap,omitempty" md:"recap" desc:"Optional structured recap; use only as support, never as replacement for content"`
}

type ChapterRecap struct {
	ChapterID       string   `json:"chapter_id,omitempty"`
	Title           string   `json:"title,omitempty"`
	Location        string   `json:"location,omitempty"`
	Time            string   `json:"time,omitempty"`
	Present         []string `json:"present,omitempty"`
	PlotBeats       []string `json:"plot_beats,omitempty"`
	LastLine        string   `json:"last_line,omitempty"`
	NextOpeningHint string   `json:"next_opening_hint,omitempty"`
}

// ChapterToDSLOutput is the output containing the DSL content
type ChapterToDSLOutput struct {
	DSLContent string `json:"dsl_content" md:"dsl_content" desc:"Generated RPG-DSL code"`
}

// ChapterToDSLRepairInput is the input for repairing invalid DSL
type ChapterToDSLRepairInput struct {
	BookName     string `json:"book_name" md:"book_name" desc:"Book name"`
	ParseError   string `json:"parse_error" md:"parse_error" desc:"Parser error message"`
	InvalidDSL   string `json:"invalid_dsl" md:"invalid_dsl" desc:"Invalid DSL content to repair"`
	LanguageHint string `json:"language_hint" md:"language_hint" desc:"Language hint for text fields"`
}

// ChapterToDSLAgent converts novelgen chapters to RPG-DSL format
type ChapterToDSLAgent struct {
	base      *BaseAgent
	setup     *models.StorySetup
	requireAI bool
}

// NewChapterToDSLAgent creates a new ChapterToDSLAgent
func NewChapterToDSLAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, setup *models.StorySetup) *ChapterToDSLAgent {
	// 如果没有提供 projectLLM，则从 config 创建
	if projectLLM == nil && config != nil {
		projectLLM = &models.ProjectLLM{
			Provider: config.DefaultProvider,
			Model:    config.DefaultModel,
		}
	}

	base := NewBaseAgent(BaseAgentConfig{
		Name:       "ChapterToDSLAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &ChapterToDSLAgent{
		base:      base,
		setup:     setup,
		requireAI: false,
	}
}

// SetLanguage sets the output language
func (a *ChapterToDSLAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// SetRequireAI toggles strict mode: if true, conversion must succeed via AI.
func (a *ChapterToDSLAgent) SetRequireAI(requireAI bool) {
	a.requireAI = requireAI
}

// ConvertChapters converts multiple chapters to DSL format
func (a *ChapterToDSLAgent) ConvertChapters(ctx context.Context, bookName string, characters map[string]models.Character, locations map[string]models.Location, chapterFiles []string) (string, error) {
	logger.Section("CHAPTER TO DSL AGENT - Converting Chapters")
	logger.Info("Book: %s, Chapters: %d", bookName, len(chapterFiles))
	logger.Info("Language: %s", a.base.language)

	// Load all chapter data
	var chapters []ChapterData
	for _, file := range chapterFiles {
		chapter, err := a.loadChapterFile(file)
		if err != nil {
			logger.Warn("Failed to load chapter %s: %v", file, err)
			continue
		}
		chapters = append(chapters, *chapter)
	}

	return a.ConvertChapterData(ctx, bookName, characters, locations, chapters)
}

// ConvertChapterData converts already-loaded full chapter data to DSL format.
func (a *ChapterToDSLAgent) ConvertChapterData(ctx context.Context, bookName string, characters map[string]models.Character, locations map[string]models.Location, chapters []ChapterData) (string, error) {
	if len(chapters) == 0 {
		return "", fmt.Errorf("no valid chapters found")
	}

	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].ChapterID < chapters[j].ChapterID
	})

	logger.Info("Loaded %d chapters", len(chapters))

	// If LLM is not configured, use rule-based conversion
	if a.base.config == nil {
		if a.requireAI {
			return "", fmt.Errorf("LLM not configured and requireAI=true")
		}
		logger.Info("LLM not configured, using rule-based conversion")
		return a.convertChaptersRuleBased(bookName, characters, locations, chapters)
	}

	// Prepare input
	input := ChapterToDSLInput{
		BookName:   bookName,
		StorySetup: *a.setup,
		Characters: characters,
		Locations:  locations,
		Chapters:   chapters,
	}

	// Execute conversion
	var output ChapterToDSLOutput
	params := InvokeParams{
		Skills:  []string{"chapter-to-dsl"},
		Command: "convert chapters to RPG-DSL format",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		if a.requireAI {
			return "", fmt.Errorf("AI conversion failed: %w", err)
		}
		logger.Warn("AI conversion failed: %v", err)
		logger.Info("Falling back to rule-based conversion")
		return a.convertChaptersRuleBased(bookName, characters, locations, chapters)
	}

	logger.Info("✓ Generated DSL content (%d characters)", len(output.DSLContent))

	return output.DSLContent, nil
}

// loadChapterFile loads a single chapter recap JSON or chapter markdown file.
func (a *ChapterToDSLAgent) loadChapterFile(filePath string) (*ChapterData, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if strings.EqualFold(filepath.Ext(filePath), ".md") {
		chapterID := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(filePath), "chapter-"), filepath.Ext(filePath))
		return &ChapterData{
			ChapterID: chapterID,
			Title:     firstMarkdownHeading(string(content), chapterID),
			Content:   string(content),
		}, nil
	}

	var chapter ChapterData
	if err := json.Unmarshal(content, &chapter); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &chapter, nil
}

func firstMarkdownHeading(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return fallback
}

// ConvertSingleChapter converts a single chapter to DSL (for testing)
func (a *ChapterToDSLAgent) ConvertSingleChapter(ctx context.Context, bookName string, characters map[string]models.Character, locations map[string]models.Location, chapterFile string) (string, error) {
	logger.Section("CHAPTER TO DSL AGENT - Converting Single Chapter")
	logger.Info("File: %s", chapterFile)

	chapter, err := a.loadChapterFile(chapterFile)
	if err != nil {
		return "", err
	}

	input := ChapterToDSLInput{
		BookName:   bookName,
		StorySetup: *a.setup,
		Characters: characters,
		Locations:  locations,
		Chapters:   []ChapterData{*chapter},
	}

	var output ChapterToDSLOutput
	params := InvokeParams{
		Skills:  []string{"chapter-to-dsl"},
		Command: "convert chapter to RPG-DSL format",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}

	return output.DSLContent, nil
}

// RepairDSL asks AI to repair an invalid DSL string into parser-compatible DSL.
func (a *ChapterToDSLAgent) RepairDSL(ctx context.Context, bookName, parseErr, invalidDSL string) (string, error) {
	if a.base.config == nil {
		return "", fmt.Errorf("LLM not configured")
	}

	input := ChapterToDSLRepairInput{
		BookName:     bookName,
		ParseError:   parseErr,
		InvalidDSL:   invalidDSL,
		LanguageHint: a.base.language,
	}

	var output ChapterToDSLOutput
	params := InvokeParams{
		Skills:  []string{"chapter-to-dsl-repair"},
		Command: "repair invalid RPG-DSL to valid parser-compatible DSL",
	}

	if err := a.base.Execute(ctx, params, input, &output); err != nil {
		return "", err
	}
	return output.DSLContent, nil
}

// GenerateOutlineDSL generates a complete outline DSL from all chapters
func (a *ChapterToDSLAgent) GenerateOutlineDSL(ctx context.Context, bookName string, characters map[string]models.Character, locations map[string]models.Location, chaptersDir string) (string, error) {
	// Find all chapter files
	pattern := filepath.Join(chaptersDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to find chapter files: %w", err)
	}

	// Filter only valid chapter files (P1-V*-C*.json)
	var validFiles []string
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "P") && strings.Contains(base, "-V") && strings.Contains(base, "-C") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) == 0 {
		return "", fmt.Errorf("no valid chapter files found in %s", chaptersDir)
	}

	// Sort files by name
	sort.Strings(validFiles)

	// Convert all chapters
	return a.ConvertChapters(ctx, bookName, characters, locations, validFiles)
}

// convertChaptersRuleBased performs rule-based conversion without LLM
func (a *ChapterToDSLAgent) convertChaptersRuleBased(bookName string, characters map[string]models.Character, locations map[string]models.Location, chapters []ChapterData) (string, error) {
	logger.Info("Performing rule-based DSL conversion")

	var dsl strings.Builder

	// Generate metadata
	dsl.WriteString(fmt.Sprintf("# %s Outline DSL\n", bookName))
	dsl.WriteString(fmt.Sprintf("# Generated from chapter recaps\n\n"))
	dsl.WriteString(fmt.Sprintf("metadata {\n"))
	dsl.WriteString(fmt.Sprintf("  title = \"%s\"\n", bookName))
	dsl.WriteString(fmt.Sprintf("  dsl_version = \"0.2.0\"\n"))
	dsl.WriteString(fmt.Sprintf("  source = \"novelgen_outline\"\n"))
	dsl.WriteString(fmt.Sprintf("}\n\n"))

	// Generate characters section
	if len(characters) > 0 {
		dsl.WriteString(fmt.Sprintf("characters {\n"))
		for id, char := range characters {
			dsl.WriteString(fmt.Sprintf("  npc \"%s\" {\n", char.Name))
			dsl.WriteString(fmt.Sprintf("    id = \"%s\"\n", id))
			dsl.WriteString(fmt.Sprintf("    name = \"%s\"\n", char.Name))
			if char.Appearance != "" {
				dsl.WriteString(fmt.Sprintf("    description = \"%s\"\n", escapeDSLString(char.Appearance)))
			}
			if char.RoleInStory != "" {
				dsl.WriteString(fmt.Sprintf("    role = \"%s\"\n", char.RoleInStory))
			}
			dsl.WriteString(fmt.Sprintf("    __placeholder__ = false\n"))
			dsl.WriteString(fmt.Sprintf("  }\n"))
		}
		dsl.WriteString(fmt.Sprintf("}\n\n"))
	}

	// Generate world section
	if len(locations) > 0 {
		dsl.WriteString(fmt.Sprintf("world {\n"))
		for id, loc := range locations {
			dsl.WriteString(fmt.Sprintf("  location \"%s\" {\n", loc.Name))
			dsl.WriteString(fmt.Sprintf("    id = \"%s\"\n", id))
			dsl.WriteString(fmt.Sprintf("    name = \"%s\"\n", loc.Name))
			if loc.Description != "" {
				dsl.WriteString(fmt.Sprintf("    description = \"%s\"\n", escapeDSLString(loc.Description)))
			}
			dsl.WriteString(fmt.Sprintf("    __placeholder__ = false\n"))
			dsl.WriteString(fmt.Sprintf("  }\n"))
		}
		dsl.WriteString(fmt.Sprintf("}\n\n"))
	}

	// Generate storyline section
	dsl.WriteString(fmt.Sprintf("storyline {\n"))
	for _, chapter := range chapters {
		dsl.WriteString(fmt.Sprintf("  chapter \"%s\" {\n", chapter.Title))
		dsl.WriteString(fmt.Sprintf("    id = \"%s\"\n", chapter.ChapterID))
		dsl.WriteString(fmt.Sprintf("\n"))
		dsl.WriteString(fmt.Sprintf("    objective \"主要剧情\" {\n"))

		// Convert plot beats to steps
		for i, beat := range chapter.PlotBeats {
			if i >= 10 { // Limit steps
				break
			}
			dsl.WriteString(fmt.Sprintf("      step %d {\n", i+1))
			dsl.WriteString(fmt.Sprintf("        description = \"%s\"\n", escapeDSLString(beat)))
			dsl.WriteString(fmt.Sprintf("        event {\n"))
			dsl.WriteString(fmt.Sprintf("          type = \"%s\"\n", detectEventType(beat)))
			dsl.WriteString(fmt.Sprintf("        }\n"))
			dsl.WriteString(fmt.Sprintf("      }\n"))
		}

		dsl.WriteString(fmt.Sprintf("    }\n"))
		dsl.WriteString(fmt.Sprintf("  }\n"))
	}
	dsl.WriteString(fmt.Sprintf("}\n"))

	return dsl.String(), nil
}

// detectEventType determines the event type from beat content
func detectEventType(beat string) string {
	beatLower := strings.ToLower(beat)

	// Combat keywords
	if strings.Contains(beatLower, "杀") || strings.Contains(beatLower, "战") ||
		strings.Contains(beatLower, "打") || strings.Contains(beatLower, "斗") ||
		strings.Contains(beatLower, "攻击") || strings.Contains(beatLower, "战斗") {
		return "combat"
	}

	// Location/movement keywords
	if strings.Contains(beatLower, "去") || strings.Contains(beatLower, "来到") ||
		strings.Contains(beatLower, "进入") || strings.Contains(beatLower, "离开") ||
		strings.Contains(beatLower, "到达") || strings.Contains(beatLower, "前往") {
		return "location"
	}

	// Acquire keywords
	if strings.Contains(beatLower, "获得") || strings.Contains(beatLower, "得到") ||
		strings.Contains(beatLower, "找到") || strings.Contains(beatLower, "发现") ||
		strings.Contains(beatLower, "拿到") || strings.Contains(beatLower, "取") {
		return "acquire"
	}

	// Knowledge keywords
	if strings.Contains(beatLower, "知道") || strings.Contains(beatLower, "了解") ||
		strings.Contains(beatLower, "得知") || strings.Contains(beatLower, "明白") ||
		strings.Contains(beatLower, "领悟") || strings.Contains(beatLower, "学习") {
		return "knowledge"
	}

	// Relationship keywords
	if strings.Contains(beatLower, "认识") || strings.Contains(beatLower, "结交") ||
		strings.Contains(beatLower, "相遇") || strings.Contains(beatLower, "对话") ||
		strings.Contains(beatLower, "交流") {
		return "relationship"
	}

	// Default
	return "status"
}

// escapeDSLString escapes strings for DSL output
func escapeDSLString(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
