package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"

	"github.com/spf13/cobra"
)

var (
	recapChapterFlag     string
	recapAllFlag         bool
	recapConcurrencyFlag int
	recapSourceFlag      string
)

var recapCmd = &cobra.Command{
	Use:   "recap",
	Short: "Extract canonical recaps for continuity",
	Long: `Extract high-signal, canonical recap JSON for chapters.

Recaps are saved to story/recaps/<chapterID>.json and are designed to improve
chapter-to-chapter continuity (scene anchors, unresolved beats, promises, items, status).`,
}

var recapGenCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate recap JSON from final chapters",
	Long: `Generate recap JSON for chapters.

Examples:
  novelgen recap gen --chapter 1
  novelgen recap gen --chapter 1-10
  novelgen recap gen --all
  novelgen recap gen --source chapters`,
	RunE: runRecapGen,
}

func init() {
	recapCmd.AddCommand(recapGenCmd)

	recapGenCmd.Flags().StringVar(&recapChapterFlag, "chapter", "", "Chapter number(s) to recap (e.g., '1', '1-5', or 'P1-V1-C1')")
	recapGenCmd.Flags().BoolVar(&recapAllFlag, "all", false, "Generate recaps for all chapters")
	recapGenCmd.Flags().StringVar(&recapSourceFlag, "source", "chapters", "Source text: chapters|drafts (drafts is legacy)")
	recapGenCmd.Flags().IntVar(&recapConcurrencyFlag, "concurrency", 1, "Number of concurrent recap generations")

	// Register recap command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return recapCmd
	})
}

func runRecapGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	// Load project config
	config, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Load outline
	outline, err := loadOutline()
	if err != nil {
		return fmt.Errorf("failed to load outline: %w", err)
	}

	// Load LLM config
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load LLM config: %w", err)
	}

	// Create LLM client
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return fmt.Errorf("failed to create LLM client")
	}

	// Recap agent + store
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}
	store := recap.NewStore(root)
	agent := agents.NewRecapAgent(client, cfg, &config.LLM)
	agent.SetLanguage(config.Language)

	// Chapters to process
	chapters, err := getChaptersToGenerate(outline, recapChapterFlag, "", "", recapAllFlag)
	if err != nil {
		return err
	}
	if len(chapters) == 0 {
		return fmt.Errorf("no chapters selected")
	}

	src := strings.ToLower(strings.TrimSpace(recapSourceFlag))
	if src != "drafts" && src != "chapters" {
		return fmt.Errorf("invalid --source: %s (expected drafts|chapters)", recapSourceFlag)
	}

	concurrency := recapConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	log.Info("Generating recaps for %d chapter(s) from %s with concurrency %d", len(chapters), src, concurrency)

	chapterChan := make(chan *models.Chapter, len(chapters))
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for ch := range chapterChan {
				text := ""
				if src == "drafts" {
					text = loadDraftContent(ch.ID)
				} else {
					text = loadFinalChapterContent(ch)
				}
				if strings.TrimSpace(text) == "" {
					log.Warn("[Worker %d] No source text for %s (%s); skipping", workerID, ch.ID, src)
					continue
				}

				recapData, err := agent.Extract(ctx, ch.ID, ch.Title, text)
				if err != nil {
					log.Error("[Worker %d] Failed to extract recap for %s: %v", workerID, ch.ID, err)
					continue
				}

				if err := store.Save(recapData); err != nil {
					log.Error("[Worker %d] Failed to save recap for %s: %v", workerID, ch.ID, err)
					continue
				}

				b, _ := json.MarshalIndent(recapData, "", "  ")
				log.Info("[Worker %d] Recap saved for %s:\n%s", workerID, ch.ID, string(b))
			}
		}(i)
	}

	for _, ch := range chapters {
		chapterChan <- ch
	}
	close(chapterChan)
	wg.Wait()

	log.Info("Recap generation completed")
	return nil
}

func loadFinalChapterContent(chapter *models.Chapter) string {
	root, err := findProjectRoot()
	if err != nil {
		return ""
	}

	// Try multiple filename formats
	// Format 1: chapter-{full_id}.md (e.g., chapter-P1-V1-C1.md)
	path1 := filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapter.ID))
	if data, err := os.ReadFile(path1); err == nil {
		return string(data)
	}

	// Format 2: chapter-{number}.md (e.g., chapter-1.md)
	chapterNum := extractChapterNumber(chapter.ID)
	path2 := filepath.Join(root, "chapters", fmt.Sprintf("chapter-%s.md", chapterNum))
	if data, err := os.ReadFile(path2); err == nil {
		return string(data)
	}

	return ""
}
