package agentkit

import (
	"io/fs"
	"path"
	"strings"
)

// WorkflowInfo describes one shared stage workflow bundled with the adapters.
type WorkflowInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ReferenceInfo describes one on-demand reference document.
type ReferenceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ListWorkflows returns the shared workflow names.
func ListWorkflows() ([]WorkflowInfo, error) {
	return listTemplateMarkdown("templates/workflows")
}

// ReadWorkflow returns one shared workflow markdown body.
func ReadWorkflow(name string) (string, error) {
	return readTemplateMarkdown("templates/workflows", name)
}

// ListReferences returns the bundled reference documents.
func ListReferences() ([]ReferenceInfo, error) {
	items, err := listTemplateMarkdown("templates/references")
	if err != nil {
		return nil, err
	}
	out := make([]ReferenceInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ReferenceInfo{Name: item.Name, Description: item.Description})
	}
	return out, nil
}

// ReadReference returns one bundled reference markdown body.
func ReadReference(name string) (string, error) {
	return readTemplateMarkdown("templates/references", name)
}

func listTemplateMarkdown(dir string) ([]WorkflowInfo, error) {
	entries, err := fs.ReadDir(templates, dir)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := fs.ReadFile(templates, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, WorkflowInfo{
			Name:        strings.TrimSuffix(entry.Name(), ".md"),
			Description: firstMarkdownHeading(string(content)),
		})
	}
	return out, nil
}

func readTemplateMarkdown(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fs.ErrNotExist
	}
	data, err := fs.ReadFile(templates, path.Join(dir, name+".md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func firstMarkdownHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
