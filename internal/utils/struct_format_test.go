package utils

import (
	"strings"
	"testing"
)

func TestStructToMarkdownExpandsPointerAndMapFields(t *testing.T) {
	type navigation struct {
		TargetKind string                 `json:"target_kind"`
		PatchShape map[string]interface{} `json:"patch_shape"`
	}
	type suggestion struct {
		Issue      string      `json:"issue"`
		Navigation *navigation `json:"navigation"`
	}

	got := StructToMarkdown(suggestion{
		Issue: "repair opening beat",
		Navigation: &navigation{
			TargetKind: "chapter",
			PatchShape: map[string]interface{}{
				"changed_chapters": []map[string]string{{
					"id":           "P1-V1-C2",
					"opening_beat": "紧接着上一章结尾，主角前往执法堂。",
				}},
			},
		},
	}, 0)

	for _, want := range []string{"**Navigation:**", "**Target Kind:** chapter", "changed_chapters", "opening_beat"} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "&{") {
		t.Fatalf("markdown should not expose Go pointer formatting:\n%s", got)
	}
}
