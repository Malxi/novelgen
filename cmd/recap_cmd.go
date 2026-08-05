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
	recapAgentSDKFlag    bool
	recapAgentApplyFlag  bool
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
  novelgen recap gen --source chapters
  novelgen recap gen --agent-sdk --chapter 1
  novelgen recap gen --agent-sdk --agent-apply --chapter 1`,
	RunE: runRecapGen,
}

func init() {
	recapCmd.AddCommand(recapGenCmd)

	recapGenCmd.Flags().StringVar(&recapChapterFlag, "chapter", "", "Chapter number(s) to recap (e.g., '1', '1-5', or 'P1-V1-C1')")
	recapGenCmd.Flags().BoolVar(&recapAllFlag, "all", false, "Generate recaps for all chapters")
	recapGenCmd.Flags().StringVar(&recapSourceFlag, "source", "chapters", "Source text: chapters|drafts (drafts is legacy)")
	recapGenCmd.Flags().IntVar(&recapConcurrencyFlag, "concurrency", 1, "Number of concurrent recap generations")
	recapGenCmd.Flags().BoolVar(&recapAgentSDKFlag, "agent-sdk", false, "Use Claude Agent SDK workflow for recap extraction")
	recapGenCmd.Flags().BoolVar(&recapAgentApplyFlag, "agent-apply", false, "With --agent-sdk, let the agent save recap JSON through validated patch tools")

	// Register recap command using the new plugin mechanism
	RegisterCommand(func() *cobra.Command {
		return recapCmd
	})
}

func runRecapGen(cmd *cobra.Command, args []string) error {
	log := logger.GetLogger()
	ctx := cmd.Context()

	if err := validateRecapAgentApplyOption(recapAgentSDKFlag, recapAgentApplyFlag); err != nil {
		return err
	}

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

	concurrency := effectiveRecapConcurrency(recapConcurrencyFlag, len(chapters), recapAgentSDKFlag)
	if recapAgentSDKFlag && recapConcurrencyFlag > 1 {
		log.Warn("--agent-sdk recap extraction runs sequentially for clearer live logs; ignoring --concurrency=%d", recapConcurrencyFlag)
	}

	log.Info("Generating recaps for %d chapter(s) from %s with concurrency %d", len(chapters), src, concurrency)
	if recapAgentSDKFlag {
		log.Info("Using Agent SDK recap workflow; Go will validate and save recap JSON")
		if recapAgentApplyFlag {
			log.Info("Agent apply enabled: SDK may write recap JSON through validated patch tools")
		}
	}

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

				var recapData *models.ChapterRecap
				var err error
				var beforeRecap *models.ChapterRecap
				if recapAgentSDKFlag && recapAgentApplyFlag {
					beforeRecap, _ = store.Load(ch.ID)
				}
				if recapAgentSDKFlag {
					recapData, err = agent.ExtractWithAgentSDKApply(ctx, ch.ID, ch.Title, text, recapAgentApplyFlag)
					if err != nil {
						if recovered, ok := recoverAgentAppliedRecapAfterError(store, ch.ID, ch.Title, beforeRecap, recapAgentApplyFlag); ok {
							log.Warn("[Worker %d] Agent SDK returned an error after applying recap patch; recovering saved recap for %s: %v", workerID, ch.ID, err)
							recapData = recovered
							err = nil
						}
					}
				} else {
					recapData, err = agent.Extract(ctx, ch.ID, ch.Title, text)
				}
				if err != nil {
					log.Error("[Worker %d] Failed to extract recap for %s: %v", workerID, ch.ID, err)
					continue
				}
				agentApplied := false
				if recapAgentSDKFlag && recapAgentApplyFlag {
					recapData, agentApplied = resolveAgentAppliedRecapData(log, workerID, store, ch.ID, beforeRecap, recapData)
					if !agentApplied {
						log.Warn("[Worker %d] Agent SDK did not apply a recap patch for %s; leaving saved recap unchanged", workerID, ch.ID)
						continue
					}
				}

				if !agentApplied {
					if err := store.Save(recapData); err != nil {
						log.Error("[Worker %d] Failed to save recap for %s: %v", workerID, ch.ID, err)
						continue
					}
				}

				b, _ := json.MarshalIndent(recapData, "", "  ")
				if agentApplied {
					log.Info("[Worker %d] Recap already saved by agent patch for %s:\n%s", workerID, ch.ID, string(b))
				} else {
					log.Info("[Worker %d] Recap saved for %s:\n%s", workerID, ch.ID, string(b))
				}
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

func effectiveRecapConcurrency(requested, chapterCount int, agentSDK bool) int {
	concurrency := requested
	if concurrency <= 0 {
		concurrency = 1
	}
	if agentSDK {
		return 1
	}
	if concurrency > chapterCount {
		concurrency = chapterCount
	}
	return concurrency
}

func validateRecapAgentApplyOption(agentSDK, agentApply bool) error {
	if agentApply && !agentSDK {
		return fmt.Errorf("--agent-apply requires --agent-sdk")
	}
	return nil
}

func recoverAgentAppliedRecapAfterError(store *recap.Store, chapterID, title string, before *models.ChapterRecap, agentApply bool) (*models.ChapterRecap, bool) {
	if !agentApply || store == nil {
		return nil, false
	}
	saved, err := store.Load(chapterID)
	if err != nil || saved == nil {
		return nil, false
	}
	normalizeRecapForCommand(saved, chapterID, title)
	if recapJSONEqual(before, saved) {
		return nil, false
	}
	if ok, reasons := recap.ValidateMinimal(saved); !ok {
		logger.GetLogger().Warn("Agent SDK saved recap exists but is not recoverable: %s", strings.Join(reasons, "; "))
		return nil, false
	}
	return saved, true
}

func resolveAgentAppliedRecapData(log logger.LoggerInterface, workerID int, store *recap.Store, chapterID string, before *models.ChapterRecap, returned *models.ChapterRecap) (*models.ChapterRecap, bool) {
	if store == nil || strings.TrimSpace(chapterID) == "" {
		return returned, false
	}
	saved, err := store.Load(chapterID)
	if err != nil || saved == nil || recapJSONEqual(before, saved) {
		return saved, false
	}
	if ok, reasons := recap.ValidateMinimal(saved); !ok {
		log.Warn("[Worker %d] Agent applied recap for %s but saved recap fails minimal validation: %s", workerID, chapterID, strings.Join(reasons, "; "))
		return returned, false
	}
	if returned == nil || !recapJSONEqual(saved, returned) {
		log.Info("[Worker %d] Agent apply changed recap %s; using saved patch result instead of returned JSON", workerID, chapterID)
	}
	return saved, true
}

func normalizeRecapForCommand(r *models.ChapterRecap, chapterID, title string) {
	if r == nil {
		return
	}
	r.ChapterID = strings.TrimSpace(chapterID)
	if strings.TrimSpace(title) != "" {
		r.Title = strings.TrimSpace(title)
	}
}

func recapJSONEqual(a, b *models.ChapterRecap) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ab) == string(bb)
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
