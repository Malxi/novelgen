// Package toolcmd holds the canonical builders for the `novelgen tool ...`
// commands. The tool plane advertises these commands in its navigation output
// and the agents instruct the SDK agent to run them; keeping one builder for
// each command shape means both sides always agree on flag spelling and flag
// order.
//
// Flag order matters: the runner matches a command against the workflow
// allowlist by canonical suffix (`canonical_novelgen_command`), so a builder
// emits flags in a fixed order and optional flags are omitted rather than left
// empty. Quoting does not affect that match — the runner strips quotes before
// comparing — but ids and names are still quoted so an agent can copy the
// command into a shell unchanged.
package toolcmd

import (
	"strconv"
	"strings"
)

// prefix is the CLI invocation every tool command starts with.
const prefix = "novelgen tool "

// CheckOptions selects the flags of a scoped check command. Kind defaults to
// "all".
type CheckOptions struct {
	Kind        string // all (default), quality, simulation, schema
	Target      string // setup, outline, craft, chapter, recap
	Scope       string // outline all|volume|chapter, craft kind, chapter
	ID          string
	Category    string
	MinPriority string
	MaxIssues   int
	TargetWords int
}

// Check builds a scoped check command with the optional flags in their
// canonical order: --target, --scope, --id, --category, --min-priority,
// --max-issues, --target-words.
func Check(opts CheckOptions) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("check ")
	b.WriteString(orDefault(opts.Kind, "all"))
	writeFlag(&b, "--target", opts.Target, false)
	writeFlag(&b, "--scope", opts.Scope, false)
	writeFlag(&b, "--id", opts.ID, true)
	writeFlag(&b, "--category", opts.Category, false)
	writeFlag(&b, "--min-priority", opts.MinPriority, false)
	writeInt(&b, "--max-issues", opts.MaxIssues)
	writeInt(&b, "--target-words", opts.TargetWords)
	return b.String()
}

// ChapterCheckOptions selects the optional flags of a focused chapter check.
type ChapterCheckOptions struct {
	Kind        string // "all" (default), "quality" or "simulation"
	Category    string
	MinPriority string
	MaxIssues   int
	TargetWords int
}

// ChapterCheck builds `novelgen tool check KIND --target chapter --scope chapter
// --id <id>` with the optional flags in their canonical order.
func ChapterCheck(chapterID string, opts ChapterCheckOptions) string {
	return Check(CheckOptions{
		Kind:        opts.Kind,
		Target:      "chapter",
		Scope:       "chapter",
		ID:          chapterID,
		Category:    opts.Category,
		MinPriority: opts.MinPriority,
		MaxIssues:   opts.MaxIssues,
		TargetWords: opts.TargetWords,
	})
}

// QueryOptions selects the flags of a query command. Empty fields are omitted.
type QueryOptions struct {
	Type       string
	ID         string
	ChapterID  string
	VolumeID   string
	EntityType string
	Name       string
	Fields     string
	View       string
	Content    bool
	Limit      int
}

// QueryStorySetup builds `novelgen tool query story-setup ...`.
func QueryStorySetup(opts QueryOptions) string { return query("story-setup", opts) }

// QueryOutline builds `novelgen tool query outline ...`.
func QueryOutline(opts QueryOptions) string { return query("outline", opts) }

// QueryCraft builds `novelgen tool query craft ...`.
func QueryCraft(opts QueryOptions) string { return query("craft", opts) }

// QueryChapter builds `novelgen tool query chapter ...`.
func QueryChapter(opts QueryOptions) string { return query("chapter", opts) }

// QueryContext builds `novelgen tool query context ...`.
func QueryContext(opts QueryOptions) string { return query("context", opts) }

// QueryLogs builds `novelgen tool query logs ...`.
func QueryLogs(opts QueryOptions) string { return query("logs", opts) }

// query builds a query command with the optional flags in their canonical
// order: --type, --id, --chapter-id, --volume-id, --entity-type, --name,
// --fields, --content, --view, --limit.
func query(section string, opts QueryOptions) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("query ")
	b.WriteString(section)
	writeFlag(&b, "--type", opts.Type, false)
	writeFlag(&b, "--id", opts.ID, true)
	writeFlag(&b, "--chapter-id", opts.ChapterID, true)
	writeFlag(&b, "--volume-id", opts.VolumeID, true)
	writeFlag(&b, "--entity-type", opts.EntityType, false)
	writeFlag(&b, "--name", opts.Name, true)
	writeFlag(&b, "--fields", opts.Fields, false)
	if opts.Content {
		b.WriteString(" --content")
	}
	writeFlag(&b, "--view", opts.View, false)
	writeInt(&b, "--limit", opts.Limit)
	return b.String()
}

// PatchOptions selects the flags of a patch command. Target only applies to the
// outline and craft sections; setup and recap take no target.
type PatchOptions struct {
	Section        string // outline, craft, setup, recap, chapter
	Target         string // outline chapter|volume, craft kind
	ID             string
	Buffer         string // long-prose patch buffer id
	Apply          bool
	RefreshDerived bool // chapter apply: rebuild the derived RPG DSL
	TargetWords    int  // chapter patches carry the repair length target
}

// Patch builds a patch command: `novelgen tool patch SECTION [--target T]
// [--id ID] [--patch-buffer B] [--apply] [--refresh-derived]
// [--target-words N]`. Without Apply the command is a dry run.
func Patch(opts PatchOptions) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("patch ")
	b.WriteString(opts.Section)
	writeFlag(&b, "--target", opts.Target, false)
	writeFlag(&b, "--id", opts.ID, true)
	writeFlag(&b, "--patch-buffer", opts.Buffer, true)
	if opts.Apply {
		b.WriteString(" --apply")
	}
	if opts.RefreshDerived {
		b.WriteString(" --refresh-derived")
	}
	writeInt(&b, "--target-words", opts.TargetWords)
	return b.String()
}

// PatchOutlineVolume builds `novelgen tool patch outline --target volume --id <id>`,
// the dry-run patch command for one volume.
func PatchOutlineVolume(volumeID string) string {
	return Patch(PatchOptions{Section: "outline", Target: "volume", ID: volumeID})
}

// PatchCraft builds `novelgen tool patch craft --target <kind> --id <id>`.
func PatchCraft(kind, id string) string {
	return Patch(PatchOptions{Section: "craft", Target: kind, ID: id})
}

// PatchSetup builds `novelgen tool patch setup`.
func PatchSetup() string {
	return Patch(PatchOptions{Section: "setup"})
}

// PatchRecap builds `novelgen tool patch recap --id <chapter id>`.
func PatchRecap(chapterID string) string {
	return Patch(PatchOptions{Section: "recap", ID: chapterID})
}

// PatchChapter builds `novelgen tool patch chapter --id <chapter id>`.
func PatchChapter(chapterID string) string {
	return Patch(PatchOptions{Section: "chapter", ID: chapterID})
}

// PatchBufferAppend builds the stdin-piped form used for long chapter prose:
// `novelgen tool patch-buffer append --id <id> --stdin`.
func PatchBufferAppend(bufferID string) string {
	return prefix + "patch-buffer append --id " + SingleQuote(bufferID) + " --stdin"
}

// ChapterDSLRefresh builds `novelgen tool refresh chapter-dsl --id <id>`.
func ChapterDSLRefresh(chapterID string) string {
	return prefix + "refresh chapter-dsl --id " + SingleQuote(chapterID)
}

// SingleQuote quotes a value as one POSIX shell argument, so an agent can paste
// the resulting command into a shell unchanged.
func SingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// StdInPatchPipe wraps a patch command in the stdin form agent workflows must
// use for JSON patches: `printf '%s' '<json>' | <command>`.
func StdInPatchPipe(patchCommand, compactJSON string) string {
	return "printf '%s' " + SingleQuote(compactJSON) + " | " + patchCommand
}

// PatchTask reuses a validated outline repair task instead of an inline patch:
// `<patch command> --task '<task id>'`.
func PatchTask(patchCommand, taskID string) string {
	return patchCommand + " --task " + SingleQuote(taskID)
}

func writeFlag(b *strings.Builder, name, value string, quoted bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString(" ")
	b.WriteString(name)
	b.WriteString(" ")
	if quoted {
		b.WriteString(SingleQuote(value))
		return
	}
	b.WriteString(value)
}

func writeInt(b *strings.Builder, name string, value int) {
	if value <= 0 {
		return
	}
	b.WriteString(" ")
	b.WriteString(name)
	b.WriteString(" ")
	b.WriteString(strconv.Itoa(value))
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
