package toolplane

import (
	"context"
	"fmt"
	"strings"

	"novelgen/internal/fiction/domain/memory"
	"novelgen/internal/platform/llm"
	"novelgen/internal/platform/store"
)

type memoryQueryOptions struct {
	ChapterID string
	Intent    string
	Focus     string
	MaxChars  int
	Limit     int
}

var (
	memoryConfigPathForRoot = llm.ConfigPathForRoot
	loadMemoryLLMConfig     = llm.LoadConfig
	newMemoryEmbedder       = llm.NewOpenAIEmbedder
)

func queryNarrativeMemory(root *store.Root, opts memoryQueryOptions) (*memory.ContextResult, []string, error) {
	embedder, model, warning := configuredMemoryEmbedder(root)
	warnings := []string{}
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if model != "" && embedder == nil {
		warnings = append(warnings, "embedding model is configured but the embedding client is unavailable")
	}
	result, err := memory.Query(context.Background(), root, memory.QueryOptions{
		ChapterID: opts.ChapterID,
		Intent:    opts.Intent,
		Focus:     opts.Focus,
		MaxChars:  opts.MaxChars,
		Limit:     opts.Limit,
		Embedder:  embedder,
	})
	if err != nil {
		return nil, warnings, err
	}
	result.Warnings = append(warnings, result.Warnings...)
	return result, warnings, nil
}

func refreshNarrativeMemory(root *store.Root) (*memory.RefreshResult, error) {
	embedder, _, warning := configuredMemoryEmbedder(root)
	result, err := memory.Refresh(context.Background(), root, embedder)
	if err != nil {
		return nil, err
	}
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	return result, nil
}

func configuredMemoryEmbedder(root *store.Root) (memory.Embedder, string, string) {
	if root == nil || root.Dir() == "" {
		return nil, "", "narrative memory vectors disabled: project root is unavailable"
	}
	projectConfig, err := root.LoadConfig()
	if err != nil {
		return nil, "", "narrative memory vectors disabled: cannot load novel.json: " + err.Error()
	}
	configPath := memoryConfigPathForRoot(root)
	if strings.TrimSpace(configPath) == "" {
		return nil, "", ""
	}
	config, err := loadMemoryLLMConfig(configPath)
	if err != nil {
		return nil, "", "narrative memory vectors disabled: cannot load llm_config.json: " + err.Error()
	}
	provider, _ := config.GetActiveModel(&projectConfig.LLM)
	if provider == nil {
		return nil, "", ""
	}
	model := strings.TrimSpace(provider.EmbeddingModel)
	if model == "" {
		return nil, "", ""
	}
	embedder, err := newMemoryEmbedder(provider, model)
	if err != nil {
		return nil, model, "narrative memory vectors disabled: " + err.Error()
	}
	return embedder, model, ""
}

func NarrativeMemoryModel(root *store.Root) string {
	_, model, _ := configuredMemoryEmbedder(root)
	return model
}

func memoryToolResponse(result *memory.ContextResult, warnings []string) ToolResponse {
	if result == nil {
		return ToolResponse{OK: false, Section: "context", Count: 0, Warnings: warnings}
	}
	allWarnings := append(append([]string{}, warnings...), result.Warnings...)
	return ToolResponse{
		OK:       true,
		Section:  "context",
		Count:    len(result.Retrieved) + len(result.Promises),
		Results:  *result,
		Warnings: uniqueWarningStrings(allWarnings),
		Meta: map[string]interface{}{
			"narrative_memory": result.Meta,
		},
	}
}

func uniqueWarningStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func memoryUnavailableWarning(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("narrative memory unavailable: %v", err)
}
