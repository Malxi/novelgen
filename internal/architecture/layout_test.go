package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"novelgen/internal/store"
)

// layoutOwner is the package allowed to spell project directories. Every other
// package asks its *store.Root (or internal/project.Book) for a path.
const layoutOwner = "internal/store"

// layoutDirs are the top-level project directories whose names belong to
// internal/store. The user-global config directory ("$HOME/.novelgen") shares a
// name with the project-local agent work directory but is not project layout, so
// it is deliberately not in this list.
var layoutDirs = map[string]bool{
	"story":    true,
	"chapters": true,
	"drafts":   true,
	"logs":     true,
}

// TestProjectLayoutIsSpelledOnlyInStore pins the storage ownership rule from
// AGENTS.md: internal/store owns the on-disk layout, and no other package may
// re-derive "story/...", "chapters/...", "drafts/..." or "logs/..." itself.
//
// Without this guard the rule decayed silently: the agent tool plane had grown
// its own chapter-file candidate list, which had already drifted from the list
// the write pipeline and export use.
//
// webui is exempt because webui/boundary_test.go forbids it from importing the
// parent module at all, so it cannot use internal/store; it maintains its own
// read-only copy of the layout by design.
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
		t.Errorf("project layout must be spelled only in internal/store; use a *store.Root path method instead:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// layoutLiteralReason reports why a string literal is a layout violation. Raw
// string literals are skipped: documentation blocks (`Long`, `Example`) and
// struct tags legitimately mention project paths in prose and are quoted that
// way, while path construction uses interpreted literals.
func layoutLiteralReason(value string, phaseFiles map[string]bool, pathCallArgument bool) string {
	if phaseFiles[value] {
		return "phased RPG-DSL file name"
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
