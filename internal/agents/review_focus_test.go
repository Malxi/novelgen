package agents

import (
	"strings"
	"testing"
)

// TestReviewFocusesIntegrity 校验所有内置 focus 的基本完整性。
func TestReviewFocusesIntegrity(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range reviewFocuses {
		if f.Name == "" || f.DisplayName == "" || f.Prompt == "" {
			t.Errorf("focus 字段不可为空: name=%q display=%q", f.Name, f.DisplayName)
		}
		if seen[f.Name] {
			t.Errorf("focus 名称重复: %s", f.Name)
		}
		seen[f.Name] = true
	}
}

// TestExpertAlignedFocus 锁住"专家对齐"focus：
// 它承载反浮夸判据（LLM 评审天生偏爱表面光鲜的文本），是 write/compose review 的纠偏项。
func TestExpertAlignedFocus(t *testing.T) {
	f, ok := findReviewFocus("expert-aligned")
	if !ok {
		t.Fatal("缺少 expert-aligned focus")
	}
	// 关键判据必须留在 prompt 里，否则纠偏能力会静默失效
	for _, kw := range []string{"宛如", "状语", "省略", "白描", "词藻华丽"} {
		if !strings.Contains(f.Prompt, kw) {
			t.Errorf("expert-aligned prompt 缺少关键判据: %q", kw)
		}
	}

	if got := ResolveReviewFocusPrompt("expert-aligned"); !strings.Contains(got, "专家对齐") {
		t.Error("ResolveReviewFocusPrompt(\"expert-aligned\") 未返回对应内容")
	}
	if all := ResolveReviewFocusPrompt("all"); !strings.Contains(all, "专家对齐") {
		t.Error("--focus all 未包含 expert-aligned")
	}

	found := false
	for _, n := range ListReviewFocusNames() {
		if n == "expert-aligned" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListReviewFocusNames 未包含 expert-aligned")
	}
}
