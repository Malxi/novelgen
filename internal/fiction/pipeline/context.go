package writepipe

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/agents"
	novelcont "novelgen/internal/fiction/domain/continuity"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/mutation"
)

type WriteErrorCollector struct {
	mu   sync.Mutex
	errs []error
}

func (c *WriteErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = append(c.errs, err)
}

func (c *WriteErrorCollector) Addf(format string, args ...interface{}) {
	c.Add(fmt.Errorf(format, args...))
}

func (c *WriteErrorCollector) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.errs) == 0 {
		return nil
	}
	return errors.Join(c.errs...)
}

func loadChapterContent(book *project.Book, chapterID string) string {
	// Prefer the final chapter; the chapter file-name fallbacks (full id, legacy
	// numeric, bare id) live in the project store so this cannot drift from the
	// other readers again.
	if book == nil {
		return ""
	}
	// Best-effort context: an unreadable chapter degrades to the draft fallback
	// rather than failing the run. The paths that act on the value (chapter
	// checks, patch before-state, post-save recovery) handle the error.
	if content, _, err := book.Root().LoadChapterContent(chapterID); err == nil && content != "" {
		return content
	}
	draft, _ := book.LoadDraftContent(chapterID)
	return draft
}

func LoadChapterContext(book *project.Book, outline *models.Outline, targetChapter *models.Chapter, contextCount int) *agents.ChapterContext {
	context := &agents.ChapterContext{
		Current:  targetChapter,
		Previous: make([]*agents.ContextChapter, 0),
		Next:     make([]*agents.ContextChapter, 0),
		Recap:    "",
	}
	if book == nil {
		return context
	}
	if _, _, _, organizations, err := book.LoadElements(); err == nil {
		context.Craft = BuildOrganizationWriteContext(targetChapter, organizations)
	}

	allChapters := project.AllChapters(outline)

	// Find target chapter index
	targetIndex := -1
	for i, ch := range allChapters {
		if ch.ID == targetChapter.ID {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return context
	}

	if recapJSON := novelcont.LoadPreviousRecapJSON(book, outline, targetChapter); strings.TrimSpace(recapJSON) != "" {
		context.Recap = recapJSON
	} else if draftRecap := novelcont.LoadPreviousDraftRecap(book, outline, targetChapter); strings.TrimSpace(draftRecap) != "" {
		context.Recap = draftRecap
	}

	// Load previous chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex - i
		if idx >= 0 {
			ch := allChapters[idx]
			content := loadChapterContent(book, ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				// Ensure there's at least an offline recap persisted for the previous
				// chapter so later steps (e.g., transition checks / future drafts) can
				// use it even when generation is out-of-order.
				if i == 1 && content != "" {
					novelcont.PersistOfflineRecapIfMissing(book, ch, content)
				}

				// Note: Recap extraction is handled separately via recap command
				context.Previous = append([]*agents.ContextChapter{{
					Chapter: ch,
					Content: content,
				}}, context.Previous...)
			}
		}
	}

	// Load next chapters (prefer final, fallback to draft, then outline)
	for i := 1; i <= contextCount; i++ {
		idx := targetIndex + i
		if idx < len(allChapters) {
			ch := allChapters[idx]
			content := loadChapterContent(book, ch.ID)
			// If no final/draft, build from outline
			if content == "" {
				content = buildChapterContentFromOutline(ch)
			}
			if content != "" {
				context.Next = append(context.Next, &agents.ContextChapter{
					Chapter: ch,
					Content: content,
				})
			}
		}
	}

	return context
}

func BuildOrganizationWriteContext(chapter *models.Chapter, organizations map[string]*models.Organization) string {
	if chapter == nil || len(organizations) == 0 {
		return ""
	}

	haystackParts := []string{
		chapter.ID,
		chapter.Title,
		chapter.Summary,
		chapter.Location,
		chapter.StateAnchor.Location,
		strings.Join(chapter.Characters, " "),
		strings.Join(chapter.GetBeats(), " "),
		chapter.Conflict,
		chapter.StateChange,
	}
	for _, scene := range chapter.Scenes {
		haystackParts = append(haystackParts, scene.POV, scene.Goal, scene.Location, strings.Join(scene.Characters, " "), strings.Join(scene.Beats, " "))
	}
	for _, enemy := range chapter.Enemies {
		haystackParts = append(haystackParts, enemy.Name, enemy.Faction, enemy.Tier, enemy.BossID, enemy.Status, enemy.Context)
	}
	for _, entry := range chapter.ResourceLedger {
		haystackParts = append(haystackParts, entry.Item, entry.Reason)
	}
	for _, advance := range chapter.StorylineAdvances {
		haystackParts = append(haystackParts, advance.StorylineName, advance.Stage, advance.Change, advance.Consequence, advance.Pressure)
	}
	for _, planted := range chapter.Mysteries.Planted {
		haystackParts = append(haystackParts, planted.ID, planted.Clue)
	}
	for _, resolved := range chapter.Mysteries.Resolved {
		haystackParts = append(haystackParts, resolved.ID, resolved.Resolution)
	}
	for _, event := range chapter.Events {
		haystackParts = append(haystackParts,
			event.GetActor(),
			event.GetAction(),
			event.GetTarget(),
			event.GetTargetType(),
			strings.Join(event.Characters, " "),
			event.Context,
			event.Details,
			event.Result,
		)
	}
	haystack := strings.ToLower(strings.Join(haystackParts, " "))

	type organizationMatch struct {
		id    string
		org   *models.Organization
		score int
	}
	matches := make([]organizationMatch, 0, len(organizations))
	for id, org := range organizations {
		if org == nil {
			continue
		}
		score := 0
		candidates := []string{id, org.Name, org.Headquarters, org.Leadership}
		candidates = append(candidates, org.Members...)
		candidates = append(candidates, org.Allies...)
		candidates = append(candidates, org.Enemies...)
		candidates = append(candidates, org.DSLTags...)
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate != "" && strings.Contains(haystack, strings.ToLower(candidate)) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		matches = append(matches, organizationMatch{id: id, org: org, score: score})
	}
	if len(matches) == 0 {
		return ""
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].id < matches[j].id
	})

	if len(matches) > 8 {
		matches = matches[:8]
	}
	var sb strings.Builder
	sb.WriteString("ORGANIZATIONS:\n")
	for _, match := range matches {
		org := match.org
		name := strings.TrimSpace(org.Name)
		if name == "" {
			name = match.id
		}
		sb.WriteString(fmt.Sprintf("- %s", name))
		if strings.TrimSpace(org.Type) != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.TrimSpace(org.Type)))
		}
		sb.WriteString(": ")
		parts := []string{}
		appendPart := func(label, value string) {
			value = strings.TrimSpace(value)
			if value == "" {
				return
			}
			if label == "" {
				parts = append(parts, value)
			} else {
				parts = append(parts, label+"="+value)
			}
		}
		appendPart("", org.Description)
		appendPart("leadership", org.Leadership)
		appendPart("headquarters", org.Headquarters)
		appendPart("goals", strings.Join(org.Goals, " | "))
		appendPart("resources", strings.Join(org.Resources, " | "))
		appendPart("allies", strings.Join(org.Allies, " | "))
		appendPart("enemies", strings.Join(org.Enemies, " | "))
		appendPart("", org.Significance)
		sb.WriteString(strings.Join(parts, "; "))
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

// buildChapterContentFromOutline builds chapter content from outline data
// Used as fallback when no draft or final content exists
func buildChapterContentFromOutline(chapter *models.Chapter) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chapter: %s\n", chapter.ID))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", chapter.Summary))
	if len(chapter.GetBeats()) > 0 {
		sb.WriteString("Beats:\n")
		for i, beat := range chapter.GetBeats() {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, beat))
		}
	}
	return sb.String()
}

// SaveFinalChapter saves the generated final chapter content into an explicit
// project.
func SaveFinalChapter(ctx context.Context, book *project.Book, chapter *models.Chapter, content string) error {
	if book == nil {
		return fmt.Errorf("project is required")
	}
	if chapter == nil {
		return fmt.Errorf("chapter is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// The write/polish pipelines run their own pre-save checks and post-save
	// verification. Commit the generated chapter through the same mutation
	// surface used by `tool patch chapter`, without duplicating that quality
	// pass here.
	_, err := mutation.NewMutationService(book.Root()).ReplaceChapterObject(ctx, chapter, content, mutation.MutationOptions{
		Validate: false,
		Source:   "write-pipeline",
	})
	return err
}

// ChapterNarrativeUnitsForLog counts the narrative units the project reports
// for a chapter, used to compare generated length against the target.
func ChapterNarrativeUnitsForLog(chapter *models.Chapter, content string) int {
	return project.ChapterNarrativeUnits(chapter, content)
}

func ResolveVolumeID(outline *models.Outline, volumeFlag, partFlag string) string {
	// If already full ID, return as is
	if strings.Contains(volumeFlag, "-") {
		return volumeFlag
	}

	// Try to resolve with part flag
	partID := partFlag
	if partID == "" {
		partID = "P1" // Default to first part
	}
	if !strings.HasPrefix(partID, "P") {
		partID = "P" + partID
	}

	volumeID := volumeFlag
	if !strings.HasPrefix(volumeID, "V") {
		volumeID = "V" + volumeID
	}

	return partID + "-" + volumeID
}
