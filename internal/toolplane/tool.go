// Package toolplane implements the `novelgen tool ...` agent tool plane: read-only JSON queries,
// scoped checks, and patches that only land on disk behind an explicit --apply.
//
// This file holds the Cobra command surface, the section/type tables that generate the help text, and
// the dispatch into the per-domain implementations in the sibling files.

package toolplane

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"novelgen/internal/project"
	"strings"
	"time"
)

var ToolQueryFlags struct {
	Type           string
	ID             string
	Name           string
	EntityType     string
	ChapterID      string
	VolumeID       string
	View           string
	Fields         string
	IncludeContent bool
	Limit          int
}

var ToolProcessStartedAt = time.Now()

var ToolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Project tools for agents",
	Long: `Project tools for agents.

Query and check commands return JSON and never mutate project state. Patch
commands can mutate project state only when explicitly called with --apply.`,
}

// toolQuerySections lists the query sections in help order. It is the single
// source for the command's Use line, its long help and the "section is
// required" error, so a new section cannot be added to one and forgotten in the
// others.
var toolQuerySections = []string{"story-setup", "outline", "craft", "chapter", "context", "logs"}

// toolQuerySectionSummary describes what each section returns, for the long
// help.
var toolQuerySectionSummary = map[string]string{
	"story-setup": "Setup, storylines, premises, cast seeds, resources, timeline",
	"outline":     "Outline parts, volumes, chapters, refs, events",
	"craft":       "Crafted characters, items, locations, organizations",
	"chapter":     "Chapter outline records and manuscript text",
	"context":     "Compact workflow-specific context bundles",
	"logs":        "Prompt, response, and agent-live log indexes",
}

// toolQuerySectionTypes is the canonical --type value set per section. Aliases
// the normalizers also accept (for example "chapters" for "chapter") are
// deliberately not listed: an agent should see one spelling per query shape.
// The first entry of each list is what an omitted --type resolves to.
var toolQuerySectionTypes = map[string][]string{
	"story-setup": {"all", "index", "search", "storyline", "premise", "core-cast", "resource", "timeline", "rules", "long-form-plan"},
	"outline":     {"all", "part", "volume", "chapter", "refs", "events"},
	"craft":       {"all", "character", "item", "location", "organization"},
	"chapter":     {"all", "refs", "events"},
	"context": {
		"craft-character", "craft-item", "craft-location", "craft-organization",
		"outline-volume", "outline-repair", "outline-global-repair",
		"recap-repair", "chapter-write", "chapter-repair",
	},
	"logs": {"prompts", "responses", "agent-live"},
}

// toolQueryLongHelp renders the section list and the per-section --type values
// from the tables above, so the help text cannot drift from what the command
// accepts.
func toolQueryLongHelp() string {
	var b strings.Builder
	b.WriteString("Query novel project data as JSON.\n\nSections and the --type values they accept (an omitted --type uses the section default; logs defaults to all kinds):\n")
	for _, section := range toolQuerySections {
		fmt.Fprintf(&b, "  %-12s %s -- %s\n",
			section,
			strings.Join(toolQuerySectionTypes[section], "|"),
			toolQuerySectionSummary[section])
	}
	b.WriteString("\n--view index is route-only (ids/titles/counts); brief is the default; full is for one precise object.")
	b.WriteString("\n--fields projects fields, --limit applies to list results, and exact --id/--name narrows the target.")
	return b.String()
}

// toolQuerySectionAlias maps the section spellings the command accepts to the
// canonical section name used by the tables and the loaders.
func toolQuerySectionAlias(section string) string {
	switch section {
	case "setup":
		return "story-setup"
	case "chapters":
		return "chapter"
	case "log":
		return "logs"
	default:
		return section
	}
}

// canonicalToolQueryType resolves a raw --type against the section's aliases.
func canonicalToolQueryType(section, queryType string) string {
	switch section {
	case "story-setup":
		return normalizeSetupType(queryType)
	case "outline":
		return normalizeOutlineType(queryType)
	case "craft":
		return normalizeCraftType(queryType)
	case "chapter":
		return normalizeChapterType(queryType)
	case "context":
		return normalizeContextType(queryType)
	case "logs":
		return NormalizeLogKind(queryType)
	default:
		return NormalizeKey(queryType)
	}
}

// unknownToolQueryTypeError names the values the section does accept. It is the
// counterpart of the tables in the help text: a wrong --type tells the agent how
// to correct itself instead of just failing.
func unknownToolQueryTypeError(section, queryType string) error {
	return fmt.Errorf("unsupported --type %q for section %q; valid types: %s",
		queryType, section, strings.Join(toolQuerySectionTypes[section], ", "))
}

// toolQueryTypeKnown reports whether a canonical type is one the section's help
// table lists.
func toolQueryTypeKnown(section, canonicalType string) bool {
	for _, known := range toolQuerySectionTypes[section] {
		if known == canonicalType {
			return true
		}
	}
	return false
}

var ToolQueryCmd = &cobra.Command{
	Use:   "query [" + strings.Join(toolQuerySections, "|") + "]",
	Short: "Query novel project data as JSON",
	Long:  toolQueryLongHelp(),
	Example: `  novelgen tool query story-setup
  novelgen tool query story-setup --type storyline --name "Main arc"
  novelgen tool query outline --type chapter --id chap_001
  novelgen tool query outline --type refs --entity-type character --name Lin
  novelgen tool query outline --type events --entity-type item --name "Star Core"
  novelgen tool query craft --type character --name Lin
  novelgen tool query context --type craft-character --name Lin
  novelgen tool query logs --view index
  novelgen tool query chapter --id chap_001 --content
  novelgen tool query chapter --entity-type location --name "Mine"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runToolQuery,
}

// toolPatchSections lists the patch sections in help order, used by the Use line
// and by the unsupported-section error.
var toolPatchSections = []string{"outline", "craft", "setup", "recap", "chapter"}

var ToolCheckCmd = &cobra.Command{
	Use:   "check [quality|simulation|all|schema]",
	Short: "Run scoped checks as JSON for agents",
	Long: `Run scoped checks as JSON for agents.

quality checks deterministic structure/contract rules.
simulation checks the same target through the in-memory RPG simulation backend.
all runs both and merges issues.
setup target checks story/setup/story_setup.json.
outline target supports scoped all/volume/chapter checks.
chapter target checks saved final chapter markdown and can simulate existing RPG DSL without invoking LLM.
recap target checks story/recaps/<chapter_id>.json continuity anchors.
schema validates typed craft objects and saved recap JSON already in project state.
craft checks infer the craft kind from --id, or check all four kinds when no scope/id is given; recap defaults to chapter scope.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runToolCheck,
}

var ToolPatchCmd = &cobra.Command{
	Use:   "patch [" + strings.Join(toolPatchSections, "|") + "]",
	Short: "Apply a validated JSON patch to project state",
	Long: `Apply a validated JSON patch to project state.

The tool reads the current project from the workspace. The agent submits only
the target and patch JSON on stdin. Project files are written only with --apply;
without --apply the command performs a dry run and returns the diff/check JSON.

The section is the positional argument; --target selects what inside that
section is patched:
  outline   --target chapter (default) or volume, --id <chapter|volume id>
  craft     --target character|item|location|organization (required), --id <element key>
  setup     no --id, no --target; the patch merges into the typed story setup
  recap     no --target; --id <chapter id>
  chapter   no --target; --id <chapter id> with {"content":"..."}; runs the
            deterministic chapter check, and with --refresh-derived also
            rebuilds the chapter RPG DSL

Patch JSON comes from stdin, from --patch-json (short ASCII-only patches) or from
a --patch-buffer filled by ` + "`tool patch-buffer`" + `. --task reuses the patch
shape of a validated outline repair task instead. Use only one patch source.
Typed patches reject unknown fields; an empty patch object is rejected, and a
patch that changes nothing returns "warnings" with an empty "changed" list.`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolPatch,
}

var ToolPatchBufferCmd = &cobra.Command{
	Use:   "patch-buffer [append|show|clear]",
	Short: "Manage a novelgen-owned temporary patch buffer",
	Long: `Manage a novelgen-owned temporary patch buffer.

This command exists for agent workflows that need to pass long chapter text to
tool patch without shell temp files or very long --patch-json arguments. It
stores text under .novelgen/agent-patches and does not mutate story state.`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolPatchBuffer,
}

var ToolRefreshCmd = &cobra.Command{
	Use:   "refresh [chapter-dsl]",
	Short: "Refresh derived project artifacts for agent workflows",
	Long: `Refresh derived project artifacts for agent workflows.

Refresh commands are explicit mutating tools. They do not edit story source
state such as setup, outline, craft, recaps, or chapter markdown; they rebuild
derived files that downstream checks consume.`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolRefresh,
}

var allowedToolPatchBufferOps = map[string]bool{
	"append": true,
	"show":   true,
	"clear":  true,
}

func runToolQuery(cmd *cobra.Command, args []string) error {
	section := ""
	if len(args) > 0 {
		section = toolQuerySectionAlias(NormalizeKey(args[0]))
	}
	if section == "" {
		return fmt.Errorf("query section is required: %s", strings.Join(toolQuerySections, ", "))
	}
	if _, known := toolQuerySectionTypes[section]; !known {
		return fmt.Errorf("unsupported query section %q; valid sections: %s",
			args[0], strings.Join(toolQuerySections, ", "))
	}
	if err := ValidateToolView(ToolQueryFlags.View); err != nil {
		return err
	}
	queryType := NormalizeKey(ToolQueryFlags.Type)
	canonicalType := canonicalToolQueryType(section, queryType)
	if canonicalType != "" && !toolQueryTypeKnown(section, canonicalType) {
		return unknownToolQueryTypeError(section, ToolQueryFlags.Type)
	}
	queryID, idRepaired := RepairToolLookupText(ToolQueryFlags.ID)
	queryName, nameRepaired := RepairToolLookupText(ToolQueryFlags.Name)
	queryChapterID, chapterIDRepaired := RepairToolLookupText(ToolQueryFlags.ChapterID)
	queryVolumeID, volumeIDRepaired := RepairToolLookupText(ToolQueryFlags.VolumeID)
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root

	ctx := ToolProjectContext{Root: root}
	if err := ctx.load(section); err != nil {
		return err
	}

	var resp ToolResponse
	switch section {
	case "story-setup":
		resp = QueryStorySetup(ctx, queryType, queryName)
	case "outline":
		resp = queryOutline(ctx, queryType, queryID, ToolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID)
	case "craft":
		resp = QueryCraft(ctx, queryType, queryName)
	case "chapter":
		resp = queryChapterSection(ctx, queryType, queryID, ToolQueryFlags.EntityType, queryName, queryChapterID, queryVolumeID, ToolQueryFlags.IncludeContent)
	case "context":
		resp = QueryContext(ctx, queryType, queryID, queryName)
	case "logs":
		resp = queryLogsAt(root, queryType, queryID, queryName, ToolQueryFlags.IncludeContent)
	}
	if idRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --id from %q to %q", ToolQueryFlags.ID, queryID))
	}
	if nameRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --name from %q to %q", ToolQueryFlags.Name, queryName))
	}
	if chapterIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --chapter-id from %q to %q", ToolQueryFlags.ChapterID, queryChapterID))
	}
	if volumeIDRepaired {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("repaired possible mojibake in --volume-id from %q to %q", ToolQueryFlags.VolumeID, queryVolumeID))
	}
	appendToolQueryFlagWarnings(&resp, section, canonicalType, queryID, queryName, queryChapterID, queryVolumeID)
	if ToolQueryFlags.IncludeContent && normalizeToolView(ToolQueryFlags.View) == "index" {
		resp.Warnings = append(resp.Warnings, "--content is omitted in index view; use --view brief to include chapter/log content")
	}

	resp.Section = section
	if canonicalType == "" {
		switch section {
		case "story-setup", "outline", "craft", "chapter", "logs":
			canonicalType = "all"
		default:
			canonicalType = queryType
		}
	}
	resp.Query = map[string]string{
		"section":     section,
		"type":        canonicalType,
		"id":          queryID,
		"name":        queryName,
		"entity_type": ToolQueryFlags.EntityType,
		"chapter_id":  queryChapterID,
		"volume_id":   queryVolumeID,
		"fields":      ToolQueryFlags.Fields,
	}
	if resp.Meta == nil {
		resp.Meta = map[string]interface{}{}
	}
	resp.Meta["project_root"] = root.Dir()
	ApplyToolView(&resp, ToolQueryFlags.View)
	ApplyToolFields(&resp, ToolQueryFlags.Fields)
	applyToolLimit(&resp, ToolQueryFlags.Limit)
	return WriteToolJSON(cmd, resp)
}

func WriteToolJSON(cmd *cobra.Command, resp ToolResponse) error {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

func init() {
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Type, "type", "",
		"Query type within the selected section; the section table lists the values it accepts")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.ID, "id", "",
		"Target ID: outline part/volume/chapter, chapter ID, or context target ID")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Name, "name", "", "Name or substring to search")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.EntityType, "entity-type", "",
		"Entity type for --type refs/events queries: character, item, location, storyline")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.ChapterID, "chapter-id", "", "Limit event queries to a chapter ID")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.VolumeID, "volume-id", "", "Limit event queries to a volume ID")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.View, "view", "brief",
		"Output detail level: index (route ids/counts), brief (default), or full (one precise object)")
	ToolQueryCmd.Flags().StringVar(&ToolQueryFlags.Fields, "fields", "", "Comma-separated JSON fields to return from each result, e.g. storyline_advances,chapter_payoff,conflict")
	ToolQueryCmd.Flags().BoolVar(&ToolQueryFlags.IncludeContent, "content", false, "Include chapter/log text content when available (brief view keeps it)")
	ToolQueryCmd.Flags().IntVar(&ToolQueryFlags.Limit, "limit", 0, "Maximum list results; object-shaped results return a warning instead")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Target, "target", "outline",
		"Check target: outline, setup, craft, chapter, or recap; craft requires kind schema")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Scope, "scope", "all",
		"Scope within the target: outline all|volume|chapter, craft character|item|location|organization (inferred from --id when omitted), chapter chapter, recap chapter (default)")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.ID, "id", "", "Volume/chapter ID, craft element name, final chapter ID, or recap chapter ID for scoped checks")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.MinPriority, "min-priority", "", "Only return issues at or above priority: low, medium, high, critical")
	ToolCheckCmd.Flags().StringVar(&ToolCheckFlags.Category, "category", "", "Only return issues in comma-separated categories, e.g. logic,plot,structure")
	ToolCheckCmd.Flags().IntVar(&ToolCheckFlags.MaxIssues, "max-issues", 0, "Maximum number of issues to return without changing the summary")
	ToolCheckCmd.Flags().IntVar(&ToolCheckFlags.TargetWords, "target-words", 0, "Override target narrative units for final chapter length checks")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.Target, "target", "",
		"Section-specific patch target: outline chapter|volume (default chapter), craft character|item|location|organization (required); setup/recap/chapter ignore it")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.ID, "id", "", "Target outline ID, final chapter ID, recap chapter ID, or craft element name/key")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.PatchJSON, "patch-json", "", "Patch JSON string; stdin is used when omitted")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.PatchBuffer, "patch-buffer", "", "Read patch content from a novelgen-managed agent patch buffer")
	ToolPatchCmd.Flags().StringVar(&ToolPatchFlags.Task, "task", "",
		"Reuse a validated outline repair task instead of stdin or --patch-json; the only supported form is outline-global-repair:<issue category>")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.Apply, "apply", false, "Apply the patch and save project files")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.DryRun, "dry-run", false, "Force dry-run mode even when --apply is present")
	ToolPatchCmd.Flags().BoolVar(&ToolPatchFlags.RefreshDerived, "refresh-derived", false, "For chapter --apply, regenerate derived RPG DSL and return a post-refresh all check")
	ToolPatchCmd.Flags().IntVar(&ToolPatchFlags.TargetWords, "target-words", 0, "Override target narrative units for chapter patch length checks")
	ToolPatchBufferCmd.Flags().StringVar(&ToolPatchBufferFlags.ID, "id", "", "Patch buffer id")
	ToolPatchBufferCmd.Flags().StringVar(&ToolPatchBufferFlags.Text, "text", "", "Text chunk to append; stdin is used when omitted")
	ToolPatchBufferCmd.Flags().BoolVar(&ToolPatchBufferFlags.Stdin, "stdin", false, "Read text chunk from stdin explicitly")
	ToolPatchBufferCmd.Flags().IntVar(&ToolPatchBufferFlags.MaxChars, "max-chars", 400, "Preview character limit for show")
	ToolRefreshCmd.Flags().StringVar(&ToolRefreshFlags.ID, "id", "", "Target chapter ID for scoped refresh")
	ToolRefreshCmd.Flags().IntVar(&ToolRefreshFlags.BatchSize, "batch-size", 10, "Chapter markdown batch size for AI -> RPG DSL conversion")
	ToolCmd.AddCommand(ToolQueryCmd)
	ToolCmd.AddCommand(ToolCheckCmd)
	ToolCmd.AddCommand(ToolPatchCmd)
	ToolCmd.AddCommand(ToolPatchBufferCmd)
	ToolCmd.AddCommand(ToolRefreshCmd)
}

// NewCommand returns the novelgen tool command tree.
func NewCommand() *cobra.Command {
	return ToolCmd
}
