package memory

import (
	"context"
	"testing"
)

func TestNarrativeMemoryIntentEvaluation(t *testing.T) {
	root := memoryFixture(t)
	cases := []struct {
		name         string
		intent       string
		focus        string
		wantPromise  bool
		wantScenes   bool
		wantCiteHash bool
	}{
		{name: "chapter write", intent: "chapter-write", focus: "blue key", wantPromise: true, wantScenes: true, wantCiteHash: true},
		{name: "continuity", intent: "continuity", focus: "mine map", wantPromise: true, wantScenes: true, wantCiteHash: true},
		{name: "promise audit", intent: "promise-audit", wantPromise: true},
		{name: "voice echo", intent: "voice-echo", focus: "door voice", wantPromise: true, wantScenes: true, wantCiteHash: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Query(context.Background(), root, QueryOptions{
				ChapterID: "P1-V1-C3", Intent: tc.intent, Focus: tc.focus, MaxChars: 1800,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantPromise && len(result.Promises) == 0 {
				t.Fatalf("expected promise context: %#v", result)
			}
			if tc.wantScenes && len(result.Retrieved) == 0 {
				t.Fatalf("expected retrieved scenes: %#v", result)
			}
			if tc.wantCiteHash {
				if len(result.Citations) != len(result.Retrieved) {
					t.Fatalf("citations/scenes = %d/%d", len(result.Citations), len(result.Retrieved))
				}
				for _, citation := range result.Citations {
					if citation.SHA256 == "" || citation.ParagraphStart < 1 || citation.ParagraphEnd < citation.ParagraphStart {
						t.Fatalf("invalid citation: %#v", citation)
					}
				}
			}
			for _, scene := range result.Retrieved {
				if scene.ChapterOrder > result.ChapterOrder {
					t.Fatalf("future scene leaked: %#v", scene)
				}
			}
		})
	}
	budgeted, err := Query(context.Background(), root, QueryOptions{
		ChapterID: "P1-V1-C3", Intent: "continuity", MaxChars: 400, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if budgeted.Meta.ReturnedChars > 400 {
		t.Fatalf("narrative memory exceeded budget: %#v", budgeted.Meta)
	}
}
