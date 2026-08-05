package cmd

import (
	"strings"
	"testing"

	"novelgen/internal/agents"
	"novelgen/internal/models"
)

func TestValidateWriteAgentApplyOptionRequiresAgentSDK(t *testing.T) {
	if err := validateWriteAgentApplyOption(true, true); err != nil {
		t.Fatalf("expected --agent-sdk --agent-apply to pass, got %v", err)
	}
	if err := validateWriteAgentApplyOption(false, false); err != nil {
		t.Fatalf("expected no agent flags to pass, got %v", err)
	}
	err := validateWriteAgentApplyOption(false, true)
	if err == nil || !strings.Contains(err.Error(), "--agent-apply requires --agent-sdk") {
		t.Fatalf("expected --agent-apply validation error, got %v", err)
	}
}

func TestValidateWriteAgentHistoryOptionRequiresAgentSDK(t *testing.T) {
	if err := validateWriteAgentHistoryOption(true, true); err != nil {
		t.Fatalf("expected --agent-sdk --agent-history to pass, got %v", err)
	}
	if err := validateWriteAgentHistoryOption(false, false); err != nil {
		t.Fatalf("expected no agent history flag to pass, got %v", err)
	}
	err := validateWriteAgentHistoryOption(false, true)
	if err == nil || !strings.Contains(err.Error(), "--agent-history requires --agent-sdk") {
		t.Fatalf("expected --agent-history validation error, got %v", err)
	}
}

func TestWriteCommandsExposeAgentHistoryFlag(t *testing.T) {
	commands := []struct {
		name   string
		lookup func(string) bool
		usage  func(string) string
	}{
		{name: "gen", lookup: func(flag string) bool { return writeGenCmd.Flags().Lookup(flag) != nil }, usage: func(flag string) string { return writeGenCmd.Flags().Lookup(flag).Usage }},
		{name: "improve", lookup: func(flag string) bool { return writeImproveCmd.Flags().Lookup(flag) != nil }, usage: func(flag string) string { return writeImproveCmd.Flags().Lookup(flag).Usage }},
		{name: "pipeline", lookup: func(flag string) bool { return writePipelineCmd.Flags().Lookup(flag) != nil }, usage: func(flag string) string { return writePipelineCmd.Flags().Lookup(flag).Usage }},
	}
	for _, command := range commands {
		if !command.lookup("agent-history") {
			t.Fatalf("write %s flag %q is not registered", command.name, "agent-history")
		}
		if !strings.Contains(command.usage("agent-history"), "logs") {
			t.Fatalf("write %s --agent-history usage should mention logs, got %q", command.name, command.usage("agent-history"))
		}
	}
}

func TestWriteImproveCommandExposesWordsFlagForRepairCommands(t *testing.T) {
	flag := writeImproveCmd.Flags().Lookup("words")
	if flag == nil {
		t.Fatalf("write improve flag %q is not registered", "words")
	}
	if !strings.Contains(flag.Usage, "Target word") {
		t.Fatalf("write improve --words usage should explain target words, got %q", flag.Usage)
	}
}

func TestFinalChapterContentChangedUsesSavedNormalization(t *testing.T) {
	chapter := &models.Chapter{ID: "P1-V1-C1", Title: "荒骨苏醒"}
	saved := "# 荒骨苏醒\r\n\r\n林野醒来。\r\n"

	if finalChapterContentChanged(chapter, saved, "林野醒来。") {
		t.Fatalf("content with equivalent normalized title/body should be treated as unchanged")
	}
	if !finalChapterContentChanged(chapter, saved, "林野醒来，听见通道深处传来虫鸣。") {
		t.Fatalf("changed body should be treated as changed")
	}
}

func TestWritePipelineCommandExposesAgentSDKFlags(t *testing.T) {
	for _, name := range []string{"agent-sdk", "agent-apply", "agent-history", "recap-agent-sdk"} {
		if writePipelineCmd.Flags().Lookup(name) == nil {
			t.Fatalf("write pipeline flag %q is not registered", name)
		}
	}
	agentSDKFlag := writePipelineCmd.Flags().Lookup("agent-sdk")
	if agentSDKFlag == nil || !strings.Contains(agentSDKFlag.Usage, "review") {
		t.Fatalf("write pipeline --agent-sdk usage should mention review, got %q", agentSDKFlag.Usage)
	}
}

func TestWriteReviewCommandExposesAgentSDKFlag(t *testing.T) {
	if writeReviewCmd.Flags().Lookup("agent-sdk") == nil {
		t.Fatalf("write review flag %q is not registered", "agent-sdk")
	}
}

func TestWriteReviewCommandExposesWordsFlag(t *testing.T) {
	flag := writeReviewCmd.Flags().Lookup("words")
	if flag == nil {
		t.Fatalf("write review flag %q is not registered", "words")
	}
	if !strings.Contains(flag.Usage, "Target word") {
		t.Fatalf("write review --words usage should explain target words, got %q", flag.Usage)
	}
}

func TestMergeVolumeReviewByChapterPreservesUnreviewedChapters(t *testing.T) {
	existing := &agents.VolumeReview{
		VolumeID:    "P1-V1",
		VolumeTitle: "旧卷名",
		Reviews: []agents.DraftReview{
			{ChapterID: "P1-V1-C1", ChapterTitle: "第一章", OverallScore: 88},
			{ChapterID: "P1-V1-C2", ChapterTitle: "第二章旧评", OverallScore: 70},
		},
	}
	incoming := &agents.VolumeReview{
		VolumeID:    "P1-V1",
		VolumeTitle: "新卷名",
		Reviews: []agents.DraftReview{
			{ChapterID: "P1-V1-C2", ChapterTitle: "第二章新评", OverallScore: 93},
		},
	}

	merged := mergeVolumeReviewByChapter(existing, incoming)

	if merged.VolumeTitle != "新卷名" {
		t.Fatalf("merged title = %q, want %q", merged.VolumeTitle, "新卷名")
	}
	if len(merged.Reviews) != 2 {
		t.Fatalf("merged reviews length = %d, want 2", len(merged.Reviews))
	}
	if merged.Reviews[0].ChapterID != "P1-V1-C1" || merged.Reviews[0].OverallScore != 88 {
		t.Fatalf("unreviewed chapter was not preserved: %#v", merged.Reviews[0])
	}
	if merged.Reviews[1].ChapterID != "P1-V1-C2" || merged.Reviews[1].OverallScore != 93 {
		t.Fatalf("reviewed chapter was not replaced: %#v", merged.Reviews[1])
	}
}
