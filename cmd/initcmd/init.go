// Package initcmd implements the `novelgen init` command.
package initcmd

import (
	"fmt"
	"strings"
	"time"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

var (
	initChapterFlag  int
	initGenreFlag    string
	initModeFlag     string
	initProviderFlag string
	initLanguageFlag string
)

var initCmd = &cobra.Command{
	Use:   "init [book_name]",
	Short: "Initialize a new novel project",
	Long: `Initialize a new novel project with the specified configuration.

This command creates a new novel project directory structure and novel.json
configuration file. It does NOT generate story setup - use 'novelgen setup' for that.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().IntVar(&initChapterFlag, "chapter", 20, "Number of chapters")
	initCmd.Flags().StringVar(&initGenreFlag, "genre", "", "Genre(s), comma-separated (e.g., '科幻,废土')")
	initCmd.Flags().StringVar(&initModeFlag, "mode", "", "LLM model to use (e.g., 'gpt-5.2')")
	initCmd.Flags().StringVar(&initProviderFlag, "provider", models.ProviderClaude, "LLM provider (claude, ollama, openai, etc.)")
	initCmd.Flags().StringVar(&initLanguageFlag, "language", "zh", "Story language (zh, en, ja, etc.)")
}

// NewCommand returns the `novelgen init` command.
func NewCommand() *cobra.Command {
	return initCmd
}

func runInit(cmd *cobra.Command, args []string) error {
	bookName := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN INIT")

	// init is the one command that does not need an existing project: it creates
	// one in the target directory (--dir, NOVELGEN_PROJECT_DIR, or the working
	// directory).
	target, err := project.CommandDir(cmd)
	if err != nil {
		return err
	}
	book := project.Open(target)

	// Check if novel.json already exists
	if book.HasConfig() {
		logger.Error("A novel project already exists in this directory (novel.json found)")
		return fmt.Errorf("a novel project already exists in this directory (novel.json found)")
	}

	logger.Info("Creating project: %s", bookName)

	// Parse genres
	var genres []string
	if initGenreFlag != "" {
		genres = splitAndTrim(initGenreFlag)
	} else {
		genres = []string{"未分类"}
	}

	// Create project config
	config := &models.ProjectConfig{
		Name:      bookName,
		Version:   "1.0.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  initLanguageFlag,
		Structure: models.StoryStructure{
			TargetParts:    1,
			TargetVolumes:  1,
			TargetChapters: initChapterFlag,
		},
		ChapterConfig: models.DefaultChapterConfig(),
		LLM: models.ProjectLLM{
			Provider: initProviderFlag,
			Model:    initModeFlag,
		},
	}

	// Set default model if not specified
	if config.LLM.Model == "" {
		if initProviderFlag == models.ProviderClaude {
			config.LLM.Model = "sonnet"
		} else if initProviderFlag == models.ProviderOpenAI {
			// Keep in sync with llm_config.json defaults
			config.LLM.Model = "gpt-5.2"
		} else {
			config.LLM.Model = "qwen3.5:4b"
		}
	}

	// Create project directory structure
	if err := createProjectStructure(book, config); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Save novel.json
	if err := book.SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save novel.json: %w", err)
	}

	fmt.Printf("\n✓ Novel project '%s' initialized successfully!\n", bookName)
	fmt.Printf("\n📊 Story Structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		config.Structure.TargetParts, config.Structure.TargetVolumes, config.Structure.TargetChapters,
		config.Structure.TotalChapters())
	fmt.Printf("🎭 Genre(s): %s\n", strings.Join(genres, ", "))
	fmt.Printf("🤖 Provider: %s, Model: %s\n", config.LLM.Provider, config.LLM.Model)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Run 'novelgen setup gen \"<your story idea>\"' to generate story setup with AI")
	fmt.Println("  - Or manually edit story/setup/story_setup.json to define your story")

	return nil
}

func createProjectStructure(book *project.Book, config *models.ProjectConfig) error {
	dirs := []string{
		book.SetupDir(),
		book.ComposeDir(),
		book.CraftDir(),
		book.ReviewsDir(),
		book.ChaptersDir(),
		book.DraftsDir(),
		book.LogsDir(),
	}

	for _, dir := range dirs {
		if err := book.MkdirAll(dir, store.DirMode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create placeholder story_setup.md
	mdPath := book.SetupMarkdown()
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
`,
		config.Name,
		"- 未分类",
	)

	if err := book.WriteText(mdPath, placeholder); err != nil {
		return fmt.Errorf("failed to create story_setup.md: %w", err)
	}

	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
