// Package translate implements the `novelgen translate` command.
package translate

import (
	"fmt"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/project"

	"github.com/spf13/cobra"
)

var (
	translateSourceLang string
	translateTargetLang string
	translateOutputFile string
	translateAgentSDK   bool
)

var translateCmd = &cobra.Command{
	Use:   "translate [file]",
	Short: "Translate novel content between languages",
	Long: `Translate novel content from one language to another using AI.

This command translates chapters, story setup, or other text files
while preserving the narrative style and formatting.

Examples:
  novelgen translate story/chapters/chapter_001.txt
  novelgen translate story/setup/story_setup.md --target-lang en
  novelgen translate chapter.txt --source-lang zh --target-lang en --output chapter_en.txt
  novelgen translate chapter.txt --source-lang zh --target-lang en --agent-sdk --output chapter_en.txt`,
	Args: cobra.ExactArgs(1),
	RunE: runTranslate,
}

func init() {
	// These flags are automatically registered when the command is loaded
	translateCmd.Flags().StringVar(&translateSourceLang, "source-lang", "zh", "Source language (auto-detect if not specified)")
	translateCmd.Flags().StringVar(&translateTargetLang, "target-lang", "en", "Target language for translation")
	translateCmd.Flags().StringVar(&translateOutputFile, "output", "", "Output file (default: stdout)")
	translateCmd.Flags().BoolVar(&translateAgentSDK, "agent-sdk", false, "Use Claude Agent SDK workflow for translation")
}

// NewCommand returns the `novelgen translate` command.
func NewCommand() *cobra.Command {
	return translateCmd
}

func runTranslate(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// Initialize logger
	logger.SetDefault(logger.New(logger.DebugLevel))
	logger.Section("NOVELGEN TRANSLATE")

	book, err := project.ForCommand(cmd)
	if err != nil {
		logger.Error("Not a novel project directory (novel.json not found)")
		return fmt.Errorf("not a novel project directory (novel.json not found). Run 'novelgen init <book_name>' first")
	}

	// Load project config
	projectConfig, err := book.LoadConfig()
	if err != nil {
		logger.Error("Failed to load novel.json: %v", err)
		return fmt.Errorf("failed to load novel.json: %w", err)
	}
	logger.Info("Loaded project config: %s", projectConfig.Name)

	// Read input file
	content, err := book.Read(book.ResolvePath(inputFile))
	if err != nil {
		logger.Error("Failed to read input file: %v", err)
		return fmt.Errorf("failed to read input file %s: %w", inputFile, err)
	}
	logger.Info("Read %d bytes from %s", len(content), inputFile)

	// Create LLM client
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	client := cfg.CreateClient(&projectConfig.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	agent := agents.NewTranslateAgent(client, cfg, &projectConfig.LLM)

	agent.SetLanguage(projectConfig.Language)

	// Perform translation
	logger.Info("Translating from %s to %s...", translateSourceLang, translateTargetLang)

	var translated string
	if translateAgentSDK {
		logger.Info("Translation will use Agent SDK; Go still owns file input/output")
		translated, err = agent.TranslateWithAgentSDK(cmd.Context(), string(content), translateSourceLang, translateTargetLang)
	} else {
		systemPrompt := buildTranslationSystemPrompt()
		userPrompt := buildTranslationUserPrompt(string(content), translateSourceLang, translateTargetLang)
		translated, err = agent.Translate(systemPrompt, userPrompt)
	}
	if err != nil {
		logger.Error("Translation failed: %v", err)
		return fmt.Errorf("translation failed: %w", err)
	}

	// Output result
	if translateOutputFile != "" {
		if err := book.WriteText(book.ResolvePath(translateOutputFile), translated); err != nil {
			logger.Error("Failed to write output file: %v", err)
			return fmt.Errorf("failed to write output file: %w", err)
		}
		logger.Info("Translation saved to: %s", translateOutputFile)
	} else {
		fmt.Println(translated)
	}

	logger.Info("Translation completed!")
	return nil
}

// buildTranslationSystemPrompt returns the system prompt for translation
func buildTranslationSystemPrompt() string {
	return `You are a professional literary translator specializing in novel translation.

Your task is to translate text while preserving:
- Narrative flow and pacing
- Character voice and dialogue style
- Descriptive richness and atmosphere
- Cultural nuances and idioms (adapt appropriately)
- Tone and emotional impact

Translate naturally - don't translate word-for-word. Make the text feel like it was originally written in the target language.

Output only the translated text without explanations, notes, or formatting markers.`
}

// buildTranslationUserPrompt builds the user prompt for translation
func buildTranslationUserPrompt(content, sourceLang, targetLang string) string {
	return fmt.Sprintf(`Please translate the following text from %s to %s.

IMPORTANT TRANSLATION GUIDELINES:
1. Preserve the narrative style, tone, and voice of the original
2. Maintain character names, place names, and proper nouns appropriately
3. Keep paragraph structure and formatting intact
4. Ensure natural flow in the target language
5. Preserve dialogue style and character voice
6. Maintain cultural context where appropriate

SOURCE TEXT (%s):
%s`, sourceLang, targetLang, sourceLang, content)
}
