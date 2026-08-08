package agents

import (
	"testing"

	"novelgen/internal/models"
)

func TestClassifyReviewThemes(t *testing.T) {
	cases := []struct {
		text   string
		expect string
	}{
		{"C2-C4连续三章为每日新方案-拆解-结局不变同构循环,节奏易疲劳", "中段循环/节拍器"},
		{"日志系统数据挖掘升级但无能力边界设定,读者无法回答为何不直接看行为日志", "推演/能力边界"},
		{"李侑×陆青禾感情线全卷只推进到试探层面,线感偏薄", "感情线"},
		{"内门末日预言在卷末首次出现,前序章节未见来源铺设", "伏笔/铺垫"},
		{"张三换废符作恶的私心没有任何铺垫,因果链断裂", "角色动机/弧光"},
		{"苔藓线索太完美,恰好证明有人清晨来过,AI对称思维", "逻辑/自洽"},
		{"C1承诺30天外门大比击败顾恒属P1级目标,与10章篇幅不匹配", "结构/商业"},
	}
	for _, c := range cases {
		got := ClassifyReviewThemes(c.text)
		if len(got) == 0 || got[0] != c.expect {
			t.Errorf("ClassifyReviewThemes(%q) = %v, want first %q", c.text, got, c.expect)
		}
	}
}

func TestStratifiedSampleCoversThemes(t *testing.T) {
	var sugs []ReviewMatrixSuggestion
	texts := []string{
		"C2-C4同构循环节奏易疲劳", "日志系统无能力边界", "感情线只有张力没有温度",
		"内门末日伏笔无铺垫", "张三动机缺失因果断裂", "苔藓证据时间矛盾",
		"每章金句排比像AI味", "卷结构高潮位置不对", "30天倒计时时序核对", "修为数字抽象无细节",
	}
	for i := 0; i < 20; i++ {
		sugs = append(sugs, ReviewMatrixSuggestion{
			ReviewSuggestion: models.ReviewSuggestion{Issue: texts[i%len(texts)]},
		})
	}
	AnnotateMatrixSuggestions(sugs)
	picked := StratifiedSample(sugs, 10, 42)
	if len(picked) != 10 {
		t.Fatalf("StratifiedSample len = %d, want 10", len(picked))
	}
	seen := map[string]bool{}
	for _, idx := range picked {
		seen[sugs[idx].Themes[0]] = true
	}
	if len(seen) < 3 {
		t.Errorf("sample covers only %d themes: %v", len(seen), seen)
	}
}

func TestStratifiedSampleDeterministic(t *testing.T) {
	var sugs []ReviewMatrixSuggestion
	for i := 0; i < 15; i++ {
		sugs = append(sugs, ReviewMatrixSuggestion{
			ReviewSuggestion: models.ReviewSuggestion{Issue: "测试问题" + string(rune('0'+i))},
		})
	}
	AnnotateMatrixSuggestions(sugs)
	a := StratifiedSample(sugs, 8, 7)
	b := StratifiedSample(sugs, 8, 7)
	if len(a) != len(b) {
		t.Fatalf("deterministic mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("seed deterministic broken at %d: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestBuildRationalizePrompt(t *testing.T) {
	items := []ReviewMatrixSuggestion{
		{ReviewSuggestion: models.ReviewSuggestion{
			TargetName: "第4章", Issue: "反杀太完美", Suggestion: "让主角踩错一次", Priority: "high",
		}},
	}
	p := BuildRationalizePrompt(items)
	if len(p) < 50 {
		t.Errorf("prompt too short: %d", len(p))
	}
}
