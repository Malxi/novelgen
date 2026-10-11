package quality

import (
	"testing"

	"novelgen/internal/models"
)

func TestHasBlockingSuggestions(t *testing.T) {
	if HasBlockingSuggestions(nil) {
		t.Fatal("nil suggestions should not block")
	}
	if !HasBlockingSuggestions([]models.ReviewSuggestion{{Priority: models.PriorityHigh}}) {
		t.Fatal("high priority should block")
	}
	if HasBlockingSuggestions([]models.ReviewSuggestion{{Priority: models.PriorityLow}}) {
		t.Fatal("low priority should not block")
	}
}

func TestRunOutlineValidatorOnModelHandlesNil(t *testing.T) {
	if got := RunOutlineValidatorOnModel(nil); got != nil {
		t.Fatalf("nil outline should return nil, got %v", got)
	}
}
