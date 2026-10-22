package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"novelgen/internal/platform/store"
)

// layoutOwner is the package allowed to spell project directories. Every other
// package asks its *store.Root (or internal/fiction/domain/project.Book) for a path.
const layoutOwner = "internal/platform/store"

// layoutDirs are the top-level project directories whose names belong to
// internal/platform/store. The user-global config directory ("$HOME/.novelgen") shares a
// name with the project-local agent work directory but is not project layout, so
// it is deliberately not in this list.
var layoutDirs = map[string]bool{
	"story":    true,
	"chapters": true,
	"drafts":   true,
	"logs":     true,
}

// layoutFileNames are the artifact file names that only internal/platform/store may
// spell. The directory rule above cannot see them, which is how
// `filepath.Join(root, "novel.json")` and a second copy of "characters.json"
// survived the first cleanup. "llm_config.json" is deliberately absent: the same
// name is also the user-global config file (~/.novelgen/llm_config.json), which
// store does not own.
func layoutFileNames() map[string]bool {
	root := store.MemRoot()
	base := func(path string) string { return filepath.Base(path) }
	names := map[string]bool{
		base(root.NovelJSON()):       true,
		base(root.SetupJSON()):       true,
		base(root.SetupMarkdown()):   true,
		base(root.OutlineJSON()):     true,
		base(root.OutlineMarkdown()): true,
		base(root.UsageLog()):        true,
		base(root.CloneManifest()):   true,
	}
	for _, kind := range []store.CraftKind{store.CraftCharacters, store.CraftLocations, store.CraftItems, store.CraftOrganizations} {
		names[kind.FileName()] = true
	}
	return names
}

// TestProjectLayoutIsSpelledOnlyInStore pins the storage ownership rule from
// AGENTS.md: internal/platform/store owns the on-disk layout, and no other package may
// re-derive "story/...", "chapters/...", "drafts/..." or "logs/..." itself.
//
// Without this guard the rule decayed silently: the agent tool plane had grown
// its own chapter-file candidate list, which had already drifted from the list
// the write pipeline and export use.
//
// webui is exempt from this rule because webui/boundary_test.go forbids it from
// importing the parent module at all, so it cannot use internal/platform/store; it
// maintains its own read-only copy of the layout by design, which
// TestWebUILayoutMatchesStore checks against the paths store produces.
func TestProjectLayoutIsSpelledOnlyInStore(t *testing.T) {
	root := repoRoot(t)
	phaseFiles := map[string]bool{}
	for _, name := range store.RPGPhaseFiles() {
		phaseFiles[name] = true
	}

	fset := token.NewFileSet()
	var violations []string

	walkGoFiles(t, root, func(path string, rel string) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			line := fset.Position(literal.Pos()).Line
			if reason := layoutLiteralReason(value, phaseFiles, isPathCallArgument(file, literal)); reason != "" {
				violations = append(violations, rel+":"+strconv.Itoa(line)+": "+reason+" ("+literal.Value+")")
			}
			return true
		})
	})

	if len(violations) > 0 {
		t.Errorf("project layout must be spelled only in internal/platform/store; use a *store.Root path method instead:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// layoutLiteralReason reports why a string literal is a layout violation. Raw
// string literals are skipped: documentation blocks (`Long`, `Example`) and
// struct tags legitimately mention project paths in prose and are quoted that
// way, while path construction uses interpreted literals.
func layoutLiteralReason(value string, phaseFiles map[string]bool, pathCallArgument bool) string {
	return layoutLiteralReasonWith(value, phaseFiles, pathCallArgument, layoutFileNames())
}

// layoutLiteralReasonWith is layoutLiteralReason with the owned file-name set
// injected, so the web UI parity check can reuse the rule.
func layoutLiteralReasonWith(value string, phaseFiles map[string]bool, pathCallArgument bool, fileNames map[string]bool) string {
	if phaseFiles[value] {
		return "phased RPG-DSL file name"
	}
	if fileNames[value] {
		return "project artifact file name"
	}
	first, _, hasSeparator := strings.Cut(strings.ReplaceAll(value, `\`, "/"), "/")
	if !layoutDirs[first] {
		return ""
	}
	if hasSeparator {
		return "project-relative path"
	}
	if pathCallArgument {
		return "project directory name passed to a path function"
	}
	return ""
}

// pathCallFuncs turn literals into filesystem paths. Only their arguments count
// as layout construction: the same words appear legitimately elsewhere, as a
// `--source chapters|drafts` flag default, a check name, a switch case or a
// `json:"chapters"` struct tag.
var pathCallFuncs = map[string]bool{
	"Join": true, "Glob": true, "Abs": true, "Rel": true, "WalkDir": true, "Clean": true,
	"Stat": true, "Lstat": true, "ReadFile": true, "WriteFile": true, "ReadDir": true,
	"MkdirAll": true, "Mkdir": true, "Remove": true, "RemoveAll": true,
	"Open": true, "OpenFile": true, "Create": true, "Chtimes": true, "Rename": true,
}

// isPathCallArgument reports whether the literal is an argument of a path
// function, which is how `filepath.Join(root, "chapters")` looks.
func isPathCallArgument(file *ast.File, literal *ast.BasicLit) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if !pathCallFuncs[calleeName(call.Fun)] {
			return true
		}
		for _, arg := range call.Args {
			if arg == literal {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// calleeName returns the last identifier of a call's function expression:
// "Join" for filepath.Join, "LogFile" for root.LogFile.
func calleeName(fun ast.Expr) string {
	switch value := fun.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

// walkGoFiles visits every non-test Go file under repo that belongs to the
// layout contract: the root module's internal packages, cmd, tools and examples,
// minus the layout owner and webui.
func walkGoFiles(t *testing.T, repo string, visit func(path, rel string)) {
	t.Helper()
	for _, dir := range []string{"internal", "cmd", "tools", "examples"} {
		base := filepath.Join(repo, dir)
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(repo, path)
			if relErr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, layoutOwner+"/") {
				return nil
			}
			visit(path, rel)
			return nil
		})
	}
}

// storePathMethods are the Root methods that hand out a project path. Any os
// call fed by one of them reaches around the FS seam that Root.Read / Root.Write
// exist to provide — which is how the storage layer ended up with a MemFS that
// nothing outside its own package could use.
var storePathMethods = map[string]bool{
	"NovelJSON": true, "LLMConfigJSON": true, "StoryDir": true, "SetupDir": true,
	"SetupJSON": true, "SetupMarkdown": true, "ComposeDir": true, "ComposeFile": true,
	"OutlineJSON": true, "OutlineMarkdown": true, "CraftDir": true, "CraftFile": true,
	"RecapsDir": true, "RecapFile": true, "ReviewsDir": true, "ReviewFile": true,
	"VolumeReviewFile": true, "RPGDir": true, "RPGFile": true, "RPGChaptersDSL": true,
	"RPGCacheDir": true, "RPGChapterBatchCacheDir": true, "AgentWorkDir": true,
	"AgentPatchDir": true, "ChaptersDir": true, "ChapterFile": true,
	"ChapterCandidates": true, "DraftsDir": true, "DraftFile": true, "LogsDir": true,
	"LogFile": true, "UsageLog": true, "CloneManifest": true,
}

// storeRootConstructors build a Root inline, so an os call around one of them is
// the same violation written the other way round: os.Stat(store.At(dir).CraftDir()).
var storeRootConstructors = map[string]bool{"At": true, "Open": true, "OpenFS": true}

// hostIOFuncs are the os package's filesystem entry points.
var hostIOFuncs = map[string]bool{
	"ReadFile": true, "WriteFile": true, "OpenFile": true, "Open": true, "Create": true,
	"ReadDir": true, "MkdirAll": true, "Mkdir": true, "Stat": true, "Lstat": true,
	"Remove": true, "RemoveAll": true, "Rename": true,
}

// projectIOAllowlist excuses an os.* call that legitimately operates on a
// store-derived path. It is empty on purpose: the code that works on host paths
// instead (the user-global config, agent home, a caller-chosen output path, the
// whole-tree copy in `project clone`) never mentions a store path method, so the
// rule below cannot see it and no entry is needed. An entry belongs here only
// when the detector is right about the call being store-related and the file has
// a real reason anyway — and the stale-entry check below removes it again as soon
// as that stops being true.
var projectIOAllowlist = map[string]string{}

// TestProjectIOGoesThroughStore pins the second half of the storage contract:
// project artifacts are read and written through *store.Root (Root.Read,
// Root.Write, Root.FS()), never with a direct os call on a path store handed
// out. Only statically visible cases are caught — an os call whose argument
// mentions a store path method or an inline Root — because a path stored in a
// variable first cannot be recognized here. That is enough: the pattern this
// guards against is `os.ReadFile(root.ChapterFile(id))`.
func TestProjectIOGoesThroughStore(t *testing.T) {
	repo := repoRoot(t)
	fset := token.NewFileSet()
	var violations []string
	allowlistHits := map[string]int{}

	walkGoFiles(t, repo, func(path, rel string) {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		var hits []string
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !isHostIOCall(selector) {
				return true
			}
			for _, arg := range call.Args {
				if source, isStorePath := storePathIn(arg); isStorePath {
					hits = append(hits, fmt.Sprintf(
						"%s:%d: os.%s(…%s…) bypasses the store; use Root.Read/Root.Write/Root.FS() instead",
						rel, fset.Position(call.Pos()).Line, selector.Sel.Name, source))
					break
				}
			}
			return true
		})
		if _, allowed := projectIOAllowlist[rel]; allowed {
			allowlistHits[rel] = len(hits)
			return
		}
		violations = append(violations, hits...)
	})

	if len(violations) > 0 {
		t.Errorf("project state must be read and written through internal/platform/store:\n  %s",
			strings.Join(violations, "\n  "))
	}
	// An entry that no longer excuses anything is stale documentation, so it has
	// to go with the code it described.
	for rel := range projectIOAllowlist {
		if allowlistHits[rel] == 0 {
			t.Errorf("projectIOAllowlist entry %q no longer excuses any os.* call on a store path; remove it", rel)
		}
	}
}

// isHostIOCall reports whether the selector is os.<file operation>.
func isHostIOCall(selector *ast.SelectorExpr) bool {
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "os" && hostIOFuncs[selector.Sel.Name]
}

// storePathIn reports where an expression gets a project path from: a store path
// method (root.ChapterFile(…), book.Root().OutlineJSON()) or an inline Root
// (store.At(dir), store.Open(dir)).
func storePathIn(expr ast.Expr) (string, bool) {
	found := ""
	ast.Inspect(expr, func(node ast.Node) bool {
		if found != "" {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if storePathMethods[selector.Sel.Name] {
			found = selector.Sel.Name
			return false
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "store" && storeRootConstructors[selector.Sel.Name] {
			found = "store." + selector.Sel.Name
			return false
		}
		return true
	})
	return found, found != ""
}

// layoutPrefixStarts are the first path elements a project-relative path may
// begin with. "books" is the developer-tool/web-UI convention for a workspace
// holding several projects (store.BooksDirName).
var layoutPrefixStarts = map[string]bool{
	"story": true, "chapters": true, "drafts": true, "logs": true, "books": true,
}

// logStreamNames is the set of log stream directories store declares, and
// logStreamFiles the single files that live directly in logs/.
func logStreams() (streams, files map[string]bool) {
	streams = map[string]bool{}
	for _, name := range store.LogStreamNames() {
		streams[name] = true
	}
	root := store.OpenFS(".", store.NewMemFS())
	files = map[string]bool{
		filepath.Base(root.UsageLog()):      true,
		filepath.Base(root.CloneManifest()): true,
	}
	return streams, files
}

// storeRelativePaths returns every path store produces, plus each of its
// element-wise prefixes, as slash-joined strings ("story/compose/outline.json",
// "story/compose", "story"). A caller that spells one of these runs is in step
// with the layout; one that spells something adjacent is not.
func storeRelativePaths() map[string]bool {
	// A root of "." makes every path project-relative, which is how the web UI
	// spells them (filepath.Join(projectPath, "story", ...)).
	root := store.OpenFS(".", store.NewMemFS())
	paths := []string{
		root.NovelJSON(), root.LLMConfigJSON(), root.SetupJSON(), root.SetupMarkdown(),
		root.OutlineJSON(), root.OutlineMarkdown(), root.ComposeFile("outline_review.json"),
		root.CraftFile(store.CraftCharacters), root.CraftFile(store.CraftLocations),
		root.CraftFile(store.CraftItems), root.CraftFile(store.CraftOrganizations),
		root.RecapFile("P1-V1-C1"), root.VolumeReviewFile("P1-V1"),
		root.ReviewFile("outline_review.json"), root.RPGCacheDir(),
		root.RPGChapterBatchCacheDir(), root.RPGChaptersDSL(),
		root.ChapterFile("P1-V1-C1"), root.DraftFile("P1-V1-C1"), root.UsageLog(),
		root.CloneManifest(), root.AgentWorkDir(), root.AgentPatchDir(),
		store.BookDir(".", "mine"),
	}
	paths = append(paths, store.RPGPhaseFiles()...)
	allowed := map[string]bool{}
	for _, path := range paths {
		slashed := filepath.ToSlash(path)
		parts := strings.Split(slashed, "/")
		for i := 1; i <= len(parts); i++ {
			allowed[strings.Join(parts[:i], "/")] = true
		}
	}
	return allowed
}

// TestWebUILayoutMatchesStore keeps the web UI's hand-rolled copy of the layout
// honest. webui cannot import internal/platform/store (webui/boundary_test.go forbids it),
// so it re-derives the read-only paths by hand — a copy no compiler checks. This
// test walks its path joins and requires every chain that starts with a layout
// directory to be a prefix of a path store produces.
//
// Log stream names under logs/ are deliberately open-ended: the CLI names one
// directory per log kind, and the web UI also lists directories from older
// versions, so only the first element is checked there.
func TestWebUILayoutMatchesStore(t *testing.T) {
	repo := repoRoot(t)
	allowed := storeRelativePaths()
	logStreamNames, logStreamFiles := logStreams()
	webuiDir := filepath.Join(repo, "webui")
	entries, err := os.ReadDir(webuiDir)
	if err != nil {
		t.Fatalf("read webui dir: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		rel := "webui/" + name
		file, err := parser.ParseFile(fset, filepath.Join(webuiDir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				value, err := strconv.Unquote(literal.Value)
				if err != nil || value == "" {
					return true
				}
				slashed := strings.ReplaceAll(value, `\`, "/")
				first, second, hasSep := strings.Cut(slashed, "/")
				if !hasSep || !layoutPrefixStarts[first] || strings.HasSuffix(slashed, "/") {
					return true
				}
				if first == "logs" {
					if !logStreamNames[second] && !logStreamFiles[second] {
						violations = append(violations, fmt.Sprintf(
							"%s:%d: %q is not a log stream internal/platform/store declares", rel, fset.Position(literal.Pos()).Line, value))
					}
					return true
				}
				if !allowed[slashed] {
					violations = append(violations, fmt.Sprintf(
						"%s:%d: %q is not a project path internal/platform/store produces", rel, fset.Position(literal.Pos()).Line, value))
				}
				return true
			}
			call, ok := node.(*ast.CallExpr)
			if !ok || calleeName(call.Fun) != "Join" {
				return true
			}
			chain := literalChain(call.Args)
			if len(chain) == 0 || !layoutPrefixStarts[chain[0]] {
				return true
			}
			if chain[0] == "logs" {
				// The stream name under logs/ is part of the contract even though
				// the files inside it are not: a rename in store must not leave the
				// web UI reading a directory the CLI stopped writing.
				if len(chain) > 1 && !logStreamNames[chain[1]] && !logStreamFiles[chain[1]] {
					violations = append(violations, fmt.Sprintf(
						"%s:%d: %q is not a log stream internal/platform/store declares", rel, fset.Position(call.Pos()).Line, chain[1]))
				}
				return true
			}
			joined := strings.Join(chain, "/")
			if !allowed[joined] {
				violations = append(violations, fmt.Sprintf(
					"%s:%d: %q is not a project path internal/platform/store produces", rel, fset.Position(call.Pos()).Line, joined))
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Errorf("the web UI's copy of the project layout drifted from internal/platform/store:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// literalChain returns the string literals in a path-join call, in order. It is
// how `filepath.Join(projectPath, "story", "compose", "outline.json")` names a
// layout path without any single literal containing a separator.
func literalChain(args []ast.Expr) []string {
	chain := make([]string, 0, len(args))
	for _, arg := range args {
		literal, ok := arg.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil || value == "" {
			continue
		}
		chain = append(chain, value)
	}
	return chain
}
