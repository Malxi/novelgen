package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAICallLogSchemaMatchesWebUI pins the one artifact whose schema crosses the
// webui module boundary: internal/fiction/agents writes logs/ai_calls/<id>.json, and the
// web UI reads it with its own struct (it cannot import this module, see
// webui/boundary_test.go). Neither compiler sees the other side, so the JSON tags
// are compared here — a rename on either side then fails the build instead of
// silently emptying the AI-call page.
func TestAICallLogSchemaMatchesWebUI(t *testing.T) {
	repo := repoRoot(t)
	writer := jsonFieldTypes(t, filepath.Join(repo, "internal", "fiction", "agents", "base", "agent_logging.go"), "aiCallRecord")
	reader := jsonFieldTypes(t, filepath.Join(repo, "webui", "main.go"), "aiCallLogFile")

	for _, path := range missingPaths(reader, writer) {
		t.Errorf("the web UI reader expects %q but internal/fiction/agents/base.aiCallRecord never sets it; keep the two structs in step", path)
	}
	for _, path := range missingPaths(writer, reader) {
		t.Errorf("internal/fiction/agents/base.aiCallRecord records %q but webui.aiCallLogFile does not decode it; keep the two structs in step", path)
	}
	// A matching tag is not enough: a field the reader decodes as time.Time but
	// the writer emits as a string fails at runtime, where nothing sees it.
	for path, readerType := range reader {
		writerType, ok := writer[path]
		if !ok || writerType == readerType {
			continue
		}
		t.Errorf("%s: internal/fiction/agents/base writes %s but webui decodes it as %s", path, writerType, readerType)
	}
	if len(writer) == 0 {
		t.Fatalf("no JSON tags found for aiCallRecord; the schema test is not checking anything")
	}
}

// jsonFieldTypes maps every JSON-tagged leaf of a struct in a Go file to its type,
// flattened with dotted paths for nested or named struct fields
// ("usage.prompt_tokens" → "int").
func jsonFieldTypes(t *testing.T, path, structName string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	structs := map[string]*ast.StructType{}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if structType, ok := spec.Type.(*ast.StructType); ok {
			structs[spec.Name.Name] = structType
		}
		return true
	})
	target, ok := structs[structName]
	if !ok {
		t.Fatalf("struct %s not found in %s", structName, path)
	}

	fields := map[string]string{}
	var collect func(structType *ast.StructType, prefix string)
	collect = func(structType *ast.StructType, prefix string) {
		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}
			name := jsonTagName(field)
			if name == "" {
				continue
			}
			fieldPath := prefix + name
			switch fieldType := field.Type.(type) {
			case *ast.StructType:
				collect(fieldType, fieldPath+".")
			case *ast.Ident:
				if nested, ok := structs[fieldType.Name]; ok {
					collect(nested, fieldPath+".")
					continue
				}
				fields[fieldPath] = types.ExprString(field.Type)
			default:
				fields[fieldPath] = types.ExprString(field.Type)
			}
		}
	}
	collect(target, "")
	return fields
}

// jsonTagName returns the name part of a field's json tag, or "" when the field
// has no json tag (such fields are not part of the wire format).
func jsonTagName(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	value := reflectTagValue(tag, "json")
	if value == "" || value == "-" {
		return ""
	}
	return strings.Split(value, ",")[0]
}

// reflectTagValue extracts one key from a raw struct tag such as
// `json:"usage" md:"usage"`.
func reflectTagValue(tag, key string) string {
	needle := key + `:"`
	index := strings.Index(tag, needle)
	if index < 0 {
		return ""
	}
	rest := tag[index+len(needle):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// missingPaths reports the keys of want that left does not have.
func missingPaths(left, want map[string]string) []string {
	var missing []string
	for path := range want {
		if _, ok := left[path]; !ok {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	return missing
}
