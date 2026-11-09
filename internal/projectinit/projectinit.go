// Package projectinit creates the on-disk skeleton of a new novel project.
package projectinit

import (
	"fmt"
	"strings"
	"time"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

// Options are the user-selected project creation values.
type Options struct {
	Name     string
	Parts    int
	Volumes  int
	Chapters int
	Genre    string
	Provider string
	Model    string
	Language string
}

// Create writes the project structure and novel.json. The target directory must
// not already contain novel.json.
func Create(book *project.Book, opts Options) (*models.ProjectConfig, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		return nil, fmt.Errorf("book name is required")
	}
	if opts.Parts <= 0 || opts.Volumes <= 0 || opts.Chapters <= 0 {
		return nil, fmt.Errorf("parts, volumes, and chapters must be positive")
	}
	if book == nil || book.Root() == nil {
		return nil, store.ErrNilRoot
	}
	if book.Root().HasConfig() {
		return nil, fmt.Errorf("a novel project already exists in this directory (novel.json found)")
	}
	if opts.Language == "" {
		opts.Language = "zh"
	}
	if opts.Provider == "" {
		opts.Provider = models.ProviderClaude
	}
	if opts.Model == "" {
		switch opts.Provider {
		case models.ProviderClaude:
			opts.Model = "sonnet"
		case models.ProviderOpenAI:
			opts.Model = "gpt-5.2"
		default:
			opts.Model = "qwen3.5:4b"
		}
	}
	genres := splitAndTrim(opts.Genre)
	if len(genres) == 0 {
		genres = []string{"未分类"}
	}
	now := time.Now()
	config := &models.ProjectConfig{
		Name:      opts.Name,
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
		Language:  opts.Language,
		Structure: models.StoryStructure{
			TargetParts:    opts.Parts,
			TargetVolumes:  opts.Volumes,
			TargetChapters: opts.Chapters,
		},
		ChapterConfig: models.DefaultChapterConfig(),
		LLM: models.ProjectLLM{
			Provider: opts.Provider,
			Model:    opts.Model,
		},
	}
	if err := createProjectStructure(book, config, genres); err != nil {
		return nil, fmt.Errorf("failed to create project structure: %w", err)
	}
	if err := book.Root().SaveConfig(config); err != nil {
		return nil, fmt.Errorf("failed to save novel.json: %w", err)
	}
	return config, nil
}

func createProjectStructure(book *project.Book, config *models.ProjectConfig, genres []string) error {
	dirs := []string{
		book.Root().SetupDir(),
		book.Root().ComposeDir(),
		book.Root().CraftDir(),
		book.Root().ReviewsDir(),
		book.Root().ChaptersDir(),
		book.Root().DraftsDir(),
		book.Root().LogsDir(),
	}
	for _, dir := range dirs {
		if err := book.Root().MkdirAll(dir, store.DirMode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	placeholder := fmt.Sprintf(`# %s

## Story Setup

### Genre(s)
%s

### Core Premise
(待填写)

### Theme
(待填写)

### Story Rules/Constraints
- (待填写)

### Target Audience
(待填写)

### Tone/Style
(待填写)

### Narrative Tense
past

### POV Style
third_person_limited

### Writing Style
None
`, config.Name, "- "+strings.Join(genres, ", "))
	if err := book.Root().WriteText(book.Root().SetupMarkdown(), placeholder); err != nil {
		return fmt.Errorf("failed to create story_setup.md: %w", err)
	}
	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
