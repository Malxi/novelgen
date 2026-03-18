package prompts

import (
	"fmt"
	"strings"
)

// registerDraftPrompts registers all draft-related prompts
func registerDraftPrompts(pm *PromptManager) {
	// Chapter writing prompt
	pm.Register(&PromptTemplate{
		Skill:        SkillChapterWriting,
		Name:         "default",
		Description:  "Generate a draft chapter based on story state",
		SystemPrompt: buildChapterWritingSystemPrompt(),
		OutputFormat: FormatText,
	})
}

func buildChapterWritingSystemPrompt() string {
	return `You are a professional novelist and creative writer.
Your task is to write a compelling draft chapter based on the provided story state and chapter outline.

WRITING GUIDELINES:
1. Write in an engaging, immersive style appropriate for the genre
2. Show, don't tell - use sensory details and character actions
3. Maintain consistent character voices and personalities
4. Include dialogue that reveals character and advances plot
5. Balance description, action, and dialogue
6. Follow the chapter summary and cover all specified events
7. End with a hook that makes readers want to continue

CHAPTER STRUCTURE:
- Opening: Establish the scene and characters present
- Rising Action: Develop the events and character interactions
- Climax: The turning point or key moment of the chapter
- Resolution: Wrap up immediate conflicts while setting up future ones

Use the state matrix to understand:
- Current character motivations and goals
- Active relationships and tensions
- Storyline progression
- Character abilities/premise levels
- Items in play

Write naturally and creatively while staying true to the established story elements.`
}

// buildChapterWritingUserPrompt builds the user prompt for chapter writing
func buildChapterWritingUserPrompt(data map[string]interface{}) string {
	var sb strings.Builder

	// Story context
	if title, ok := data["story_title"].(string); ok && title != "" {
		sb.WriteString(fmt.Sprintf("Story Title: %s\n", title))
	}
	if genre, ok := data["story_genre"].(string); ok && genre != "" {
		sb.WriteString(fmt.Sprintf("Genre: %s\n", genre))
	}
	if style, ok := data["story_style"].(string); ok && style != "" {
		sb.WriteString(fmt.Sprintf("Style: %s\n", style))
	}

	sb.WriteString("\n")

	// Chapter info
	if chapterID, ok := data["chapter_id"].(string); ok {
		sb.WriteString(fmt.Sprintf("Chapter ID: %s\n", chapterID))
	}
	if chapterTitle, ok := data["chapter_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("Chapter Title: %s\n", chapterTitle))
	}
	if summary, ok := data["chapter_summary"].(string); ok && summary != "" {
		sb.WriteString(fmt.Sprintf("Chapter Summary: %s\n", summary))
	}

	sb.WriteString("\n")

	// State matrix
	if stateMatrix, ok := data["state_matrix"].(string); ok && stateMatrix != "" {
		sb.WriteString(stateMatrix)
		sb.WriteString("\n\n")
	}

	// Target word count
	if words, ok := data["target_words"].(int); ok && words > 0 {
		sb.WriteString(fmt.Sprintf("TARGET LENGTH: Approximately %d words\n\n", words))
	}

	sb.WriteString("Write the draft chapter now. Focus on:")
	sb.WriteString("\n- Bringing the characters to life through action and dialogue")
	sb.WriteString("\n- Covering all events mentioned in the chapter summary")
	sb.WriteString("\n- Maintaining the established tone and style")
	sb.WriteString("\n- Creating an engaging narrative that flows naturally")
	sb.WriteString("\n\nWrite only the chapter content, no meta-commentary.")

	return sb.String()
}
