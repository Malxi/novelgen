// Package initcmd implements the `novelgen init` command.
package initcmd

import (
	"fmt"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/logger"
	"novelgen/internal/projectinit"

	"github.com/spf13/cobra"
)

var (
	initPartsFlag    int
	initVolumesFlag  int
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
	initCmd.Flags().IntVar(&initPartsFlag, "parts", 1, "Number of parts (部)")
	initCmd.Flags().IntVar(&initVolumesFlag, "volumes", 1, "Number of volumes per part (卷)")
	initCmd.Flags().IntVar(&initChapterFlag, "chapter", 20, "Number of chapters per volume (章)")
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
	if initPartsFlag <= 0 || initVolumesFlag <= 0 || initChapterFlag <= 0 {
		return fmt.Errorf("--parts, --volumes, and --chapter must be positive")
	}

	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN INIT")

	target, err := project.CommandDir(cmd)
	if err != nil {
		return err
	}
	book := project.Open(target)
	if book.Root().HasConfig() {
		logger.Error("A novel project already exists in this directory (novel.json found)")
		return fmt.Errorf("a novel project already exists in this directory (novel.json found)")
	}

	logger.Info("Creating project: %s", args[0])
	config, err := projectinit.Create(book, projectinit.Options{
		Name:     args[0],
		Parts:    initPartsFlag,
		Volumes:  initVolumesFlag,
		Chapters: initChapterFlag,
		Genre:    initGenreFlag,
		Provider: initProviderFlag,
		Model:    initModeFlag,
		Language: initLanguageFlag,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n✓ Novel project '%s' initialized successfully!\n", config.Name)
	fmt.Printf("\n📊 Story Structure: %d parts × %d volumes × %d chapters = %d total chapters\n",
		config.Structure.TargetParts, config.Structure.TargetVolumes, config.Structure.TargetChapters,
		config.Structure.TotalChapters())
	genre := initGenreFlag
	if genre == "" {
		genre = "未分类"
	}
	fmt.Printf("🎭 Genre(s): %s\n", genre)
	fmt.Printf("🤖 Provider: %s, Model: %s\n", config.LLM.Provider, config.LLM.Model)
	fmt.Println("\nNext steps:")
	fmt.Println("  - Run 'novelgen setup gen \"<your story idea>\"' to generate story setup with AI")
	fmt.Println("  - Or manually edit story/setup/story_setup.json to define your story")
	return nil
}
