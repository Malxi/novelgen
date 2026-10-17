package refresh

import (
	"context"
	"fmt"
	"path/filepath"

	"novelgen/internal/contract/story"
	"novelgen/internal/contract/toolcmd"
	"novelgen/internal/fiction/derive"
	"novelgen/internal/fiction/domain/quality"
	rpgdsl "novelgen/internal/fiction/rpg/dsl"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/logger"
	"novelgen/internal/platform/store"
)

type Result struct {
	OK               bool             `json:"ok"`
	Target           string           `json:"target"`
	ID               string           `json:"id,omitempty"`
	DSLPath          string           `json:"dsl_path,omitempty"`
	CacheDir         string           `json:"cache_dir,omitempty"`
	ChapterCount     int              `json:"chapter_count,omitempty"`
	BatchCount       int              `json:"batch_count,omitempty"`
	ReusedBatches    int              `json:"reused_batches,omitempty"`
	GeneratedBatches int              `json:"generated_batches,omitempty"`
	Check            *ToolCheckResult `json:"check,omitempty"`
	Meta             map[string]any   `json:"meta,omitempty"`
}

func SuppressLogs() func() {
	previous := logger.Default()
	logger.SetDefault(logger.New(logger.ErrorLevel))
	return func() {
		logger.SetDefault(previous)
	}
}

// RefreshToolChapterDSL refreshes the chapter RPG-DSL through an explicit
// project root, so an in-memory or remote backend is not downgraded to OSFS.
func RefreshChapterDSL(ctx context.Context, root *store.Root, id string, batchSize int) (*Result, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	if err := store.ValidateKeyPart(id); err != nil {
		return nil, fmt.Errorf("invalid chapter id %q: %w", id, err)
	}
	config, setup, outline, cfg, client, err := loadToolRefreshProjectState(root)
	if err != nil {
		return nil, err
	}
	chapter := outline.GetChapterByID(id)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found in outline", id)
	}
	chapterPath := FirstExistingPath(root.FS(), root.ChapterCandidates(id))
	if chapterPath == "" {
		return nil, fmt.Errorf("chapter markdown for %q not found under %s", id, root.ChaptersDir())
	}
	if batchSize <= 0 {
		batchSize = 10
	}

	result, err := derive.EmitChapterRPGDSLForProject(ctx, root, filepath.Base(root.Dir()), config, cfg, setup, client, batchSize, id)
	if err != nil {
		return nil, err
	}
	gate := RunToolChapterSimulationGateAt(root, id)
	check := MakeToolCheckResult("simulation", "chapter", "chapter", id, gate)
	refresh := Result{
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
			"project_root":      root.Dir(),
			"chapter_path":      chapterPath,
			"post_check_query":  toolcmd.ChapterCheck(id, toolcmd.ChapterCheckOptions{Kind: "simulation", MinPriority: "low", MaxIssues: 12}),
			"refresh_scope":     "target chapter cache plus combined 04_chapters.rpg chapter replacement",
			"derived_artifacts": []string{root.RelPath(root.RPGChaptersDSL()), root.RelPath(root.RPGChapterBatchCacheDir())},
		},
	}
	return &refresh, nil
}

func loadToolRefreshProjectState(root *store.Root) (*models.ProjectConfig, *models.StorySetup, *models.Outline, *llm.Config, llm.Client, error) {
	if root == nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("project root is required")
	}
	config, err := root.LoadConfig()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load project config: %w", err)
	}
	setup, err := root.LoadSetup()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("load story setup: %w", err)
	}
	outline, err := root.LoadOutline()
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

func MakeChapterDSLCheck(chapterID string, issues []rpgdsl.SimulationIssue) *ToolCheckResult {
	chapterIssues := rpgdsl.NewSimulationBridge().IssuesForChapter(issues, chapterID)
	gate := quality.QualityGateResult{}
	gate.Add(rpgdsl.NewSimulationBridge().ConvertIssuesToSuggestions(chapterIssues)...)
	gate.Dedup()
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return MakeToolCheckResult("simulation", "chapter", "chapter", chapterID, gate)
}

// FirstExistingPath returns the first candidate that exists on the project's
// filesystem, so chapter-name fallbacks are probed through the store.
func FirstExistingPath(fs store.FS, paths []string) string {
	for _, path := range paths {
		if _, err := fs.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
