package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"novelgen/internal/llm"
	"novelgen/internal/logger"
	"novelgen/internal/logic/continuity/recap"
	"novelgen/internal/models"
)

// RecapExtractInput is the input for recap extraction
type RecapExtractInput struct {
	ChapterID   string `json:"chapter_id" md:"chapter_id" desc:"Chapter ID"`
	Title       string `json:"title" md:"title" desc:"Chapter title"`
	ChapterText string `json:"chapter_text" md:"chapter_text" desc:"Full chapter text to extract recap from"`
	Feedback    string `json:"feedback,omitempty" md:"feedback,omitempty" desc:"Optional feedback for recap extraction"`
}

// RecapExtractOutput is the output for recap extraction
type RecapExtractOutput struct {
	Recap models.ChapterRecap `json:"recap" md:"recap" desc:"Extracted chapter recap with character states and events"`
}

// RecapAgent extracts a canonical recap JSON from chapter text
// It wraps BaseAgent to provide type-safe methods
type RecapAgent struct {
	base *BaseAgent
}

// NewRecapAgent creates a new RecapAgent
func NewRecapAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM) *RecapAgent {
	base := NewBaseAgent(BaseAgentConfig{
		Name:       "RecapAgent",
		Client:     client,
		Config:     config,
		ProjectLLM: projectLLM,
		Language:   "zh",
	})

	return &RecapAgent{
		base: base,
	}
}

// SetLanguage sets the output language
func (a *RecapAgent) SetLanguage(language string) {
	a.base.SetLanguage(language)
}

// Extract extracts a recap from chapter text
func (a *RecapAgent) Extract(ctx context.Context, chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	return a.ExtractWithFeedback(ctx, chapterID, title, chapterText, "")
}

// ExtractWithFeedback extracts a recap and optionally provides structured feedback
// to force the model to fill missing fields (minimal gate).
func (a *RecapAgent) ExtractWithFeedback(ctx context.Context, chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	logger.Section("RECAP AGENT - Extract Recap")
	logger.Info("Chapter: %s - %s", chapterID, title)
	logger.Info("Language: %s", a.base.language)

	input := RecapExtractInput{
		ChapterID:   chapterID,
		Title:       title,
		ChapterText: chapterText,
		Feedback:    feedback,
	}

	var output RecapExtractOutput
	params := InvokeParams{
		Skills:  []string{"recap-extract"},
		Command: "extract chapter recap",
	}

	// Up to two passes: first extraction, then an auto-repair pass if the recap
	// fails the minimal continuity gate.
	attempts := 1
	if strings.TrimSpace(feedback) == "" {
		attempts = 2
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		curFeedback := strings.TrimSpace(feedback)
		if i == 1 {
			// Second pass: inject deterministic reasons as "must address" feedback.
			if ok, reasons := recap.ValidateMinimal(&output.Recap); !ok {
				curFeedback = strings.Join(reasons, "; ")
				input.Feedback = curFeedback
			} else if ok, reasons := recap.ValidateConsistency(&output.Recap); !ok {
				curFeedback = strings.Join(reasons, "; ")
				input.Feedback = curFeedback
			}
		}

		if err := a.base.Execute(ctx, params, input, &output); err != nil {
			lastErr = err
			continue
		}

		// If it passes minimal continuity gate (and consistency), we're done.
		if ok, _ := recap.ValidateMinimal(&output.Recap); ok {
			if ok2, _ := recap.ValidateConsistency(&output.Recap); ok2 {
				logger.Info("✓ Recap extracted successfully")
				return &output.Recap, nil
			}
		}
	}

	// Return whatever we got from the last pass, or the last error
	if lastErr != nil {
		return nil, lastErr
	}

	logger.Info("✓ Recap extracted (with warnings)")
	return &output.Recap, nil
}

// ExtractFromJSON extracts a recap from a JSON string (for manual editing)
func (a *RecapAgent) ExtractFromJSON(jsonStr string) (*models.ChapterRecap, error) {
	var recap models.ChapterRecap
	if err := json.Unmarshal([]byte(jsonStr), &recap); err != nil {
		return nil, fmt.Errorf("failed to parse recap JSON: %w", err)
	}
	return &recap, nil
}
