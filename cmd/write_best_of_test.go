package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"novelgen/internal/agents"
	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/models"
)

type bestOfFakeClient struct {
	mu sync.Mutex

	contents       []string
	scoreByMarker  map[string]int
	failMarker     string
	failScoring    bool
	genCalls       int
	scoreCalls     int
	generationErrs int
}

func (c *bestOfFakeClient) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	var userPrompt string
	for _, m := range messages {
		if m.Role == "user" {
			userPrompt = m.Content
		}
	}

	if strings.Contains(userPrompt, "Chapter Content") {
		return c.scoringResponse(userPrompt)
	}
	return c.generationResponse()
}

func (c *bestOfFakeClient) generationResponse() (*llm.ChatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.contents) == 0 {
		return nil, errors.New("no more generated contents queued")
	}
	marker := c.contents[0]
	c.contents = c.contents[1:]
	c.genCalls++
	if c.failMarker != "" && marker == c.failMarker {
		c.generationErrs++
		return nil, errors.New("simulated generation failure")
	}
	content := marker + " Lin opened the sealed door and walked forward. The signal grew stronger with every step."
	return &llm.ChatResponse{
		Content: fmt.Sprintf(`{"content": %q}`, content),
		Model:   "test",
		Usage:   llm.Usage{TotalTokens: 12},
	}, nil
}

func (c *bestOfFakeClient) scoringResponse(userPrompt string) (*llm.ChatResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scoreCalls++
	if c.failScoring {
		return nil, errors.New("simulated scoring failure")
	}
	score := 0
	for marker, s := range c.scoreByMarker {
		if strings.Contains(userPrompt, marker) {
			score = s
			break
		}
	}
	if score == 0 {
		return nil, errors.New("scoring call could not identify the copy content")
	}
	return &llm.ChatResponse{
		Content: fmt.Sprintf(`{"score": %d, "reason": "reader experience holds up"}`, score),
		Model:   "test",
		Usage:   llm.Usage{TotalTokens: 12},
	}, nil
}

func newBestOfFixture(t *testing.T) (*bestOfFakeClient, *agents.WriteAgent, *agents.ChapterScorerAgent) {
	t.Helper()
	oldProjectDir := logger.Default().ProjectDir()
	logger.Default().SetProjectDir(t.TempDir())
	t.Cleanup(func() { logger.Default().SetProjectDir(oldProjectDir) })

	client := &bestOfFakeClient{
		contents:      []string{"COPY_1", "COPY_2", "COPY_3"},
		scoreByMarker: map[string]int{"COPY_1": 78, "COPY_2": 92, "COPY_3": 85},
	}
	agent := agents.NewWriteAgent(client, &llm.Config{}, nil, &models.StorySetup{Premise: "A sealed door hides a signal."}, nil)
	scorer := agents.NewChapterScorerAgent(client, &llm.Config{}, nil)
	return client, agent, scorer
}

func TestGenerateChapterBestOfSelectsHighestScoringCopy(t *testing.T) {
	client, agent, scorer := newBestOfFixture(t)
	chapter := &models.Chapter{
		ID:          "P1-V1-C1",
		Title:       "Opening",
		Summary:     "Lin opens the sealed door.",
		LegacyBeats: []string{"open door"},
	}
	chapterCtx := &agents.ChapterContext{
		Next: []*agents.ContextChapter{{
			Chapter: &models.Chapter{ID: "P1-V1-C2", Title: "Signal", Summary: "Lin follows the signal."},
		}},
	}

	content, err := generateChapterBestOf(context.Background(), agent, scorer, chapter, chapterCtx, nil, 0, 3, false, false)
	if err != nil {
		t.Fatalf("generateChapterBestOf() error = %v", err)
	}
	if !strings.Contains(content, "COPY_2") {
		t.Fatalf("selected content = %.60q, want COPY_2 (highest score)", content)
	}
	if client.genCalls != 3 {
		t.Fatalf("generation calls = %d, want 3", client.genCalls)
	}
	if client.scoreCalls != 3 {
		t.Fatalf("scoring calls = %d, want 3", client.scoreCalls)
	}
}

func TestGenerateChapterBestOfSkipsFailedCopy(t *testing.T) {
	client, agent, scorer := newBestOfFixture(t)
	client.failMarker = "COPY_2"
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin opens the sealed door."}

	content, err := generateChapterBestOf(context.Background(), agent, scorer, chapter, nil, nil, 0, 3, false, false)
	if err != nil {
		t.Fatalf("generateChapterBestOf() error = %v", err)
	}
	if !strings.Contains(content, "COPY_3") {
		t.Fatalf("selected content = %.60q, want COPY_3 (highest among surviving copies)", content)
	}
	if client.generationErrs != 1 {
		t.Fatalf("generation errors = %d, want 1", client.generationErrs)
	}
	if client.scoreCalls != 2 {
		t.Fatalf("scoring calls = %d, want 2 (only successful copies scored)", client.scoreCalls)
	}
}

func TestGenerateChapterBestOfFallsBackToFirstCopyWhenScoringFails(t *testing.T) {
	client, agent, scorer := newBestOfFixture(t)
	client.failScoring = true
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening", Summary: "Lin opens the sealed door."}

	content, err := generateChapterBestOf(context.Background(), agent, scorer, chapter, nil, nil, 0, 3, false, false)
	if err != nil {
		t.Fatalf("generateChapterBestOf() error = %v", err)
	}
	if !strings.Contains(content, "COPY_1") && !strings.Contains(content, "COPY_2") && !strings.Contains(content, "COPY_3") {
		t.Fatalf("fallback content = %.60q, want one of the successfully generated copies", content)
	}
	if client.scoreCalls != 3 {
		t.Fatalf("scoring calls = %d, want 3 attempts before fallback", client.scoreCalls)
	}
}

func TestGenerateChapterBestOfErrorsWhenAllCopiesFail(t *testing.T) {
	client, agent, scorer := newBestOfFixture(t)
	client.failMarker = "COPY_1"
	client.contents = []string{"COPY_1"}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}

	if _, err := generateChapterBestOf(context.Background(), agent, scorer, chapter, nil, nil, 0, 1, false, false); err == nil {
		t.Fatalf("expected error when the only copy fails")
	}
}

func TestGenerateChapterContentForRunSinglePathSkipsScoring(t *testing.T) {
	client, agent, scorer := newBestOfFixture(t)
	client.contents = []string{"COPY_1"}
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "Opening"}

	content, err := generateChapterContentForRun(context.Background(), agent, scorer, chapter, nil, nil, 0, 1, false, false)
	if err != nil {
		t.Fatalf("generateChapterContentForRun() error = %v", err)
	}
	if !strings.Contains(content, "COPY_1") {
		t.Fatalf("content = %.60q", content)
	}
	if client.scoreCalls != 0 {
		t.Fatalf("scoring calls = %d, want 0 when best-of is disabled", client.scoreCalls)
	}
}

func TestValidateWriteBestOfFlag(t *testing.T) {
	for _, n := range []int{0, 1, 3, 5} {
		writeBestOfFlag = n
		if err := validateWriteBestOfFlag(); err != nil {
			t.Fatalf("validateWriteBestOfFlag(%d) error = %v", n, err)
		}
	}
	for _, n := range []int{-1, 6, 10} {
		writeBestOfFlag = n
		if err := validateWriteBestOfFlag(); err == nil {
			t.Fatalf("validateWriteBestOfFlag(%d) expected error", n)
		}
	}
}

func TestBuildChapterScoreInput(t *testing.T) {
	chapter := &models.Chapter{
		ID:          "P1-V1-C1",
		Title:       "Opening",
		Summary:     "Lin opens the sealed door.",
		LegacyBeats: []string{"open door", "find signal"},
	}
	chapterCtx := &agents.ChapterContext{
		Next: []*agents.ContextChapter{{
			Chapter: &models.Chapter{ID: "P1-V1-C2", Title: "Signal", Summary: "Lin follows the signal."},
		}},
	}
	input := buildChapterScoreInput(chapter, "chapter body", chapterCtx)
	if input.ChapterID != "P1-V1-C1" || input.ChapterTitle != "Opening" {
		t.Fatalf("identity fields = %#v", input)
	}
	if input.ChapterContent != "chapter body" {
		t.Fatalf("ChapterContent = %q", input.ChapterContent)
	}
	if input.ChapterSummary != "Lin opens the sealed door." {
		t.Fatalf("ChapterSummary = %q", input.ChapterSummary)
	}
	if len(input.ChapterBeats) != 2 || input.ChapterBeats[0] != "open door" {
		t.Fatalf("ChapterBeats = %#v", input.ChapterBeats)
	}
	if input.NextChapterSummary != "Lin follows the signal." {
		t.Fatalf("NextChapterSummary = %q", input.NextChapterSummary)
	}
}
