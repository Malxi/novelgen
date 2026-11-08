package project

import (
	"fmt"
	"strings"

	models "novelgen/internal/contract/story"
	"novelgen/internal/platform/store"
)

// ChapterStatus is one outline chapter with the state of its saved prose.
type ChapterStatus struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	VolumeID string `json:"volume_id,omitempty"`
	Written  bool   `json:"written"`
	Words    int    `json:"words,omitempty"`
}

// OutlineStatus summarizes the canonical outline.
type OutlineStatus struct {
	Parts    int `json:"parts"`
	Volumes  int `json:"volumes"`
	Chapters int `json:"chapters"`
}

// NextStep is the single best next creative step for a project.
type NextStep struct {
	Stage    string `json:"stage"`
	TargetID string `json:"targetId"`
	Reason   string `json:"reason,omitempty"`
}

// ProjectStatus is the shared, read-only project projection used by the CLI
// hook/MCP adapters. It deliberately owns the next-step rules once instead of
// letting each adapter re-derive them.
type ProjectStatus struct {
	Root      string                `json:"root,omitempty"`
	Name      string                `json:"name,omitempty"`
	Language  string                `json:"language,omitempty"`
	Structure models.StoryStructure `json:"structure"`
	Setup     bool                  `json:"setup"`
	Outline   OutlineStatus         `json:"outline"`
	Chapters  []ChapterStatus       `json:"chapters"`
	Next      NextStep              `json:"next"`
	Warnings  []string              `json:"warnings,omitempty"`
}

// Status loads the project into one deterministic status projection.
//
// Missing and unreadable artifacts are kept distinct: a missing setup/outline
// is a normal early-project state, while a read or stat failure is reported in
// Warnings instead of being collapsed into "not written".
func (b *Book) Status() (ProjectStatus, error) {
	root := b.Root()
	if root == nil {
		return ProjectStatus{}, store.ErrNilRoot
	}
	status := ProjectStatus{
		Root:     root.Dir(),
		Chapters: make([]ChapterStatus, 0),
	}

	if config, err := b.LoadConfig(); err == nil && config != nil {
		status.Name = config.Name
		status.Language = config.Language
		status.Structure = config.Structure
	} else if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to load novel.json: %v", err))
	}

	setupExists, err := root.ExistsErr(root.SetupJSON())
	switch {
	case err != nil:
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to inspect story setup: %v", err))
	case setupExists:
		if setup, loadErr := b.LoadSetup(); loadErr != nil || setup == nil {
			if loadErr != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("failed to load story setup: %v", loadErr))
			}
		} else {
			status.Setup = true
		}
	}

	outlineExists, err := root.ExistsErr(root.OutlineJSON())
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("failed to inspect outline: %v", err))
	}
	var outline *models.Outline
	if err == nil && outlineExists {
		outline, err = b.LoadOutline()
		if err != nil || outline == nil {
			if err != nil {
				status.Warnings = append(status.Warnings, fmt.Sprintf("failed to load outline: %v", err))
			}
			outline = nil
		}
	}
	emptyVolume := ""
	firstUnwritten := ""
	if outline != nil {
		status.Outline.Parts = len(outline.Parts)
		for _, part := range outline.Parts {
			status.Outline.Volumes += len(part.Volumes)
			for _, volume := range part.Volumes {
				if len(volume.Chapters) == 0 && emptyVolume == "" {
					emptyVolume = volume.ID
				}
				for _, chapter := range volume.Chapters {
					status.Outline.Chapters++
					item := ChapterStatus{ID: chapter.ID, Title: chapter.Title, VolumeID: volume.ID}
					exists, statErr := root.ExistsErr(root.ChapterFile(chapter.ID))
					switch {
					case statErr != nil:
						status.Warnings = append(status.Warnings,
							fmt.Sprintf("failed to inspect chapter %s: %v", chapter.ID, statErr))
					case exists:
						item.Written = true
						if data, readErr := root.Read(root.ChapterFile(chapter.ID)); readErr != nil {
							status.Warnings = append(status.Warnings,
								fmt.Sprintf("failed to read chapter %s: %v", chapter.ID, readErr))
						} else {
							item.Words = len([]rune(strings.TrimSpace(string(data))))
						}
					case firstUnwritten == "":
						firstUnwritten = chapter.ID
					}
					status.Chapters = append(status.Chapters, item)
				}
			}
		}
	}

	switch {
	case !status.Setup:
		status.Next = NextStep{Stage: "setup", TargetID: "setup", Reason: "故事设定尚未生成。"}
	case status.Outline.Parts == 0 || status.Outline.Volumes == 0:
		status.Next = NextStep{Stage: "outline", Reason: "大纲骨架尚未生成。"}
	case emptyVolume != "":
		status.Next = NextStep{Stage: "outline", TargetID: "volume:" + emptyVolume, Reason: "有卷还没有章节。"}
	case firstUnwritten != "":
		status.Next = NextStep{Stage: "write", TargetID: "chapter:" + firstUnwritten, Reason: "仍有章节未写正文。"}
	default:
		status.Next = NextStep{Stage: "review", Reason: "正文已完成，可以进入审校。"}
	}
	return status, nil
}
