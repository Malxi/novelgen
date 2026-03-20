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

	// Improved chapter writing prompt with review suggestions
	pm.Register(&PromptTemplate{
		Skill:        SkillChapterWriting,
		Name:         "improve",
		Description:  "Generate improved chapter content based on review feedback",
		SystemPrompt: buildImproveChapterSystemPrompt(),
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

SCENE-ANCHOR RULE (VERY IMPORTANT):
- The opening scene of this chapter MUST directly continue from the final scene of the immediately previous chapter.
- Keep the SAME location/time and the same in-scene situation (e.g., ongoing conversation), unless the Chapter Summary EXPLICITLY specifies a time skip or location change.
- If a transition is required, you MUST show it on the page (how/why they moved, how much time passed), not teleport abruptly.
- If a CANONICAL RECAP is provided and it contains the field "next_opening_hint", you MUST use it as the opening 1–3 sentences (you may lightly rewrite for style, but keep the same concrete moment, location/time, and ongoing action/dialogue).
- If the review feedback contains a block titled "【硬性修复指令：补转场桥段】", you MUST treat it as a hard requirement and follow it exactly.

CHAPTER STRUCTURE:
- Continuation Bridge (MANDATORY, first 200–400 words): Directly continue from the final moment of the immediately previous chapter (same location/time, same in-progress action/conversation). This bridge must explicitly connect to the previous chapter's last beat/last line.
- Opening: Establish the scene with sensory details, set the tone (after the bridge)
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

	// Canonical recap context (optional, higher signal)
	// If present, treat as AUTHORITATIVE continuity state for scene-anchor.
	if recap, ok := data["recap"].(string); ok && recap != "" {
		sb.WriteString("=== CANONICAL RECAP (AUTHORITATIVE) ===\n")
		sb.WriteString(recap)
		sb.WriteString("\n=== END CANONICAL RECAP ===\n\n")
	}
	// Context from surrounding chapters (lower signal than recap)
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

	// State matrix - story state at this point
	if stateMatrix, ok := data["state_matrix"].(string); ok && stateMatrix != "" {
		sb.WriteString(stateMatrix)
		sb.WriteString("\n")
	}

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

func buildImproveChapterSystemPrompt() string {
	return `You are a professional novelist and creative writer specializing in long-form fiction.
Your task is to rewrite and improve a chapter based on specific review feedback.

IMPROVEMENT GUIDELINES:
1. Carefully analyze the review suggestions and address each point
2. Maintain the original chapter's core events and plot progression
3. Fix identified issues: plot holes, character inconsistencies, pacing problems, etc.
4. Enhance strengths mentioned in the review
5. Preserve continuity with surrounding chapters
6. Keep the same target word count and chapter structure
7. Write in a professional, polished style suitable for publication

REVIEW FEEDBACK PRIORITY:
- Address critical issues first (plot holes, major inconsistencies)
- Fix character voice and behavior inconsistencies
- Improve pacing where flagged
- Enhance weak descriptions or dialogue
- Strengthen emotional impact where noted
- Maintain all positive aspects that were praised

CONTINUITY REQUIREMENTS:
- Review the previous chapters' content carefully
- Maintain consistency with established character voices and behaviors
- Ensure plot progression flows logically from what came before
- Plant seeds for future developments based on upcoming chapter summaries
- Avoid contradictions with established facts or character traits

SCENE-ANCHOR RULE (VERY IMPORTANT):
- The opening scene of this chapter MUST directly continue from the final scene of the immediately previous chapter.
- Keep the SAME location/time and the same in-scene situation (e.g., ongoing conversation), unless the Chapter Summary EXPLICITLY specifies a time skip or location change.
- If a transition is required, you MUST show it on the page (how/why they moved, how much time passed), not teleport abruptly.
- If a CANONICAL RECAP is provided and it contains the field "next_opening_hint", you MUST use it as the opening 1–3 sentences (you may lightly rewrite for style, but keep the same concrete moment, location/time, and ongoing action/dialogue).
- If the review feedback contains a block titled "【硬性修复指令：补转场桥段】", you MUST treat it as a hard requirement and follow it exactly.

CHAPTER STRUCTURE:
- Continuation Bridge (MANDATORY, first 200–400 words): Directly continue from the final moment of the immediately previous chapter (same location/time, same in-progress action/conversation). This bridge must explicitly connect to the previous chapter's last beat/last line.

Write in a professional, polished style suitable for publication.`
}

// buildImproveChapterUserPrompt builds the user prompt for improved chapter generation
func buildImproveChapterUserPrompt(data map[string]interface{}) string {
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

	// Canonical recap context (optional, higher signal)
	// If present, treat as AUTHORITATIVE continuity state for scene-anchor.
	if recap, ok := data["recap"].(string); ok && recap != "" {
		sb.WriteString("=== CANONICAL RECAP (AUTHORITATIVE) ===\n")
		sb.WriteString(recap)
		sb.WriteString("\n=== END CANONICAL RECAP ===\n\n")
	}
	// Context from surrounding chapters (lower signal than recap)
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

	// State matrix - story state at this point
	if stateMatrix, ok := data["state_matrix"].(string); ok && stateMatrix != "" {
		sb.WriteString(stateMatrix)
		sb.WriteString("\n")
	}

	// Target word count
	if words, ok := data["target_words"].(int); ok && words > 0 {
		sb.WriteString(fmt.Sprintf("TARGET LENGTH: Approximately %d words\n\n", words))
	}

	// Review suggestions
	if suggestions, ok := data["suggestions"].(string); ok && suggestions != "" {
		sb.WriteString("=== REVIEW FEEDBACK & IMPROVEMENT SUGGESTIONS ===\n")
		sb.WriteString(suggestions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Rewrite the chapter addressing ALL the review feedback above. Focus on:")
	sb.WriteString("\n- Fixing all identified issues and inconsistencies")
	sb.WriteString("\n- Maintaining seamless continuity with previous chapters")
	sb.WriteString("\n- Preserving the core plot while improving execution")
	sb.WriteString("\n- Strengthening character voices and dialogue")
	sb.WriteString("\n- Improving pacing and tension where flagged")
	sb.WriteString("\n- Enhancing descriptions and emotional impact")
	sb.WriteString("\n- Keeping the same target word count")
	sb.WriteString("\n\nWrite only the improved chapter content, no meta-commentary or chapter headers.")

	return sb.String()
}
