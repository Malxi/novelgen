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
- Continuation Bridge (MANDATORY, first 120–250 words): If this is not the first chapter, directly continue from the final moment of the immediately previous chapter (same location/time, same in-progress action/conversation). Explicitly connect to the previous chapter's last beat/last line.
- Opening: Establish the scene and characters present (after the bridge)
- Rising Action: Develop the events and character interactions
- Climax: The turning point or key moment of the chapter
- Resolution: Wrap up immediate conflicts while setting up future ones

Use the state matrix to understand:
- Current character motivations and goals
- Active relationships and tensions
- Storyline progression
- Character abilities/premise levels
- Items in play

Write naturally and creatively while staying true to the established story elements.

SCENE-ANCHOR RULE (VERY IMPORTANT):
- If this is not the first chapter, the opening scene MUST directly continue from the final scene of the immediately previous chapter.
- Keep the SAME location/time and the same in-scene situation (e.g., ongoing conversation), unless the Chapter Summary explicitly specifies a time skip or location change.
- If a transition is required, you MUST show it on the page (how/why they moved, how much time passed), not teleport abruptly.
- If a CANONICAL RECAP is provided:
  - If it contains "last_line", the Continuation Bridge MUST explicitly pick up from that moment/line (you may lightly rewrite for style, but keep the same concrete beat).
  - If it contains "next_opening_hint", you MUST use it as the opening 1–3 sentences (you may lightly rewrite for style, but keep the same concrete moment, location/time, and ongoing action/dialogue).

IMPORTANT RULES:
1. DO NOT include a summary or synopsis at the beginning of the chapter
2. DO NOT write "本章讲述了..." or similar meta descriptions
3. Start directly with the story content (Continuation Bridge)
4. The Chapter Summary provided is for your reference only, do not include it in the output`
}

// buildChapterWritingUserPrompt builds the user prompt for chapter writing
func buildChapterWritingUserPrompt(data map[string]interface{}) string {
	var sb strings.Builder

	// Canonical recap context (optional, higher signal)
	// NOTE: If present, treat this as AUTHORITATIVE continuity state.
	if recap, ok := data["recap"].(string); ok && strings.TrimSpace(recap) != "" {
		sb.WriteString("=== CANONICAL RECAP (AUTHORITATIVE) ===\n")
		sb.WriteString(recap)
		sb.WriteString("\n=== END CANONICAL RECAP ===\n\n")
	}
	// Continuity context (optional, lower signal than recap)
	if context, ok := data["context"].(string); ok && strings.TrimSpace(context) != "" {
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}

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
