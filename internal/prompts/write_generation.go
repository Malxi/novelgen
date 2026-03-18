package prompts

import (
	"fmt"
	"strings"
)

// registerWritePrompts registers all write-related prompts
func registerWritePrompts(pm *PromptManager) {
	// Final chapter writing prompt with continuity
	pm.Register(&PromptTemplate{
		Skill:        SkillChapterWriting,
		Name:         "final",
		Description:  "Generate final chapter content with continuity",
		SystemPrompt: buildFinalChapterSystemPrompt(),
		OutputFormat: FormatText,
	})
}

func buildFinalChapterSystemPrompt() string {
	return `You are a professional novelist and creative writer specializing in long-form fiction.
Your task is to write polished, publication-ready chapter content.

WRITING GUIDELINES:
1. Write immersive, engaging prose with strong narrative voice
2. Show, don't tell - use sensory details, actions, and dialogue
3. Maintain consistent character voices and personalities throughout
4. Include natural, purposeful dialogue that reveals character and advances plot
5. Balance description, action, dialogue, and internal monologue
6. Create smooth transitions and pacing appropriate to the scene
7. End with a compelling hook that makes readers want to continue

CONTINUITY REQUIREMENTS:
1. Review the previous chapters' content carefully
2. Maintain consistency with established character voices and behaviors
3. Reference previous events naturally when appropriate
4. Ensure plot progression flows logically from what came before
5. Plant seeds for future developments based on upcoming chapter summaries
6. Avoid contradictions with established facts or character traits

CHAPTER STRUCTURE:
- Opening: Establish the scene with sensory details, set the tone
- Rising Action: Develop tension through character interactions and events
- Climax: The key moment or turning point of the chapter
- Resolution: Resolve immediate conflicts while setting up future ones
- Transition: Smoothly lead into the next chapter's events

Write in a professional, polished style suitable for publication.`
}

// buildFinalChapterUserPrompt builds the user prompt for final chapter generation
func buildFinalChapterUserPrompt(data map[string]interface{}) string {
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

	// Context from surrounding chapters
	if context, ok := data["context"].(string); ok && context != "" {
		sb.WriteString(context)
		sb.WriteString("\n")
	}

	// Current chapter info
	if chapterID, ok := data["chapter_id"].(string); ok {
		sb.WriteString(fmt.Sprintf("CURRENT CHAPTER: %s\n", chapterID))
	}
	if chapterTitle, ok := data["chapter_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("Chapter Title: %s\n", chapterTitle))
	}
	if summary, ok := data["chapter_summary"].(string); ok && summary != "" {
		sb.WriteString(fmt.Sprintf("Chapter Summary: %s\n", summary))
	}

	sb.WriteString("\n")

	// Characters in this chapter
	if characters, ok := data["characters"].(string); ok && characters != "" {
		sb.WriteString(fmt.Sprintf("Characters Present: %s\n", characters))
	}

	// Location
	if location, ok := data["location"].(string); ok && location != "" {
		sb.WriteString(fmt.Sprintf("Primary Location: %s\n", location))
	}

	sb.WriteString("\n")

	// Target word count
	if words, ok := data["target_words"].(int); ok && words > 0 {
		sb.WriteString(fmt.Sprintf("TARGET LENGTH: Approximately %d words\n\n", words))
	}

	sb.WriteString("Write the final chapter content now. Focus on:")
	sb.WriteString("\n- Seamless continuity with previous chapters")
	sb.WriteString("\n- Deepening character development through action and dialogue")
	sb.WriteString("\n- Advancing the plot while maintaining tension")
	sb.WriteString("\n- Rich, immersive descriptions that engage the senses")
	sb.WriteString("\n- Natural dialogue that reveals character and moves the story forward")
	sb.WriteString("\n- Subtle foreshadowing of future events based on upcoming chapter summaries")
	sb.WriteString("\n- A satisfying chapter arc that leaves readers wanting more")
	sb.WriteString("\n\nWrite only the chapter content, no meta-commentary or chapter headers.")

	return sb.String()
}
