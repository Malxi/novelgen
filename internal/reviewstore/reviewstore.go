// Package reviewstore reads and writes per-volume review JSON and formats a
// DraftReview into the improvement-suggestion block used by the write/draft
// prompts.
package reviewstore

import (
	"fmt"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/project"
)

// SaveVolumeReviewAt writes a volume review under an explicit project root.
// There is deliberately no ambient variant: the project a review belongs to must
// come from the caller, not from the process working directory.
func SaveVolumeReviewAt(book *project.Book, review *agents.VolumeReview) error {
	if review == nil {
		return fmt.Errorf("cannot save nil volume review")
	}
	return book.WriteJSON(book.VolumeReviewFile(review.VolumeID), review)
}

// LoadVolumeReviewAt reads a volume review under an explicit project root.
func LoadVolumeReviewAt(book *project.Book, volumeID string) (*agents.VolumeReview, error) {
	var review agents.VolumeReview
	if err := book.ReadJSON(book.VolumeReviewFile(volumeID), &review); err != nil {
		return nil, err
	}
	return &review, nil
}

// BuildImprovementSuggestions formats a chapter review into the
// "## 改进建议" markdown block appended to improve prompts.
func BuildImprovementSuggestions(review *agents.DraftReview) string {
	var sb strings.Builder
	sb.WriteString("## 改进建议\n\n")

	writeSection := func(title string, suggestions []string) {
		if len(suggestions) == 0 {
			return
		}
		sb.WriteString("### " + title + "\n")
		for _, s := range suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	writeSection("场景/转场连续性", review.SceneContinuity.Suggestions)
	writeSection("角色出场一致性", review.CharacterPresence.Suggestions)
	writeSection("剧情连贯性", review.PlotCoherence.Suggestions)
	writeSection("情节合理性", review.PlotRationality.Suggestions)
	writeSection("角色一致性", review.CharacterConsistency.Suggestions)
	writeSection("节奏把控", review.PacingReview.Suggestions)
	// The final section intentionally has no trailing blank line, matching the
	// historical output byte for byte.
	if len(review.Suggestions) > 0 {
		sb.WriteString("### 总体建议\n")
		for _, s := range review.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
	}

	return sb.String()
}
