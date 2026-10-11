// Package rpgllm holds the LLM-backed RPG extraction adapters. It sits above
// both internal/rpg and internal/llm so the deterministic rpg engine stays
// free of LLM dependencies.
package rpgllm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

// NovelgenLLMClient 适配 novelgen 的 LLM client
type NovelgenLLMClient struct {
	client  llm.Client
	options *llm.ChatOptions
}

// NewNovelgenLLMClient 创建基于 novelgen LLM client 的提取器
func NewNovelgenLLMClient(client llm.Client, options *llm.ChatOptions) *rpg.LLMExtractor {
	llmClient := &NovelgenLLMClient{
		client:  client,
		options: options,
	}
	return rpg.NewLLMExtractor(llmClient)
}

// Complete 实现 rpg.LLMClient 接口
func (c *NovelgenLLMClient) Complete(prompt string, text string) (string, error) {
	messages := []llm.Message{
		{
			Role:    "system",
			Content: prompt,
		},
		{
			Role:    "user",
			Content: text,
		},
	}

	resp, err := c.client.ChatCompletion(context.Background(), messages, c.options)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	return resp.Content, nil
}

// ExtractWithNovelgenClient 使用 novelgen LLM client 提取小说实体
func ExtractWithNovelgenClient(client llm.Client, options *llm.ChatOptions, text string) (*rpg.NovelRPGData, error) {
	extractor := NewNovelgenLLMClient(client, options)
	return extractor.ExtractFromNovel(text)
}

// ExtractWithConfig 使用配置文件创建 client 并提取
func ExtractWithConfig(config *llm.Config, projectLLM *models.ProjectLLM, text string) (*rpg.NovelRPGData, error) {
	client := config.CreateClient(projectLLM)
	if client == nil {
		return nil, fmt.Errorf("failed to create LLM client")
	}

	options := config.GetChatOptions(projectLLM)
	return ExtractWithNovelgenClient(client, options, text)
}

// SimpleExtract 简单提取（使用默认配置）
func SimpleExtract(text string) (*rpg.NovelRPGData, error) {
	config, err := llm.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 使用默认配置
	defaultProjectLLM := &models.ProjectLLM{
		Provider: config.DefaultProvider,
		Model:    config.DefaultModel,
	}

	return ExtractWithConfig(config, defaultProjectLLM, text)
}

// LLMExtractResult 结构化的 LLM 提取结果
type LLMExtractResult struct {
	Success   bool              `json:"success"`
	Data      *rpg.NovelRPGData `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
	TokenUsed int               `json:"token_used"`
	Model     string            `json:"model"`
}

// ExtractWithDetails 带详细信息的提取
func ExtractWithDetails(client llm.Client, options *llm.ChatOptions, text string) (*LLMExtractResult, error) {
	result := &LLMExtractResult{
		Success: false,
	}

	// 记录 token 使用（需要在 client 中实现）
	messages := []llm.Message{
		{
			Role:    "system",
			Content: rpg.BuildExtractionPrompt(),
		},
		{
			Role:    "user",
			Content: text,
		},
	}

	resp, err := client.ChatCompletion(context.Background(), messages, options)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Model = resp.Model
	result.TokenUsed = resp.Usage.TotalTokens

	// 解析响应
	var extractionResult rpg.LLMExtractionResult
	jsonStr := rpg.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &extractionResult); err != nil {
		result.Error = fmt.Sprintf("failed to parse LLM response: %v", err)
		return result, err
	}

	// 转换为 rpg.NovelRPGData
	extractor := rpg.NewLLMExtractor(&NovelgenLLMClient{client: client, options: options})
	data := extractor.ConvertToSystemFormat(extractionResult)

	result.Success = true
	result.Data = data

	return result, nil
}

// BatchExtract 批量提取多个文本片段
func BatchExtract(client llm.Client, options *llm.ChatOptions, texts []string) ([]*rpg.NovelRPGData, error) {
	var results []*rpg.NovelRPGData

	for i, text := range texts {
		fmt.Printf("Processing chunk %d/%d...\n", i+1, len(texts))

		data, err := ExtractWithNovelgenClient(client, options, text)
		if err != nil {
			return nil, fmt.Errorf("failed to extract chunk %d: %w", i+1, err)
		}

		results = append(results, data)
	}

	return results, nil
}

// MergeExtractResults 合并多个提取结果
func MergeExtractResults(results []*rpg.NovelRPGData) *rpg.NovelRPGData {
	merged := &rpg.NovelRPGData{
		Characters:       make([]*rpg.CharacterTemplate, 0),
		Items:            make([]*rpg.Item, 0),
		Skills:           make([]*rpg.Skill, 0),
		Locations:        make([]*rpg.Map, 0),
		Events:           make([]*rpg.Event, 0),
		Quests:           make([]*rpg.Quest, 0),
		Timeline:         make([]*rpg.TimelineEvent, 0),
		ValidationIssues: make([]rpg.ValidationIssue, 0),
	}

	// 使用 map 去重
	charMap := make(map[string]*rpg.CharacterTemplate)
	itemMap := make(map[string]*rpg.Item)
	skillMap := make(map[string]*rpg.Skill)
	locMap := make(map[string]*rpg.Map)
	eventMap := make(map[string]*rpg.Event)

	for _, result := range results {
		for _, char := range result.Characters {
			charMap[char.Name] = char
		}
		for _, item := range result.Items {
			itemMap[item.Name] = item
		}
		for _, skill := range result.Skills {
			skillMap[skill.Name] = skill
		}
		for _, loc := range result.Locations {
			locMap[loc.Name] = loc
		}
		for _, event := range result.Events {
			eventMap[event.Name] = event
		}
		merged.Timeline = append(merged.Timeline, result.Timeline...)
		merged.ValidationIssues = append(merged.ValidationIssues, result.ValidationIssues...)
	}

	// 转换回 slice
	for _, char := range charMap {
		merged.Characters = append(merged.Characters, char)
	}
	for _, item := range itemMap {
		merged.Items = append(merged.Items, item)
	}
	for _, skill := range skillMap {
		merged.Skills = append(merged.Skills, skill)
	}
	for _, loc := range locMap {
		merged.Locations = append(merged.Locations, loc)
	}
	for _, event := range eventMap {
		merged.Events = append(merged.Events, event)
	}

	return merged
}

// ExtractLargeText 处理大文本（自动分块）
func ExtractLargeText(client llm.Client, options *llm.ChatOptions, text string, chunkSize int) (*rpg.NovelRPGData, error) {
	// 分块
	chunks := splitTextIntoChunks(text, chunkSize)

	// 批量提取
	results, err := BatchExtract(client, options, chunks)
	if err != nil {
		return nil, err
	}

	// 合并结果
	return MergeExtractResults(results), nil
}

// splitTextIntoChunks 将文本分割成块
func splitTextIntoChunks(text string, chunkSize int) []string {
	var chunks []string
	lines := strings.Split(text, "\n")

	var currentChunk strings.Builder
	for _, line := range lines {
		if currentChunk.Len()+len(line) > chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}
