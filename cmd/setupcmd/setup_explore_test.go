package setupcmd

import (
	"encoding/json"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/creativity"
	"novelgen/internal/platform/store"
)

func TestGenericSetupTropeHits(t *testing.T) {
	setup := models.StorySetup{
		Premise: "一个废柴弟子靠老爷爷戒指逆袭。",
		Storylines: []models.Storyline{{
			Name:        "宗门大比",
			Description: "主角在秘境夺宝后参加宗门大比。",
		}},
	}
	hits := genericSetupTropeHits(&setup, "一个普通的修仙故事")
	for _, want := range []string{"废柴", "逆袭", "老爷爷", "宗门大比", "秘境夺宝"} {
		if !containsString(hits, want) {
			t.Errorf("missing trope %q in %#v", want, hits)
		}
	}
}

func TestGenericSetupTropeHitsRespectsExplicitIdea(t *testing.T) {
	setup := models.StorySetup{
		Premise: "一个废柴弟子靠老爷爷戒指逆袭。",
	}
	hits := genericSetupTropeHits(&setup, "我要写一个废柴逆袭的故事")
	if containsString(hits, "废柴") || containsString(hits, "逆袭") {
		t.Fatalf("explicit idea should suppress matching trope penalties: %#v", hits)
	}
	if !containsString(hits, "老爷爷") {
		t.Fatalf("unrequested trope should still be penalized: %#v", hits)
	}
}

func TestWriteSetupExploreCandidate(t *testing.T) {
	root := store.MemRoot()
	candidate := setupExploreCandidate{
		Candidate:   2,
		Provocation: &creativity.Provocation{ID: "axiom-42-01", Sentence: "测试句子"},
		Setup:       models.StorySetup{ProjectName: "测试", Premise: "一个测试设定"},
		ReviewScore: 88,
	}
	path, err := writeSetupExploreCandidate(root, 42, candidate)
	if err != nil {
		t.Fatalf("writeSetupExploreCandidate: %v", err)
	}
	if path != root.SetupExploreFile(42, "candidate-02.json") {
		t.Fatalf("path = %q", path)
	}
	data, err := root.Read(path)
	if err != nil {
		t.Fatalf("read candidate: %v", err)
	}
	var loaded setupExploreCandidate
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal candidate: %v", err)
	}
	if loaded.Candidate != 2 || loaded.Setup.ProjectName != "测试" || loaded.ReviewScore != 88 {
		t.Fatalf("loaded candidate = %#v", loaded)
	}
}

func TestSortSetupExploreCandidatesPutsRejectedLast(t *testing.T) {
	candidates := []setupExploreCandidate{
		{Candidate: 1, RankingScore: 95, CausalRejected: true},
		{Candidate: 2, RankingScore: 80, CausalRejected: false},
		{Candidate: 3, RankingScore: 90, CausalRejected: false},
	}
	sortSetupExploreCandidates(candidates)
	if candidates[0].Candidate != 3 || candidates[1].Candidate != 2 || candidates[2].Candidate != 1 {
		t.Fatalf("unexpected order: %#v", candidates)
	}
}

func TestSummarizeCausalIssuesCapsOutput(t *testing.T) {
	got := summarizeCausalIssues([]string{"a", "b", "c", "d"}, 2)
	if got != "a; b; ... 2 more" {
		t.Fatalf("got %q", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
