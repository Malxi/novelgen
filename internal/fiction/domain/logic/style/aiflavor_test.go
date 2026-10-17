package style

import "testing"

func TestCheckAIFlavorFlagsFormulaicProse(t *testing.T) {
	text := "这不仅是一次普通的胜利，更是命运的齿轮开始转动的重要标志。与此同时，他内心深处涌起复杂的情绪，空气仿佛凝固。"
	result := CheckAIFlavor(text, 80)
	if !result.HasIssue {
		t.Fatalf("expected AI flavor issue, got score %d", result.Score)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestCheckAIFlavorAllowsConcreteProse(t *testing.T) {
	text := "林砚把湿透的袖口拧了一下，水滴落在石阶上。他没看追兵，只把刀柄往掌心压紧，低声说：走左边。"
	result := CheckAIFlavor(text, 80)
	if result.HasIssue {
		t.Fatalf("expected concrete prose to pass, got score %d issues=%v", result.Score, result.Issues)
	}
}
