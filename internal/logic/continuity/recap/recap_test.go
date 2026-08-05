package recap

import (
	"testing"

	"novelgen/internal/models"
)

func TestValidateConsistency_LongNextOpeningHint(t *testing.T) {
	r := &models.ChapterRecap{
		ChapterID:       "C1",
		Title:           "T",
		Location:        "L",
		Present:         []string{"A"},
		LastLine:        "他推开门。",
		NextOpeningHint: "他推开门。" + repeat("很长的提示", 60),
	}
	ok, reasons := ValidateConsistency(r)
	if ok {
		t.Fatalf("expected ok=false for long hint")
	}
	found := false
	for _, s := range reasons {
		if s == "next_opening_hint 过长（建议 1–3 句，避免跑题）" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected long hint reason, got: %#v", reasons)
	}
}

func TestValidateConsistency_ShortHint_OK(t *testing.T) {
	r := &models.ChapterRecap{
		ChapterID:       "C1",
		Title:           "T",
		Location:        "L",
		Present:         []string{"A"},
		LastLine:        "他推开门。",
		NextOpeningHint: "他推开门，寒气扑面而来。",
	}
	ok, reasons := ValidateConsistency(r)
	if !ok {
		t.Fatalf("expected ok=true, got reasons: %#v", reasons)
	}
}

func TestValidateConsistency_AllowsSharedLastSceneKeyword(t *testing.T) {
	r := &models.ChapterRecap{
		ChapterID:       "C1",
		Title:           "T",
		Location:        "L",
		Present:         []string{"A"},
		LastLine:        "但在地下三层的那个角落里，银色的菱形金属舱体始终没有停止运转——火种核心还在呼吸，等待它的主人回来。",
		NextOpeningHint: "火种核心还在呼吸，还在等待——但它的主人已被勘探队抬上担架，在废土的夜色中远去。",
	}
	ok, reasons := ValidateConsistency(r)
	if !ok {
		t.Fatalf("expected ok=true for shared last-scene keyword, got reasons: %#v", reasons)
	}
}

func TestValidateConsistency_RejectsUnrelatedShortHint(t *testing.T) {
	r := &models.ChapterRecap{
		ChapterID:       "C1",
		Title:           "T",
		Location:        "L",
		Present:         []string{"A"},
		LastLine:        "他推开门。",
		NextOpeningHint: "远处传来新的警报声。",
	}
	ok, reasons := ValidateConsistency(r)
	if ok {
		t.Fatalf("expected ok=false for unrelated hint")
	}
	found := false
	for _, s := range reasons {
		if s == "next_opening_hint 与 last_line 缺少明显承接词（可能跑题）" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected continuation reason, got: %#v", reasons)
	}
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
