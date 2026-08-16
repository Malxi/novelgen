package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"novelgen/internal/llm"
)

type fakeScorerClient struct {
	content  string
	err      error
	system   string
	user     string
	messages []llm.Message
}

func (c *fakeScorerClient) ChatCompletion(ctx context.Context, messages []llm.Message, options *llm.ChatOptions) (*llm.ChatResponse, error) {
	c.messages = messages
	for _, m := range messages {
		if m.Role == "system" {
			c.system = m.Content
		}
		if m.Role == "user" {
			c.user = m.Content
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	return &llm.ChatResponse{Content: c.content, Model: "test", Usage: llm.Usage{TotalTokens: 12}}, nil
}

func TestChapterScorerAgentScoreChapterUsesChapterScoreSkill(t *testing.T) {
	client := &fakeScorerClient{content: `{"score": 87, "reason": "开篇钩子强，张力在线，结尾期待感足。"}`}
	agent := NewChapterScorerAgent(client, &llm.Config{}, nil)

	output, err := agent.ScoreChapter(context.Background(), ChapterScoreInput{
		ChapterID:      "P1-V1-C1",
		ChapterTitle:   "Opening",
		ChapterContent: "Lin opened the sealed door and saw blue light.",
	})
	if err != nil {
		t.Fatalf("ScoreChapter() error = %v", err)
	}
	if output.Score != 87 {
		t.Fatalf("score = %v, want 87", output.Score)
	}
	if !strings.Contains(output.Reason, "开篇钩子强") {
		t.Fatalf("reason = %q", output.Reason)
	}
	if !strings.Contains(client.system, "SKILL: chapter-score") {
		t.Fatalf("system prompt does not load chapter-score skill: %.120s", client.system)
	}
	if !strings.Contains(client.user, "Chapter Content") {
		t.Fatalf("user prompt does not carry chapter content: %.120s", client.user)
	}
	if len(client.messages) != 2 {
		t.Fatalf("messages = %d, want a single system+user invocation", len(client.messages))
	}
}

func TestChapterScorerAgentScoreChapterNormalizesScore(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want float64
	}{
		{name: "decimal score rounds", raw: `{"score": 87.5, "reason": "ok"}`, want: 88},
		{name: "0-10 scale converts to 0-100", raw: `{"score": 9, "reason": "ok"}`, want: 90},
		{name: "over 100 clamps to 100", raw: `{"score": 150, "reason": "ok"}`, want: 100},
		{name: "negative clamps to 0", raw: `{"score": -5, "reason": "ok"}`, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agent := NewChapterScorerAgent(&fakeScorerClient{content: tc.raw}, &llm.Config{}, nil)
			output, err := agent.ScoreChapter(context.Background(), ChapterScoreInput{
				ChapterID:      "P1-V1-C1",
				ChapterContent: "Some chapter content.",
			})
			if err != nil {
				t.Fatalf("ScoreChapter() error = %v", err)
			}
			if output.Score != tc.want {
				t.Fatalf("score = %v, want %v", output.Score, tc.want)
			}
		})
	}
}

func TestChapterScorerAgentScoreChapterClipsReason(t *testing.T) {
	longReason := strings.Repeat("理由", 200) // 400 runes
	agent := NewChapterScorerAgent(&fakeScorerClient{content: `{"score": 80, "reason": "` + longReason + `"}`}, &llm.Config{}, nil)
	output, err := agent.ScoreChapter(context.Background(), ChapterScoreInput{
		ChapterID:      "P1-V1-C1",
		ChapterContent: "Some chapter content.",
	})
	if err != nil {
		t.Fatalf("ScoreChapter() error = %v", err)
	}
	if got := len([]rune(output.Reason)); got > 300 {
		t.Fatalf("reason length = %d, want <= 300", got)
	}
}

func TestChapterScorerAgentScoreChapterRejectsEmptyContent(t *testing.T) {
	agent := NewChapterScorerAgent(&fakeScorerClient{content: `{"score": 80, "reason": "ok"}`}, &llm.Config{}, nil)
	if _, err := agent.ScoreChapter(context.Background(), ChapterScoreInput{
		ChapterID: "P1-V1-C1",
	}); err == nil {
		t.Fatalf("expected error for empty content")
	}
}

func TestChapterScorerAgentScoreChapterPropagatesClientError(t *testing.T) {
	agent := NewChapterScorerAgent(&fakeScorerClient{err: errors.New("boom")}, &llm.Config{}, nil)
	if _, err := agent.ScoreChapter(context.Background(), ChapterScoreInput{
		ChapterID:      "P1-V1-C1",
		ChapterContent: "Some chapter content.",
	}); err == nil {
		t.Fatalf("expected client error to propagate")
	}
}

func TestSelectBestScoredChapterPicksHighestScore(t *testing.T) {
	copies := []ScoredChapterCopy{
		{CopyIndex: 0, Content: "a", Score: 70},
		{CopyIndex: 1, Content: "b", Score: 92},
		{CopyIndex: 2, Content: "c", Score: 85},
	}
	best, err := SelectBestScoredChapter(copies)
	if err != nil {
		t.Fatalf("SelectBestScoredChapter() error = %v", err)
	}
	if best.CopyIndex != 1 || best.Content != "b" {
		t.Fatalf("best = %#v, want copy 1", best)
	}
}

func TestSelectBestScoredChapterTieBreaksToLowestCopyIndex(t *testing.T) {
	copies := []ScoredChapterCopy{
		{CopyIndex: 0, Content: "a", Score: 88},
		{CopyIndex: 1, Content: "b", Score: 88},
		{CopyIndex: 2, Content: "c", Score: 88},
	}
	best, err := SelectBestScoredChapter(copies)
	if err != nil {
		t.Fatalf("SelectBestScoredChapter() error = %v", err)
	}
	if best.CopyIndex != 0 {
		t.Fatalf("best.CopyIndex = %d, want 0 on tie", best.CopyIndex)
	}
}

func TestSelectBestScoredChapterRejectsEmptyList(t *testing.T) {
	if _, err := SelectBestScoredChapter(nil); err == nil {
		t.Fatalf("expected error for empty copy list")
	}
}
