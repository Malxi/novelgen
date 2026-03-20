package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/logger"
	"nolvegen/internal/logic/continuity/recap"
	"nolvegen/internal/models"
	"nolvegen/internal/prompts"
)

// RecapAgent extracts a canonical recap JSON from chapter text
type RecapAgent struct {
	client     llm.Client
	config     *llm.Config
	projectLLM *models.ProjectLLM
	language   string
	log        logger.LoggerInterface
	pm         *prompts.PromptManager
}

func NewRecapAgent(client llm.Client, config *llm.Config, projectLLM *models.ProjectLLM, language string) *RecapAgent {
	return &RecapAgent{
		client:     client,
		config:     config,
		projectLLM: projectLLM,
		language:   language,
		log:        logger.GetLogger(),
		pm:         prompts.NewPromptManager(),
	}
}

func (a *RecapAgent) Extract(chapterID, title string, chapterText string) (*models.ChapterRecap, error) {
	return a.ExtractWithFeedback(chapterID, title, chapterText, "")
}

// ExtractWithFeedback extracts a recap and optionally provides structured feedback
// to force the model to fill missing fields (minimal gate).
func (a *RecapAgent) ExtractWithFeedback(chapterID, title string, chapterText string, feedback string) (*models.ChapterRecap, error) {
	data := map[string]interface{}{
		"chapter_id": chapterID,
		"title":      title,
		"text":       chapterText,
		"language":   a.language,
	}

	systemPrompt, userPrompt, err := a.pm.Build(prompts.SkillChapterRecap, "extract", data)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Up to two passes: first extraction, then an auto-repair pass if the recap
	// fails the minimal continuity gate.
	attempts := 1
	if strings.TrimSpace(feedback) == "" {
		attempts = 2
	}

	var lastRaw string
	var out models.ChapterRecap
	for i := 0; i < attempts; i++ {
		curFeedback := strings.TrimSpace(feedback)
		if i == 1 {
			// Second pass: inject deterministic reasons as "must address" feedback.
			if ok, reasons := recap.ValidateMinimal(&out); !ok {
				curFeedback = strings.Join(reasons, "; ")
			} else if ok, reasons := recap.ValidateConsistency(&out); !ok {
				curFeedback = strings.Join(reasons, "; ")
			}
		}

		userPrompt = "CHAPTER METADATA:\n" + fmt.Sprintf("ChapterID: %s\nTitle: %s\n\n", chapterID, title)
		if curFeedback != "" {
			userPrompt += "RECAP FIX FEEDBACK (MUST ADDRESS):\n" + curFeedback + "\n\n"
		}
		userPrompt += "CHAPTER TEXT:\n" + chapterText

		messages := []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		opts := a.config.GetChatOptions(a.projectLLM)
		if opts.MaxTokens < 2000 {
			opts.MaxTokens = 2000
		}

		resp, err := a.client.ChatCompletion(messages, opts)
		if err != nil {
			return nil, err
		}

		// Try parse JSON
		content := strings.TrimSpace(resp.Content)
		// Strip possible fenced code blocks
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)

		lastRaw = resp.Content
		out = models.ChapterRecap{}
		if err := json.Unmarshal([]byte(content), &out); err != nil {
			return nil, fmt.Errorf("failed to parse recap json: %w; raw=%s", err, lastRaw)
		}

		// If it passes minimal continuity gate (and consistency), we're done.
		if ok, _ := recap.ValidateMinimal(&out); ok {
			if ok2, _ := recap.ValidateConsistency(&out); ok2 {
				return &out, nil
			}
		}
	}

	// Return whatever we got from the last pass.
	return &out, nil
}
