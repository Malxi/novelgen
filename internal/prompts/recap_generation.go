package prompts

import (
	"fmt"
	"strings"
)

// registerRecapPrompts registers recap extraction prompts
func registerRecapPrompts(pm *PromptManager) {
	pm.Register(&PromptTemplate{
		Skill:        SkillChapterRecap,
		Name:         "extract",
		Description:  "Extract a compact canonical recap from chapter text for continuity",
		SystemPrompt: buildRecapSystemPrompt(),
		OutputFormat: FormatJSON,
		OutputModel:  &RecapOutput{},
	})
}

// RecapOutput is a light wrapper to generate schema
type RecapOutput struct {
	ChapterID       string   `json:"chapter_id"`
	Title           string   `json:"title"`
	Location        string   `json:"location"`
	Time            string   `json:"time"`
	Present         []string `json:"present"`
	PlotBeats       []string `json:"plot_beats"`
	Decisions       []string `json:"decisions"`
	Reveals         []string `json:"reveals"`
	Unresolved      []string `json:"unresolved"`
	Promises        []string `json:"promises"`
	Items           []string `json:"items"`
	Status          []string `json:"status"`
	LastLine        string   `json:"last_line"`
	Cliffhanger     string   `json:"cliffhanger"`
	NextOpeningHint string   `json:"next_opening_hint"`
}

func buildRecapSystemPrompt() string {
	return `You are a story editor. Your job is to extract HIGH-SIGNAL continuity information from a chapter.

RULES:
- Output MUST be valid JSON matching the provided schema.
- Prefer concrete facts over interpretation.
- Be compact and specific.
- Do not invent names, locations, items, or events that are not in the chapter.
- If a field is unknown, use an empty string or empty array.

FOCUS ON CONTINUITY:
- Where are we at the end of the chapter (location/time)?
- Who is present?
- What is the exact unresolved question / conflict?
- What promises/flags were made?
- Any item ownership/status changes?
- Injuries/mood/status that must carry over.
- Capture the last line / last moment as accurately as possible.

SCENE-ANCHOR FOR NEXT CHAPTER:
- The field next_opening_hint MUST be 1–3 sentences that begin the next chapter by directly continuing from the final moment.
- It MUST be compatible with the chapter's last_line/cliffhanger and MUST NOT contradict them.
- It MUST visibly "pick up" from last_line: reuse a concrete phrase/entity/action from last_line (a short quoted fragment is OK) so the reader can feel the immediate continuation.
- Keep the same location/time and in-progress action/conversation unless the text clearly indicates a time skip.
- Be concrete (who is doing what, what is being said/seen) and avoid vague advice.

LANGUAGE:
- Keep content in the same language as the input chapter (usually Chinese).`
}

func buildRecapUserPrompt(data map[string]interface{}) string {
	chapterID, _ := data["chapter_id"].(string)
	title, _ := data["title"].(string)
	text, _ := data["text"].(string)

	var sb strings.Builder
	sb.WriteString("CHAPTER METADATA:\n")
	sb.WriteString(fmt.Sprintf("ChapterID: %s\n", chapterID))
	sb.WriteString(fmt.Sprintf("Title: %s\n\n", title))

	if feedback, ok := data["feedback"].(string); ok && feedback != "" {
		sb.WriteString("RECAP FIX FEEDBACK (MUST ADDRESS):\n")
		sb.WriteString(feedback)
		sb.WriteString("\n\n")
	}

	sb.WriteString("CHAPTER TEXT:\n")
	sb.WriteString(text)

	return sb.String()
}
