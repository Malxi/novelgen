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
