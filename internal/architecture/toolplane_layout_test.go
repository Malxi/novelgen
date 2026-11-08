package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// toolPlaneSurfaceFile is the command surface of the agent tool plane: the
// Cobra commands, the section/--type tables that generate the help text, and the
// dispatch into the per-domain files.
const toolPlaneSurfaceFile = "internal/toolplane/tool.go"

// toolPlaneSurfaceFuncs is every top-level function the surface file may
// declare. Anything else belongs in a domain file.
var toolPlaneSurfaceFuncs = map[string]bool{
	"runToolQuery":              true,
	"toolQueryLongHelp":         true,
	"toolQuerySectionAlias":     true,
	"canonicalToolQueryType":    true,
	"unknownToolQueryTypeError": true,
	"toolQueryTypeKnown":        true,
	"WriteToolJSON":             true,
	"NewCommand":                true,
	"init":                      true,
}

// toolPlaneCommands are the command values the surface file owns.
var toolPlaneCommands = []string{
	"ToolCmd", "ToolQueryCmd", "ToolCheckCmd", "ToolPatchCmd", "ToolPatchBufferCmd", "ToolRefreshCmd",
}

// toolPlaneConcernFiles pins where each concern lives, together with the
// declarations that prove the concern has not moved. The query sections and the
// workflow context bundles get one file each; check, patch and brief follow the
// same rule per target (setup, outline, craft, chapter, recap), with the shared
// parts in their own file.
var toolPlaneConcernFiles = map[string][]string{
	// Shared models and helpers.
	"internal/toolplane/tool_types.go":     {"ToolResponse", "OutlinePath", "StorySetupBrief", "ChapterBrief"},
	"internal/toolplane/tool_match.go":     {"flattenChapters", "collectVolumeEntities", "chapterEntityReasons", "textMatches"},
	"internal/toolplane/tool_normalize.go": {"NormalizeKey", "normalizeContextType", "normalizeSetupType", "normalizeEntityType"},
	"internal/toolplane/tool_flags.go":     {"ToolCheckFlags", "ToolPatchFlags", "ToolPatchBufferFlags", "ToolRefreshFlags"},

	// Query sections.
	"internal/toolplane/tool_query_setup.go":   {"QueryStorySetup", "queryStorySetupSearch"},
	"internal/toolplane/tool_query_outline.go": {"QueryOutlineNode", "QueryOutlineRefs", "QueryEvents", "queryChapter"},
	"internal/toolplane/tool_query_craft.go":   {"QueryCraft", "queryNamedElement"},
	"internal/toolplane/tool_query_ideas.go":   {"QueryIdeas"},

	// Workflow context bundles.
	"internal/toolplane/tool_context.go":         {"QueryContext"},
	"internal/toolplane/tool_context_chapter.go": {"queryChapterWriteContext", "queryChapterRepairContext", "BuildChapterSimulationRepairHints"},
	"internal/toolplane/tool_context_outline.go": {"QueryOutlineGlobalRepairContext", "buildOutlineRepairIssues"},
	"internal/toolplane/tool_context_volume.go":  {"queryOutlineVolumeContext", "buildOutlineVolumeNextActions"},
	"internal/toolplane/tool_context_craft.go":   {"QueryCraftCharacterContext", "queryCraftElementContext"},
	"internal/toolplane/tool_context_recap.go":   {"queryRecapRepairContext", "buildRecapRepairNextActions"},
	"internal/toolplane/tool_mystery.go":         {"CollectOutlineMysteryThreads", "FirstPatchableOutlineMysteryThread"},

	// Patches, one file per section.
	"internal/toolplane/tool_patch_buffer.go": {"RunToolPatchBuffer", "LoadToolPatchBuffer"},

	// Brief projections, one file per section.
	"internal/toolplane/tool_brief.go":         {"briefToolResults", "indexToolResults", "clipToolStrings"},
	"internal/toolplane/tool_brief_setup.go":   {"briefStorySetupResults", "makeLongFormPlanBrief"},
	"internal/toolplane/tool_brief_outline.go": {"briefOutlineResults", "makeVolumeBrief"},
	"internal/toolplane/tool_brief_craft.go":   {"briefCraftResults", "briefCraftHit"},
	"internal/toolplane/tool_brief_context.go": {"briefContextResults", "indexContextResults", "toolCheckIndex"},
	"internal/toolplane/tool_brief_chapter.go": {"briefChapterResults", "makeChapterBrief", "loadChapterExcerptForTool"},
	"internal/toolplane/tool_brief_recap.go":   {"makeRecapBrief", "recapUsableForChapter"},
	"internal/toolplane/tool_brief_ideas.go":   {"briefIdeaResults"},
}

// toolPlaneModuleFiles are the remaining tool plane modules. They are listed so a
// new file in the package has to be classified here on purpose.
var toolPlaneModuleFiles = map[string]string{
	"internal/toolplane/mutation_aliases.go":    "compatibility aliases over the fiction/mutation service",
	"internal/toolplane/tool_patch_cli.go":      "CLI adapter over the fiction/mutation service",
	"internal/toolplane/tool_replace_cli.go":    "complete-artifact replace adapter over the fiction/mutation service",
	"internal/toolplane/tool_patch_adapters.go": "per-section adapters over the fiction/mutation service",
	"internal/toolplane/check_aliases.go":       "compatibility aliases over the fiction/check engine",
	"internal/toolplane/tool_check_cli.go":      "CLI adapter over the fiction/check engine",
	"internal/toolplane/tool_view.go":           "view levels and context/check budgets",
	"internal/toolplane/tool_logs.go":           "prompt/response/agent-live log queries",
	"internal/toolplane/tool_refresh_cli.go":    "CLI adapter over the fiction/refresh engine",
	"internal/toolplane/tool_memory.go":         "narrative-memory query and refresh adapters",
	"internal/toolplane/tool_skill.go":          "agent skill catalog and on-demand skill loading",
}

// TestToolPlaneStaysSplitByConcern pins the internal/toolplane file layout.
//
// The package used to be a single 3900-line tool.go that mixed the command
// surface, the agent-facing DTOs and five query/context domains, so no reviewer
// could see a whole concern at once. Splitting it only helps while it stays
// split: a query section drifting back into the surface file, or a new domain
// file nobody classified, fails here instead of silently regrowing the monolith.
func TestToolPlaneStaysSplitByConcern(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "toolplane")

	classified := map[string]bool{toolPlaneSurfaceFile: true}
	for file := range toolPlaneConcernFiles {
		classified[file] = true
	}
	for file := range toolPlaneModuleFiles {
		classified[file] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if rel := "internal/toolplane/" + name; !classified[rel] {
			t.Errorf("internal/toolplane/%s is not classified by the tool plane layout test; add it to the concern or module table instead of leaving an unowned file", name)
		}
	}

	for file, want := range toolPlaneConcernFiles {
		funcs, types, vars := toolPlaneTopDecls(t, filepath.Join(root, filepath.FromSlash(file)))
		for _, name := range want {
			if !funcs[name] && !types[name] && !vars[name] {
				t.Errorf("%s no longer declares %s; keep each query section, context bundle and shared concern in its own file", file, name)
			}
		}
	}
	for file, purpose := range toolPlaneModuleFiles {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); err != nil {
			t.Errorf("tool plane module %s (%s) is missing: %v", file, purpose, err)
		}
	}

	surfacePath := filepath.Join(root, filepath.FromSlash(toolPlaneSurfaceFile))
	funcs, types, vars := toolPlaneTopDecls(t, surfacePath)
	for _, name := range sortedToolPlaneNames(funcs) {
		if !toolPlaneSurfaceFuncs[name] {
			t.Errorf("%s declares %s; the surface file holds the command definitions, the help tables and the dispatch only", toolPlaneSurfaceFile, name)
		}
	}
	if len(types) > 0 {
		t.Errorf("%s declares type %s; the agent-facing DTOs live in tool_types.go", toolPlaneSurfaceFile, strings.Join(sortedToolPlaneNames(types), ", "))
	}
	for _, name := range toolPlaneCommands {
		if !vars[name] {
			t.Errorf("%s must declare the %s command", toolPlaneSurfaceFile, name)
		}
	}
}

// toolPlaneTopDecls splits a file's top-level declarations into package-level
// functions, types and vars. Methods stay with their receiver and are ignored.
func toolPlaneTopDecls(t *testing.T, path string) (funcs, types, vars map[string]bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	funcs, types, vars = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, decl := range file.Decls {
		switch value := decl.(type) {
		case *ast.FuncDecl:
			if value.Recv == nil {
				funcs[value.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range value.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					types[spec.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						vars[name.Name] = true
					}
				}
			}
		}
	}
	return funcs, types, vars
}

func sortedToolPlaneNames(names map[string]bool) []string {
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
