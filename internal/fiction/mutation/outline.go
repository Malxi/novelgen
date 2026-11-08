// Outline patches: chapters and volumes, the blocking-issue guard around them, and the post-patch
// check they advertise.

package mutation

import (
	"encoding/json"
	"fmt"
	"novelgen/internal/contract/toolcmd"
	"strings"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
)

type outlineVolumePatch struct {
	ID              string                       `json:"id,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Summary         string                       `json:"summary,omitempty"`
	PayoffContract  *models.VolumePayoffContract `json:"payoff_contract,omitempty"`
	ChangedChapters []json.RawMessage            `json:"changed_chapters,omitempty"`
	ChangedEvents   []json.RawMessage            `json:"changed_events,omitempty"`
}

func runOutlinePatchMutation(root *store.Root, target, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	opts = opts.normalized()
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runOutlinePatchMutationLocked(root, target, id, patchBytes, opts, warnings)
		return err
	})
	return result, err
}

func runOutlinePatchMutationLocked(root *store.Root, target, id string, patchBytes []byte, opts MutationOptions, warnings []string) (*ToolPatchResult, error) {
	ctx := ToolProjectContext{Root: root}
	if err := ctx.LoadSetup(); err != nil {
		return nil, err
	}
	if err := ctx.LoadOutline(); err != nil {
		return nil, err
	}

	beforeOutline := CloneOutline(ctx.Outline)
	var changes []ToolPatchChange
	var err error
	switch target {
	case "chapter":
		changes, err = ApplyChapterPatch(ctx.Outline, id, patchBytes)
	case "volume":
		changes, err = ApplyVolumePatch(ctx.Outline, id, patchBytes)
	}
	if err != nil {
		return nil, err
	}
	warnings = appendNoChangeWarning(warnings, changes)

	scope := target
	check, err := RunToolPatchValidationCheckAt(root, ctx.Setup, ctx.Outline, scope, id)
	if err != nil {
		return nil, err
	}
	addToolCheckMeta(check, "project_root", root.Dir())
	globalCheck, err := RunToolPatchValidationCheckAt(root, ctx.Setup, ctx.Outline, "all", "")
	if err != nil {
		return nil, err
	}
	addToolCheckMeta(globalCheck, "project_root", root.Dir())
	if newBlocking := newBlockingPatchIssues(root, beforeOutline, ctx.Setup, globalCheck); len(newBlocking) > 0 {
		ctx.Outline = beforeOutline
		first := newBlocking[0]
		return nil, fmt.Errorf("patch rejected: global outline check introduced %d new blocking issue(s); first [%s/%s] %s: %s", len(newBlocking), first.Priority, first.Category, first.TargetID, first.Issue)
	}
	if opts.Apply && opts.Validate && check.Blocking {
		ctx.Outline = beforeOutline
		return nil, fmt.Errorf("patch rejected: quality/simulation check found blocking issues (critical=%d high=%d total=%d)", check.Summary.Critical, check.Summary.High, check.Summary.Total)
	}

	files := map[string]string{}
	if opts.Apply {
		checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeCompose, "outline", beforeOutline)
		if err != nil {
			return nil, err
		}
		OutlinePath := root.OutlineJSON()
		if err := root.SaveOutlineAt(OutlinePath, ctx.Outline); err != nil {
			return nil, fmt.Errorf("failed to save patched outline: %w", err)
		}
		mdPath, err := project.Open(root).SaveOutlineMarkdown(ctx.Outline)
		if err != nil {
			return nil, fmt.Errorf("failed to save outline markdown: %w", err)
		}
		files["outline"] = OutlinePath
		files["markdown"] = mdPath
		files["checkpoint"] = checkpointPath
		warnings = markMemoryDirtyAfterWrite(root, "outline:"+target+":"+id, warnings)
	} else {
		ctx.Outline = beforeOutline
	}

	result := ToolPatchResult{
		OK:          !check.Blocking,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      target,
		ID:          id,
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(opts.Apply, check, OutlinePatchPostCheckCommand(target, id)),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"validation":   toolPatchValidationMeta(check),
			"mode":         string(opts.Mode),
			"source":       opts.Source,
		},
	}
	return &result, nil
}

// runOutlineReplaceMutation commits a complete outline (compose gen/regen).
// It is the full-artifact counterpart of the incremental volume/chapter patch
// above. Host pipelines opt out of the second quality pass when they already
// ran their own repair gate; the CLI patch path always validates.
func runOutlineReplaceMutation(root *store.Root, outline *models.Outline, opts MutationOptions) (*ToolPatchResult, error) {
	if root == nil {
		return nil, fmt.Errorf("project root is required")
	}
	opts = opts.normalized()
	var result *ToolPatchResult
	err := root.WithMutationLock(func() error {
		var err error
		result, err = runOutlineReplaceMutationLocked(root, outline, opts)
		return err
	})
	return result, err
}

func runOutlineReplaceMutationLocked(root *store.Root, outline *models.Outline, opts MutationOptions) (*ToolPatchResult, error) {
	setup, err := root.LoadSetup()
	if err != nil {
		return nil, fmt.Errorf("failed to load story setup: %w", err)
	}
	before := &models.Outline{}
	exists, err := root.ExistsErr(root.OutlineJSON())
	if err != nil {
		return nil, fmt.Errorf("failed to inspect outline: %w", err)
	}
	if exists {
		before, err = root.LoadOutline()
		if err != nil {
			return nil, fmt.Errorf("failed to load existing outline: %w", err)
		}
	}

	candidate := CloneOutline(outline)
	if opts.Normalize {
		models.NormalizeOutline(candidate)
	}

	var check *ToolCheckResult
	if opts.Validate {
		check, err = RunToolPatchValidationCheckAt(root, setup, candidate, "all", "")
		if err != nil {
			return nil, err
		}
		addToolCheckMeta(check, "project_root", root.Dir())
		if newBlocking := newBlockingPatchIssues(root, before, setup, check); len(newBlocking) > 0 {
			first := newBlocking[0]
			return nil, fmt.Errorf("outline rejected: global check introduced %d new blocking issue(s); first [%s/%s] %s: %s", len(newBlocking), first.Priority, first.Category, first.TargetID, first.Issue)
		}
	}

	files := map[string]string{}
	changes := DiffStructFields("outline", *before, *candidate)
	if opts.Apply {
		if exists {
			checkpointPath, err := saveToolCheckpointJSON(root, store.ScopeCompose, "outline", before)
			if err != nil {
				return nil, err
			}
			files["checkpoint"] = checkpointPath
		}
		outlinePath := root.OutlineJSON()
		if err := root.SaveOutlineAt(outlinePath, candidate); err != nil {
			return nil, fmt.Errorf("failed to save outline: %w", err)
		}
		if opts.WriteDerived {
			mdPath, err := project.Open(root).SaveOutlineMarkdown(candidate)
			if err != nil {
				return nil, fmt.Errorf("failed to save outline markdown: %w", err)
			}
			files["markdown"] = mdPath
		}
		files["outline"] = outlinePath
	}

	ok := !opts.Validate || check == nil || !check.Blocking
	warnings := appendNoChangeWarning(nil, changes)
	warnings = append(warnings, removedOutlineChapterWarnings(root, before, candidate)...)
	if opts.Apply {
		warnings = markMemoryDirtyAfterWrite(root, "outline", warnings)
	}
	return &ToolPatchResult{
		OK:          ok,
		Applied:     opts.Apply,
		DryRun:      !opts.Apply,
		Target:      "outline",
		ID:          "outline",
		Changed:     changes,
		Check:       check,
		Files:       files,
		Warnings:    warnings,
		NextActions: buildPatchNextActions(opts.Apply, check, "novelgen tool check all --target outline --min-priority medium --max-issues 12"),
		Meta: map[string]interface{}{
			"project_root": root.Dir(),
			"mode":         string(opts.Mode),
			"source":       opts.Source,
		},
	}, nil
}

// removedOutlineChapterWarnings reports chapters that a whole-outline replace
// drops, including whether saved prose or a recap would be orphaned. The warning
// is emitted for dry runs too, so an agent can review the deletion before apply.
func removedOutlineChapterWarnings(root *store.Root, before, after *models.Outline) []string {
	if root == nil || before == nil || after == nil {
		return nil
	}
	remaining := map[string]bool{}
	for _, part := range after.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				remaining[chapter.ID] = true
			}
		}
	}
	var warnings []string
	for _, part := range before.Parts {
		for _, volume := range part.Volumes {
			for _, chapter := range volume.Chapters {
				if chapter.ID == "" || remaining[chapter.ID] {
					continue
				}
				detail := ""
				hasProse, proseErr := root.ExistsErr(root.ChapterFile(chapter.ID))
				hasRecap, recapErr := root.ExistsErr(root.RecapFile(chapter.ID))
				switch {
				case proseErr == nil && hasProse && recapErr == nil && hasRecap:
					detail = " (saved chapter content and recap exist)"
				case proseErr == nil && hasProse:
					detail = " (saved chapter content exists)"
				case recapErr == nil && hasRecap:
					detail = " (saved recap exists)"
				}
				warnings = append(warnings,
					fmt.Sprintf("outline replace removes chapter %s%s", chapter.ID, detail))
			}
		}
	}
	return warnings
}

func newBlockingPatchIssues(root *store.Root, before *models.Outline, setup *models.StorySetup, afterCheck *ToolCheckResult) []models.ReviewSuggestion {
	if before == nil || setup == nil || afterCheck == nil {
		return nil
	}
	beforeCheck, err := RunToolPatchValidationCheckAt(root, setup, before, "all", "")
	if err != nil {
		return nil
	}
	beforeBlocking := map[string]bool{}
	for _, issue := range beforeCheck.Issues {
		if toolPatchIssueIsBlocking(issue) {
			beforeBlocking[toolPatchIssueSignature(issue)] = true
		}
	}
	var out []models.ReviewSuggestion
	for _, issue := range afterCheck.Issues {
		if !toolPatchIssueIsBlocking(issue) {
			continue
		}
		if beforeBlocking[toolPatchIssueSignature(issue)] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func toolPatchIssueIsBlocking(issue models.ReviewSuggestion) bool {
	rank, ok := reviewPriorityRank(issue.Priority)
	return ok && rank >= 3
}

func toolPatchIssueSignature(issue models.ReviewSuggestion) string {
	return strings.Join([]string{
		NormalizeKey(issue.Category),
		strings.ToLower(strings.TrimSpace(issue.TargetID)),
		strings.ToLower(strings.TrimSpace(issue.TargetName)),
		strings.ToLower(strings.TrimSpace(issue.Issue)),
	}, "\x00")
}

func OutlinePatchPostCheckCommand(target, id string) string {
	target = NormalizeKey(target)
	id = strings.TrimSpace(id)
	switch target {
	case "volume":
		if id != "" {
			return toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "volume", ID: id, MinPriority: "medium", MaxIssues: 12})
		}
	case "chapter":
		if id != "" {
			return toolcmd.Check(toolcmd.CheckOptions{Target: "outline", Scope: "chapter", ID: id, MinPriority: "low", MaxIssues: 8})
		}
	}
	return ""
}

func ApplyChapterPatch(outline *models.Outline, chapterID string, patchBytes []byte) ([]ToolPatchChange, error) {
	chapter := outline.GetChapterByID(chapterID)
	if chapter == nil {
		return nil, fmt.Errorf("chapter %q not found", chapterID)
	}
	var validated models.Chapter
	if err := decodeStrictToolPatch(patchBytes, &validated); err != nil {
		return nil, fmt.Errorf("chapter patch does not match the chapter shape: %w", err)
	}
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(patchBytes, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if strings.TrimSpace(probe.ID) != "" && probe.ID != chapter.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", probe.ID, chapter.ID)
	}
	before := *chapter
	merged, err := MergeJSONPatchObject(before, patchBytes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merged.ID) != "" && merged.ID != before.ID {
		return nil, fmt.Errorf("patch id %q does not match target chapter %q", merged.ID, before.ID)
	}
	merged.ID = before.ID
	*chapter = merged
	return DiffStructFields("outline.chapter."+chapterID, before, merged), nil
}

func ApplyVolumePatch(outline *models.Outline, volumeID string, patchBytes []byte) ([]ToolPatchChange, error) {
	volume := outline.GetVolumeByID(volumeID)
	if volume == nil {
		return nil, fmt.Errorf("volume %q not found", volumeID)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(patchBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse patch JSON: %w", err)
	}
	if _, ok := raw["chapters"]; ok {
		return nil, fmt.Errorf("volume patch must use changed_chapters; replacing chapters is not allowed")
	}
	var patch outlineVolumePatch
	if err := decodeStrictToolPatch(patchBytes, &patch); err != nil {
		return nil, fmt.Errorf("volume patch does not match the volume shape: %w", err)
	}
	if strings.TrimSpace(patch.ID) != "" && patch.ID != volume.ID {
		return nil, fmt.Errorf("patch id %q does not match target volume %q", patch.ID, volume.ID)
	}

	before := *volume
	if _, ok := raw["title"]; ok {
		volume.Title = patch.Title
	}
	if _, ok := raw["summary"]; ok {
		volume.Summary = patch.Summary
	}
	if _, ok := raw["payoff_contract"]; ok {
		volume.PayoffContract = models.MergeVolumePayoffContract(volume.PayoffContract, patch.PayoffContract)
	}
	changes := DiffStructFields("outline.volume."+volumeID, before, *volume)

	for _, rawChapter := range patch.ChangedChapters {
		var validatedChapter models.Chapter
		if err := decodeStrictToolPatch(rawChapter, &validatedChapter); err != nil {
			return nil, fmt.Errorf("changed_chapters item does not match the chapter shape: %w", err)
		}
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(rawChapter, &probe); err != nil {
			return nil, fmt.Errorf("failed to parse changed chapter patch: %w", err)
		}
		if strings.TrimSpace(probe.ID) == "" {
			return nil, fmt.Errorf("changed_chapters item missing id")
		}
		found := false
		for i := range volume.Chapters {
			if volume.Chapters[i].ID != probe.ID {
				continue
			}
			found = true
			beforeChapter := volume.Chapters[i]
			merged, err := MergeJSONPatchObject(beforeChapter, rawChapter)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(merged.ID) != "" && merged.ID != beforeChapter.ID {
				return nil, fmt.Errorf("patch chapter id %q does not match target chapter %q", merged.ID, beforeChapter.ID)
			}
			merged.ID = beforeChapter.ID
			volume.Chapters[i] = merged
			changes = append(changes, DiffStructFields("outline.chapter."+probe.ID, beforeChapter, merged)...)
			break
		}
		if !found {
			return nil, fmt.Errorf("changed chapter %q is not in target volume %q", probe.ID, volumeID)
		}
	}
	for _, rawEvent := range patch.ChangedEvents {
		eventChanges, err := applyVolumeEventPatch(volume, volumeID, rawEvent)
		if err != nil {
			return nil, err
		}
		changes = append(changes, eventChanges...)
	}
	return changes, nil
}

func applyVolumeEventPatch(volume *models.Volume, volumeID string, rawEvent json.RawMessage) ([]ToolPatchChange, error) {
	var probe struct {
		ChapterID  string `json:"chapter_id"`
		EventIndex *int   `json:"event_index"`
		Index      *int   `json:"index"`
	}
	if err := json.Unmarshal(rawEvent, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch: %w", err)
	}
	chapterID := strings.TrimSpace(probe.ChapterID)
	if chapterID == "" {
		return nil, fmt.Errorf("changed_events item missing chapter_id")
	}
	eventIndex := -1
	if probe.EventIndex != nil {
		eventIndex = *probe.EventIndex
	} else if probe.Index != nil {
		eventIndex = *probe.Index
	} else {
		return nil, fmt.Errorf("changed_events item missing event_index")
	}
	if eventIndex < 0 {
		return nil, fmt.Errorf("changed_events item has negative event_index %d", eventIndex)
	}
	var patchFields map[string]json.RawMessage
	if err := json.Unmarshal(rawEvent, &patchFields); err != nil {
		return nil, fmt.Errorf("failed to parse changed event patch fields: %w", err)
	}
	delete(patchFields, "chapter_id")
	delete(patchFields, "event_index")
	delete(patchFields, "index")
	if len(patchFields) == 0 {
		return nil, fmt.Errorf("changed_events item for %s[%d] has no event fields to patch", chapterID, eventIndex)
	}
	patchBytes, err := json.Marshal(patchFields)
	if err != nil {
		return nil, fmt.Errorf("failed to encode changed event patch: %w", err)
	}
	var validatedEvent models.Event
	if err := decodeStrictToolPatch(patchBytes, &validatedEvent); err != nil {
		return nil, fmt.Errorf("changed_events item does not match the event shape: %w", err)
	}
	for chapterIdx := range volume.Chapters {
		if volume.Chapters[chapterIdx].ID != chapterID {
			continue
		}
		if eventIndex >= len(volume.Chapters[chapterIdx].Events) {
			return nil, fmt.Errorf("changed_events index %d out of range for chapter %q", eventIndex, chapterID)
		}
		before := volume.Chapters[chapterIdx].Events[eventIndex]
		merged, err := MergeJSONPatchObject(before, patchBytes)
		if err != nil {
			return nil, err
		}
		volume.Chapters[chapterIdx].Events[eventIndex] = merged
		return DiffStructFields(fmt.Sprintf("outline.event.%s.%d", chapterID, eventIndex), before, merged), nil
	}
	return nil, fmt.Errorf("changed event chapter %q is not in target volume %q", chapterID, volumeID)
}
