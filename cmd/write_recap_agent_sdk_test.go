package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
)

type fakeWriteRecapExtractor struct {
	extractCalls        int
	extractSDKCalls     int
	feedbackCalls       int
	feedbackSDKCalls    int
	first               *models.ChapterRecap
	feedback            *models.ChapterRecap
	lastFeedback        string
	lastChapterID       string
	lastChapterTitle    string
	lastChapterText     string
	lastFeedbackContent string
}

func (f *fakeWriteRecapExtractor) Extract(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	f.extractCalls++
	f.record(chapterID, title, chapterText)
	return f.first, nil
}

func (f *fakeWriteRecapExtractor) ExtractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	f.feedbackCalls++
	f.lastFeedback = feedback
	f.record(chapterID, title, chapterText)
	return f.feedback, nil
}

func (f *fakeWriteRecapExtractor) ExtractWithAgentSDK(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	f.extractSDKCalls++
	f.record(chapterID, title, chapterText)
	return f.first, nil
}

func (f *fakeWriteRecapExtractor) ExtractWithFeedbackAgentSDK(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	f.feedbackSDKCalls++
	f.lastFeedback = feedback
	f.record(chapterID, title, chapterText)
	return f.feedback, nil
}

func (f *fakeWriteRecapExtractor) record(chapterID, title, chapterText string) {
	f.lastChapterID = chapterID
	f.lastChapterTitle = title
	f.lastChapterText = chapterText
	f.lastFeedbackContent = chapterText
}

func TestExtractAndSaveRecapWithGateUsesAgentSDKWhenEnabled(t *testing.T) {
	root := t.TempDir()
	extractor := &fakeWriteRecapExtractor{
		first: validTestChapterRecap("P1-V1-C1", "醒来"),
	}
	store := recap.NewStore(root)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "醒来"}

	if err := extractAndSaveRecapWithGate(context.Background(), extractor, store, chapter, "正文", 0, true); err != nil {
		t.Fatal(err)
	}
	if extractor.extractSDKCalls != 1 || extractor.extractCalls != 0 || extractor.feedbackSDKCalls != 0 {
		t.Fatalf("unexpected extractor calls: ordinary=%d sdk=%d feedback=%d feedbackSDK=%d",
			extractor.extractCalls, extractor.extractSDKCalls, extractor.feedbackCalls, extractor.feedbackSDKCalls)
	}
	if _, err := store.Load(chapter.ID); err != nil {
		t.Fatalf("recap was not saved under %s: %v", filepath.Join(root, "story", "recaps", chapter.ID+".json"), err)
	}
}

func TestExtractAndSaveRecapWithGateRetriesWithAgentSDKFeedback(t *testing.T) {
	root := t.TempDir()
	extractor := &fakeWriteRecapExtractor{
		first: &models.ChapterRecap{
			ChapterID: "P1-V1-C1",
			Title:     "醒来",
		},
		feedback: validTestChapterRecap("P1-V1-C1", "醒来"),
	}
	store := recap.NewStore(root)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "醒来"}

	if err := extractAndSaveRecapWithGate(context.Background(), extractor, store, chapter, "正文", 0, true); err != nil {
		t.Fatal(err)
	}
	if extractor.extractSDKCalls != 1 || extractor.feedbackSDKCalls != 1 || extractor.feedbackCalls != 0 {
		t.Fatalf("unexpected retry calls: ordinary=%d sdk=%d feedback=%d feedbackSDK=%d",
			extractor.extractCalls, extractor.extractSDKCalls, extractor.feedbackCalls, extractor.feedbackSDKCalls)
	}
	if extractor.lastFeedback == "" {
		t.Fatalf("expected deterministic feedback to be sent to SDK retry")
	}
}

func TestExtractAndSaveRecapWithGateUsesOrdinaryExtractorByDefault(t *testing.T) {
	root := t.TempDir()
	extractor := &fakeWriteRecapExtractor{
		first: validTestChapterRecap("P1-V1-C1", "醒来"),
	}
	store := recap.NewStore(root)
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "醒来"}

	if err := extractAndSaveRecapWithGate(context.Background(), extractor, store, chapter, "正文", 0, false); err != nil {
		t.Fatal(err)
	}
	if extractor.extractCalls != 1 || extractor.extractSDKCalls != 0 {
		t.Fatalf("unexpected extractor calls: ordinary=%d sdk=%d", extractor.extractCalls, extractor.extractSDKCalls)
	}
}

func validTestChapterRecap(chapterID, title string) *models.ChapterRecap {
	return &models.ChapterRecap{
		ChapterID:       chapterID,
		Title:           title,
		Location:        "残骸",
		Time:            "同夜",
		Present:         []string{"林砚"},
		PlotBeats:       []string{"林砚醒来。"},
		LastLine:        "他推开舱门。",
		NextOpeningHint: "他推开舱门后，蓝色火光照进残骸。",
	}
}
