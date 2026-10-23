package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/platform/llm"
)

func TestComposeChapterBatchSizeUsesOutputBudget(t *testing.T) {
	if got := composeChapterBatchSize(25, 200000, 8000, 4000); got != 6 {
		t.Fatalf("8k output budget batch size = %d, want 6", got)
	}
	if got := composeChapterBatchSize(25, 200000, 32000, 4000); got != composeChapterBatchHardLimit {
		t.Fatalf("large output budget should still cap at %d, got %d", composeChapterBatchHardLimit, got)
	}
}

func TestComposeChapterBatchSizeUsesContextWindow(t *testing.T) {
	if got := composeChapterBatchSize(25, 6000, 8000, 5000); got != 1 {
		t.Fatalf("tight context window batch size = %d, want 1", got)
	}
	if got := composeChapterBatchSize(3, 200000, 8000, 1000); got != 3 {
		t.Fatalf("batch size must not exceed remaining target, got %d", got)
	}
}

type batchChapterClient struct {
	calls int
}

func (c *batchChapterClient) ChatCompletion(_ context.Context, messages []llm.Message, _ *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.calls++
	userPrompt := ""
	for _, message := range messages {
		if message.Role == "user" {
			userPrompt = message.Content
		}
	}
	rangeMatch := regexp.MustCompile(`generate chapters ([0-9]+)-([0-9]+)`).FindStringSubmatch(userPrompt)
	if len(rangeMatch) != 3 {
		return nil, fmt.Errorf("batch command missing chapter range: %s", userPrompt)
	}
	start, _ := strconv.Atoi(rangeMatch[1])
	end, _ := strconv.Atoi(rangeMatch[2])
	count := end - start + 1
	chapters := make([]models.Chapter, 0, count)
	for i := 0; i < count; i++ {
		number := start + i
		chapters = append(chapters, models.Chapter{
			Title:   fmt.Sprintf("Chapter %d", number),
			Summary: fmt.Sprintf("Chapter %d advances the plan.", number),
			Scenes: []models.OutlineScene{{
				Order:    1,
				POV:      "Lin",
				Goal:     "Advance",
				Beats:    []string{fmt.Sprintf("Beat %d", number)},
				Words:    700,
				Location: "Archive",
			}},
			Events: []models.Event{{
				Actor:      "Lin",
				Action:     models.ActionDiscover,
				Target:     "Clue",
				TargetType: models.TargetTypeItem,
			}},
		})
	}
	data, err := json.Marshal(ComposeChaptersOutput{Chapters: chapters})
	if err != nil {
		return nil, err
	}
	return &llm.ChatResponse{Content: string(data), Model: "test-model"}, nil
}

func TestGenerateChaptersForVolumeSplitsLongVolume(t *testing.T) {
	client := &batchChapterClient{}
	config := &llm.Config{
		DefaultProvider: "test",
		DefaultModel:    "test-model",
		Providers: map[string]*llm.ProviderConfig{
			"test": {
				Name: "test",
				Models: map[string]*llm.ModelConfig{
					"test-model": {Name: "test-model", Context: 200000, MaxTokens: 8000},
				},
			},
		},
	}
	agent := NewComposeAgent(client, config, &models.ProjectLLM{Provider: "test", Model: "test-model"})
	output, err := agent.GenerateChaptersForVolume(context.Background(), ComposeChaptersInput{
		Part:           models.Part{ID: "P1", Title: "Part"},
		Volume:         models.Volume{ID: "P1-V1", Title: "Volume"},
		VolumeIndex:    1,
		TotalVolumes:   1,
		ChaptersPerVol: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.calls <= 1 {
		t.Fatalf("25 chapters should be split across calls, got %d", client.calls)
	}
	if len(output.Chapters) != 25 {
		t.Fatalf("generated chapters = %d, want 25", len(output.Chapters))
	}
	for i, chapter := range output.Chapters {
		want := fmt.Sprintf("Chapter %d", i+1)
		if chapter.Title != want {
			t.Fatalf("chapter %d title = %q, want %q", i+1, chapter.Title, want)
		}
	}
}
