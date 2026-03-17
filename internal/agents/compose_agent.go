package agents

import (
	"encoding/json"
	"fmt"
	"strings"

	"nolvegen/internal/llm"
	"nolvegen/internal/models"
)

// ComposeAgent handles AI generation for story outline
type ComposeAgent struct {
	client llm.Client
	model  string
}

// NewComposeAgent creates a new ComposeAgent
func NewComposeAgent(client llm.Client, model string) *ComposeAgent {
	return &ComposeAgent{
		client: client,
		model:  model,
	}
}

// GenerateOutline generates a story outline from a story setup
func (a *ComposeAgent) GenerateOutline(setup *models.StorySetup) (*models.Outline, error) {
	fmt.Println("🤖 Generating story outline with AI...")
	fmt.Println()

	// Build the system prompt
	systemPrompt := fmt.Sprintf(`You are a creative writing assistant specializing in novel outlining.
Your task is to generate a detailed story outline based on the story setup provided.

The outline must follow a strict 3-level structure: Parts → Volumes → Chapters.

Respond ONLY with a valid JSON object in the following format:
{
  "parts": [
    {
      "id": "part_1",
      "title": "Part Title",
      "summary": "Brief summary of this part",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Volume Title",
          "summary": "Brief summary of this volume",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "Chapter Title",
              "summary": "Brief summary of this chapter",
              "beats": ["Plot beat 1", "Plot beat 2", "Plot beat 3"],
              "conflict": "Main conflict in this chapter",
              "pacing": "slow|normal|fast"
            }
          ]
        }
      ]
    }
  ]
}

Guidelines:
- Create 2-3 parts for a complete story
- Each part should have 1-3 volumes
- Each volume should have 2-5 chapters
- Ensure the outline follows a coherent narrative arc
- Include specific plot beats for each chapter
- Vary the pacing (slow/normal/fast) based on the story needs
- Make conflicts clear and compelling

Story Setup:
- Project Name: %s
- Genres: %s
- Premise: %s
- Theme: %s
- Rules: %s
- Tone: %s
- Tense: %s
- POV: %s`,
		setup.ProjectName,
		strings.Join(setup.Genres, ", "),
		setup.Premise,
		setup.Theme,
		strings.Join(setup.Rules, "; "),
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
	)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: "Generate a complete story outline based on the story setup above.",
		},
	}

	options := &llm.ChatOptions{
		Temperature: 0.8,
		MaxTokens:   50000,
		Model:       a.model,
	}

	fmt.Println("Sending request to AI (this may take a while)...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Printf("Received response (%d tokens used)\n", resp.Usage.TotalTokens)
	fmt.Println()

	// Parse the JSON response
	var outline models.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &outline); err != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate the outline
	if len(outline.Parts) == 0 {
		return nil, fmt.Errorf("AI did not generate any parts")
	}

	fmt.Printf("✓ Generated outline with %d part(s)\n", len(outline.Parts))
	for _, part := range outline.Parts {
		chapterCount := 0
		for _, vol := range part.Volumes {
			chapterCount += len(vol.Chapters)
		}
		fmt.Printf("  - %s: %d volume(s), %d chapter(s)\n", part.Title, len(part.Volumes), chapterCount)
	}
	fmt.Println()

	return &outline, nil
}

// GenerateOutlineWithStructure generates a story outline with a predefined structure
func (a *ComposeAgent) GenerateOutlineWithStructure(setup *models.StorySetup, structure models.StoryStructure) (*models.Outline, error) {
	fmt.Println("🤖 Generating story outline with AI...")
	fmt.Println()

	totalChapters := structure.TotalChapters()

	// Build the system prompt with strict structure requirements
	systemPrompt := fmt.Sprintf(`You are a creative writing assistant specializing in novel outlining.
Your task is to generate a detailed story outline based on the story setup provided.

STRICT STRUCTURE REQUIREMENTS:
- You MUST generate exactly %d parts
- Each part MUST have exactly %d volumes  
- Each volume MUST have exactly %d chapters
- Total chapters: %d

The outline must follow a strict 3-level structure: Parts → Volumes → Chapters.

Respond ONLY with a valid JSON object in the following format:
{
  "parts": [
    {
      "id": "part_1",
      "title": "Part Title",
      "summary": "Brief summary of this part",
      "volumes": [
        {
          "id": "vol_1_1",
          "title": "Volume Title",
          "summary": "Brief summary of this volume",
          "chapters": [
            {
              "id": "chap_1_1_1",
              "title": "Chapter Title",
              "summary": "Brief summary of this chapter",
              "beats": ["Plot beat 1", "Plot beat 2", "Plot beat 3"],
              "conflict": "Main conflict in this chapter",
              "pacing": "slow|normal|fast"
            }
          ]
        }
      ]
    }
  ]
}

Guidelines:
- Follow the EXACT structure specified above
- Ensure the outline follows a coherent narrative arc across all parts
- Include specific plot beats for each chapter (3-5 beats per chapter)
- Vary the pacing (slow/normal/fast) based on the story needs
- Make conflicts clear and compelling
- Each part should have a clear narrative purpose
- Each volume should advance the story within its part
- Each chapter should have clear progression

Story Setup:
- Project Name: %s
- Genres: %s
- Premise: %s
- Theme: %s
- Rules: %s
- Tone: %s
- Tense: %s
- POV: %s

Structure: %d parts × %d volumes × %d chapters = %d total chapters`,
		structure.TargetParts,
		structure.TargetVolumes,
		structure.TargetChapters,
		totalChapters,
		setup.ProjectName,
		strings.Join(setup.Genres, ", "),
		setup.Premise,
		setup.Theme,
		strings.Join(setup.Rules, "; "),
		setup.Tone,
		setup.Tense,
		setup.POVStyle,
		structure.TargetParts,
		structure.TargetVolumes,
		structure.TargetChapters,
		totalChapters,
	)

	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Generate a complete story outline with exactly %d parts, %d volumes per part, and %d chapters per volume.",
				structure.TargetParts, structure.TargetVolumes, structure.TargetChapters),
		},
	}

	options := &llm.ChatOptions{
		Temperature: 0.8,
		MaxTokens:   50000,
		Model:       a.model,
	}

	fmt.Println("Sending request to AI (this may take a while)...")
	resp, err := a.client.ChatCompletion(messages, options)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Printf("Received response (%d tokens used)\n", resp.Usage.TotalTokens)
	fmt.Println()

	// Parse the JSON response
	var outline models.Outline
	if err := json.Unmarshal([]byte(resp.Content), &outline); err != nil {
		// Try to extract JSON from markdown code block if present
		content := extractJSONFromMarkdown(resp.Content)
		if err := json.Unmarshal([]byte(content), &outline); err != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON: %w\nResponse: %s", err, resp.Content)
		}
	}

	// Validate the outline structure
	if len(outline.Parts) != structure.TargetParts {
		return nil, fmt.Errorf("AI generated %d parts, but %d were requested", len(outline.Parts), structure.TargetParts)
	}

	for i, part := range outline.Parts {
		if len(part.Volumes) != structure.TargetVolumes {
			return nil, fmt.Errorf("part %d has %d volumes, but %d were requested", i+1, len(part.Volumes), structure.TargetVolumes)
		}
		for j, volume := range part.Volumes {
			if len(volume.Chapters) != structure.TargetChapters {
				return nil, fmt.Errorf("volume %d.%d has %d chapters, but %d were requested", i+1, j+1, len(volume.Chapters), structure.TargetChapters)
			}
		}
	}

	fmt.Printf("✓ Generated outline with %d part(s), %d volume(s) per part, %d chapter(s) per volume\n",
		len(outline.Parts), structure.TargetVolumes, structure.TargetChapters)
	fmt.Printf("  Total: %d chapters\n", totalChapters)
	fmt.Println()

	return &outline, nil
}
