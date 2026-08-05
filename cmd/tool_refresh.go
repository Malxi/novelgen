package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
	rpgdsl "novelgen/internal/rpg/dsl"

	"github.com/spf13/cobra"
)

var toolRefreshFlags struct {
	ID        string
	BatchSize int
}

type toolRefreshResult struct {
	OK               bool             `json:"ok"`
	Target           string           `json:"target"`
	ID               string           `json:"id,omitempty"`
	DSLPath          string           `json:"dsl_path,omitempty"`
	CacheDir         string           `json:"cache_dir,omitempty"`
	ChapterCount     int              `json:"chapter_count,omitempty"`
	BatchCount       int              `json:"batch_count,omitempty"`
	ReusedBatches    int              `json:"reused_batches,omitempty"`
	GeneratedBatches int              `json:"generated_batches,omitempty"`
	Check            *toolCheckResult `json:"check,omitempty"`
	Meta             map[string]any   `json:"meta,omitempty"`
}

func runToolRefresh(cmd *cobra.Command, args []string) error {
	target := normalizeKey(args[0])
	switch target {
	case "chapter-dsl", "chapters-dsl", "chapter-rpg-dsl", "rpg-chapters":
		return runToolRefreshChapterDSL(cmd)
	default:
		return fmt.Errorf("unsupported refresh target %q", target)
	}
}

func runToolRefreshChapterDSL(cmd *cobra.Command) error {
	restoreLogger := suppressToolRefreshLogs()
	defer restoreLogger()

	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	id := strings.TrimSpace(toolRefreshFlags.ID)
	if id == "" {
		return fmt.Errorf("--id is required for chapter-dsl refresh")
	}
	refresh, err := refreshToolChapterDSL(cmd.Context(), root, id, toolRefreshFlags.BatchSize)
	if err != nil {
		return err
	}
	return writeJSON(cmd, refresh)
}

func suppressToolRefreshLogs() func() {
	previous := logger.Default()
	logger.SetDefault(logger.New(logger.ErrorLevel))
	return func() {
		logger.SetDefault(previous)
	}
}

func refreshToolChapterDSL(ctx context.Context, root, id string, batchSize int) (*toolRefreshResult, error) {
	config, setup, outline, cfg, client, err := loadToolRefreshProjectState()
	if err != nil {
		return nil, err
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found in outline", id)
	}
	chapterPath := firstExistingPath(candidateChapterMarkdownPaths(root, id))
	if chapterPath == "" {
		return nil, fmt.Errorf("chapter markdown for %q not found under %s", id, filepath.Join(root, "chapters"))
	}
	if batchSize <= 0 {
		batchSize = 10
	}

	result, err := emitChapterRPGDSLForProject(ctx, root, filepath.Base(root), config, cfg, setup, client, batchSize, id)
	if err != nil {
		return nil, err
	}
	gate := runToolChapterSimulationGate(root, id)
	check := makeToolCheckResult("simulation", "chapter", "chapter", id, gate)
	refresh := toolRefreshResult{
		OK:               check == nil || !check.Blocking,
		Target:           "chapter-dsl",
		ID:               id,
		DSLPath:          result.DSLPath,
		CacheDir:         result.CacheDir,
		ChapterCount:     result.ChapterCount,
		BatchCount:       result.BatchCount,
		ReusedBatches:    result.ReusedBatches,
		GeneratedBatches: result.GeneratedBatches,
		Check:            check,
		Meta: map[string]any{
			"project_root":      root,
			"chapter_path":      chapterPath,
			"post_check_query":  fmt.Sprintf("novelgen tool check simulation --target chapter --scope chapter --id %q --min-priority low --max-issues 12", id),
			"refresh_scope":     "target chapter cache plus combined 04_chapters.rpg chapter replacement",
			"derived_artifacts": []string{"story/rpg/04_chapters.rpg", "story/rpg/cache/chapter_batches"},
		},
	}
	return &refresh, nil
}

func loadToolRefreshProjectState() (*models.ProjectConfig, *models.StorySetup, *models.Outline, *llm.Config, llm.Client, error) {
	config, err := loadProjectConfig()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load project config: %w", err)
	}
	setup, err := loadStorySetup()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load story setup: %w", err)
	}
	outline, err := loadOutline()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load outline: %w", err)
	}
	cfg, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load LLM config: %w", err)
	}
	client := cfg.CreateClient(&config.LLM)
	if client == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to create LLM client")
	}
	return config, setup, outline, cfg, client, nil
}

func makeToolRefreshChapterDSLCheck(chapterID string, issues []rpgdsl.SimulationIssue) *toolCheckResult {
	chapterIssues := rpgdsl.NewSimulationBridge().IssuesForChapter(issues, chapterID)
	gate := qualityGateResult{}
	gate.add(rpgdsl.NewSimulationBridge().ConvertIssuesToSuggestions(chapterIssues)...)
	gate.dedup()
	gate.Blocking = hasBlockingSuggestions(gate.Suggestions)
	return makeToolCheckResult("simulation", "chapter", "chapter", chapterID, gate)
}

func firstExistingPath(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
