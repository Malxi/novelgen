package store

import (
	"path/filepath"
	"strings"

	"novelgen/internal/utils"
)

// Layout directories and file names. These constants are the canonical
// spelling of the project layout documented in docs/ARCHITECTURE.md §7.
const (
	novelFileName       = "novel.json"
	llmConfigName       = "llm_config.json"
	storyDirName        = "story"
	setupDirName        = "setup"
	composeDirName      = "compose"
	craftDirName        = "craft"
	recapsDirName       = "recaps"
	reviewsDirName      = "reviews"
	rpgDirName          = "rpg"
	chaptersDirName     = "chapters"
	draftsDirName       = "drafts"
	logsDirName         = "logs"
	checkpointsName     = "checkpoints"
	backupsName         = "backups"
	setupJSONName       = "story_setup.json"
	setupMarkdownName   = "story_setup.md"
	outlineJSONName     = "outline.json"
	outlineMarkdownName = "outline.md"
)

// CraftKind identifies one of the four craft element files.
type CraftKind string

const (
	CraftCharacters    CraftKind = "characters"
	CraftLocations     CraftKind = "locations"
	CraftItems         CraftKind = "items"
	CraftOrganizations CraftKind = "organizations"
)

// CraftKinds lists the craft element files in their canonical order.
func CraftKinds() []CraftKind {
	return []CraftKind{CraftCharacters, CraftLocations, CraftItems, CraftOrganizations}
}

// FileName returns the craft file's base name, for example "characters.json".
func (k CraftKind) FileName() string { return string(k) + ".json" }

// Valid reports whether k is one of the four known craft kinds.
func (k CraftKind) Valid() bool {
	for _, known := range CraftKinds() {
		if k == known {
			return true
		}
	}
	return false
}

// Scope names a directory that carries derived side files: checkpoints written
// before a patch, and backups written before a regeneration.
type Scope string

const (
	ScopeSetup    Scope = "setup"
	ScopeCompose  Scope = "compose"
	ScopeCraft    Scope = "craft"
	ScopeRecaps   Scope = "recaps"
	ScopeChapters Scope = "chapters"
)

// NovelJSON is the project config file.
func (r *Root) NovelJSON() string { return filepath.Join(r.Dir(), novelFileName) }

// BooksDirName is the directory holding one subdirectory per book. The web UI
// and the standalone developer tools address projects that way; the CLI itself
// resolves a project by its novel.json, so this constant exists purely so the
// convention has one spelling.
const BooksDirName = "books"

// BookDir returns <booksRoot>/books/<name>, the layout the developer tools and
// the web UI use to address a book by name.
func BookDir(booksRoot, name string) string {
	if strings.TrimSpace(booksRoot) == "" {
		booksRoot = "."
	}
	return filepath.Join(booksRoot, BooksDirName, name)
}

// ResolvePath resolves a caller-supplied path against the project root, so a
// relative path such as "story/reviews/outline_review.json" means "inside this
// project" rather than "inside the working directory". Absolute paths are
// returned unchanged, and an empty path stays empty so callers can keep their
// "use the default" branches.
func (r *Root) ResolvePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.Dir(), path)
}

// LLMConfigJSON is the optional project-local LLM configuration.
func (r *Root) LLMConfigJSON() string { return filepath.Join(r.Dir(), llmConfigName) }

// StoryDir is the directory holding all story state.
func (r *Root) StoryDir() string { return filepath.Join(r.Dir(), storyDirName) }

// SetupDir is story/setup.
func (r *Root) SetupDir() string { return filepath.Join(r.StoryDir(), setupDirName) }

// SetupJSON is story/setup/story_setup.json.
func (r *Root) SetupJSON() string { return filepath.Join(r.SetupDir(), setupJSONName) }

// SetupMarkdown is story/setup/story_setup.md, the human-readable rendering.
func (r *Root) SetupMarkdown() string { return filepath.Join(r.SetupDir(), setupMarkdownName) }

// ComposeDir is story/compose.
func (r *Root) ComposeDir() string { return filepath.Join(r.StoryDir(), composeDirName) }

// ComposeFile returns a named file inside story/compose, so the pipeline's
// progress files, review reports and normalization reports stay spelled in one
// place.
func (r *Root) ComposeFile(name string) string { return filepath.Join(r.ComposeDir(), name) }

// OutlineJSON is story/compose/outline.json, the canonical story outline.
func (r *Root) OutlineJSON() string { return r.ComposeFile(outlineJSONName) }

// OutlineMarkdown is story/compose/outline.md, the derived outline rendering.
func (r *Root) OutlineMarkdown() string { return r.ComposeFile(outlineMarkdownName) }

// CraftDir is story/craft.
func (r *Root) CraftDir() string { return filepath.Join(r.StoryDir(), craftDirName) }

// CraftFile returns one of the four craft element files.
func (r *Root) CraftFile(kind CraftKind) string {
	return filepath.Join(r.CraftDir(), kind.FileName())
}

// CraftKindForFile maps a craft file name ("characters.json") to its kind.
func CraftKindForFile(name string) (CraftKind, bool) {
	kind := CraftKind(strings.TrimSuffix(filepath.Base(name), ".json"))
	return kind, kind.Valid()
}

// RecapsDir is story/recaps.
func (r *Root) RecapsDir() string { return filepath.Join(r.StoryDir(), recapsDirName) }

// RecapFile is story/recaps/<chapterID>.json.
func (r *Root) RecapFile(chapterID string) string {
	return filepath.Join(r.RecapsDir(), chapterID+".json")
}

// ReviewsDir is story/reviews.
func (r *Root) ReviewsDir() string { return filepath.Join(r.StoryDir(), reviewsDirName) }

// ReviewFile is story/reviews/<name>, used for per-volume reviews and for the
// outline/cross-volume review reports.
func (r *Root) ReviewFile(name string) string { return filepath.Join(r.ReviewsDir(), name) }

// VolumeReviewFile is story/reviews/<volumeID>_review.json.
func (r *Root) VolumeReviewFile(volumeID string) string {
	return r.ReviewFile(volumeID + "_review.json")
}

// RPGDir is story/rpg, holding the phased DSL fragments.
func (r *Root) RPGDir() string { return filepath.Join(r.StoryDir(), rpgDirName) }

// RPGFile is story/rpg/<name>.
func (r *Root) RPGFile(name string) string { return filepath.Join(r.RPGDir(), name) }

// ChaptersDir is the directory of finished chapter markdown.
func (r *Root) ChaptersDir() string { return filepath.Join(r.Dir(), chaptersDirName) }

// ChapterFile is the canonical file for a chapter ID,
// chapters/chapter-<id>.md.
func (r *Root) ChapterFile(chapterID string) string {
	return filepath.Join(r.ChaptersDir(), "chapter-"+chapterID+".md")
}

// ChapterCandidates lists the file names a chapter may have been saved under,
// most canonical first: chapters/chapter-<id>.md, the legacy numeric form
// (chapter-3.md for "chap_1_2_3"), and finally the bare chapters/<id>.md that
// the web UI and export used to read. Readers walk this list instead of
// re-deriving the fallbacks, which used to differ per caller.
func (r *Root) ChapterCandidates(chapterID string) []string {
	paths := []string{r.ChapterFile(chapterID)}
	if number := utils.ExtractChapterNumber(chapterID); number != "" && number != chapterID {
		paths = append(paths, r.ChapterFile(number))
	}
	paths = append(paths, filepath.Join(r.ChaptersDir(), chapterID+".md"))
	return paths
}

// DraftsDir is the legacy drafts directory.
func (r *Root) DraftsDir() string { return filepath.Join(r.Dir(), draftsDirName) }

// DraftFile is drafts/<chapterID>.md.
func (r *Root) DraftFile(chapterID string) string {
	return filepath.Join(r.DraftsDir(), chapterID+".md")
}

// LogsDir is the directory of run logs.
func (r *Root) LogsDir() string { return filepath.Join(r.Dir(), logsDirName) }

// LogFile is logs/<name>.
func (r *Root) LogFile(name string) string { return filepath.Join(r.LogsDir(), name) }

// PromptLogsDir, ResponseLogsDir, AICallLogsDir, AgentLiveLogsDir and
// WriteContextLogsDir are the log streams the CLI and the web UI both read.
func (r *Root) PromptLogsDir() string       { return r.LogFile("prompts") }
func (r *Root) ResponseLogsDir() string     { return r.LogFile("responses") }
func (r *Root) AICallLogsDir() string       { return r.LogFile("ai_calls") }
func (r *Root) AgentLiveLogsDir() string    { return r.LogFile("agent-live") }
func (r *Root) WriteContextLogsDir() string { return r.LogFile("write_contexts") }

// UsageLog is logs/usage.jsonl, the append-only LLM usage ledger.
func (r *Root) UsageLog() string { return r.LogFile("usage.jsonl") }

// CloneManifest is logs/clone_manifest.json, written by `project clone`.
func (r *Root) CloneManifest() string { return r.LogFile("clone_manifest.json") }

// scopeDir maps a scope to the directory it belongs under.
func (r *Root) scopeDir(scope Scope) string {
	if scope == ScopeChapters {
		return r.ChaptersDir()
	}
	return filepath.Join(r.StoryDir(), string(scope))
}

// CheckpointsDir is where the pre-patch snapshots for a scope are written.
func (r *Root) CheckpointsDir(scope Scope) string {
	return filepath.Join(r.scopeDir(scope), checkpointsName)
}

// BackupsDir is where the pre-regeneration snapshots for a scope are written.
func (r *Root) BackupsDir(scope Scope) string {
	return filepath.Join(r.scopeDir(scope), backupsName)
}
