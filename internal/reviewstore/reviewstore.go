// Package reviewstore reads and writes per-volume review JSON and formats a
// DraftReview into the improvement-suggestion block used by the write/draft
// prompts.
package reviewstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/agents"
	"novelgen/internal/project"
)

// SaveVolumeReview writes story/reviews/<volumeID>_review.json.
func SaveVolumeReview(review *agents.VolumeReview) error {
	root, err := project.FindRoot()
	if err != nil {
		return err
	}
	reviewDir := filepath.Join(root, "story", "reviews")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		return fmt.Errorf("failed to create reviews directory: %w", err)
	}
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}
	return os.WriteFile(filepath.Join(reviewDir, review.VolumeID+"_review.json"), data, 0644)
}

// LoadVolumeReview reads story/reviews/<volumeID>_review.json.
func LoadVolumeReview(volumeID string) (*agents.VolumeReview, error) {
	root, err := project.FindRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "story", "reviews", volumeID+"_review.json"))
	if err != nil {
		return nil, err
	}
	var review agents.VolumeReview
	if err := json.Unmarshal(data, &review); err != nil {
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
