// Package agentkit installs portable Novelgen adapters for agent harnesses.
//
// The harness-specific parts (Claude plugin manifest/hooks, Codex AGENTS.md and
// MCP config) are thin. The project contract and write path remain in the
// novelgen CLI; the shared workflow notes live under .novelgen/workflows.
package agentkit

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Version is the adapter/template version written into generated manifests.
const Version = "0.1.0"

//go:embed all:templates
var templates embed.FS

// Options controls installation.
type Options struct {
	Apply        bool
	Force        bool
	AllowHooks   bool
	NovelgenPath string
	CodexHome    string
}

// FileChange is one file the installer touched or skipped.
type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // create, update, unchanged, skip
}

// Result is the installer report.
type Result struct {
	OK       bool         `json:"ok"`
	Root     string       `json:"root"`
	Targets  []string     `json:"targets"`
	Files    []FileChange `json:"files"`
	Warnings []string     `json:"warnings,omitempty"`
}

// Install writes the selected adapters under root.
func Install(root string, targets []string, opts Options) (Result, error) {
	result := Result{Root: root}
	if strings.TrimSpace(root) == "" {
		return result, fmt.Errorf("project root is required")
	}
	if strings.TrimSpace(opts.NovelgenPath) == "" {
		if exe, err := os.Executable(); err == nil {
			opts.NovelgenPath = exe
		} else {
			opts.NovelgenPath = "novelgen"
		}
	}
	normalized, err := normalizeTargets(targets)
	if err != nil {
		return result, err
	}
	result.Targets = normalized

	workflows, err := fs.Sub(templates, "templates/workflows")
	if err != nil {
		return result, err
	}
	changes, err := copyTree(workflows, root, ".novelgen/workflows", opts, nil)
	if err != nil {
		return result, err
	}
	result.Files = append(result.Files, changes...)

	for _, target := range normalized {
		switch target {
		case "claude":
			changes, err = installClaude(root, opts, &result)
		case "codex":
			changes, err = installCodex(root, opts, &result)
		case "dsh":
			changes, err = installDSH(root, opts, &result)
		}
		if err != nil {
			return result, err
		}
		result.Files = append(result.Files, changes...)
	}
	result.OK = true
	return result, nil
}

func normalizeTargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		targets = []string{"claude", "codex"}
	}
	seen := map[string]bool{}
	var out []string
	for _, target := range targets {
		for _, piece := range strings.Split(target, ",") {
			piece = strings.ToLower(strings.TrimSpace(piece))
			if piece == "" {
				continue
			}
			if piece == "all" {
				for _, name := range []string{"claude", "codex"} {
					if !seen[name] {
						seen[name] = true
						out = append(out, name)
					}
				}
				continue
			}
			switch piece {
			case "claude", "codex", "dsh":
			default:
				return nil, fmt.Errorf("unsupported target %q (use claude, codex, dsh, or all)", piece)
			}
			if !seen[piece] {
				seen[piece] = true
				out = append(out, piece)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one install target is required")
	}
	return out, nil
}

func installClaude(root string, opts Options, result *Result) ([]FileChange, error) {
	sub, err := fs.Sub(templates, "templates/claude")
	if err != nil {
		return nil, err
	}
	skip := func(rel string) bool {
		return !opts.AllowHooks && strings.Contains(path.Clean(rel), "plugin/hooks/")
	}
	changes, err := copyTree(sub, root, ".novelgen/adapters/claude", opts, skip)
	if err != nil {
		return nil, err
	}
	if !opts.AllowHooks {
		result.Warnings = append(result.Warnings,
			"Claude hooks were skipped; re-run with --allow-hooks to install the PreToolUse write guard and SessionStart hook")
	}
	return changes, nil
}

func installCodex(root string, opts Options, result *Result) ([]FileChange, error) {
	sub, err := fs.Sub(templates, "templates/codex")
	if err != nil {
		return nil, err
	}
	changes, err := copyTree(sub, root, ".novelgen/adapters/codex", opts, nil)
	if err != nil {
		return nil, err
	}

	// Codex reads AGENTS.md at the project root and .codex/config.toml for MCP.
	block, err := fs.ReadFile(templates, "templates/codex/AGENTS.block.md")
	if err != nil {
		return nil, err
	}
	agentsPath := filepath.Join(root, "AGENTS.md")
	agentsChange, err := upsertManagedBlock(agentsPath, "novelgen", string(block), opts)
	if err != nil {
		return nil, err
	}
	changes = append(changes, agentsChange)

	config, err := fs.ReadFile(templates, "templates/codex/.codex/config.toml")
	if err != nil {
		return nil, err
	}
	codexConfig, err := writeFile(root, ".codex/config.toml", config, opts)
	if err != nil {
		return nil, err
	}
	changes = append(changes, codexConfig)

	skill, err := fs.ReadFile(templates, "templates/codex/.codex/skills/novelgen/SKILL.md")
	if err != nil {
		return nil, err
	}
	codexSkill, err := writeFile(root, ".codex/skills/novelgen/SKILL.md", skill, opts)
	if err != nil {
		return nil, err
	}
	changes = append(changes, codexSkill)

	if strings.TrimSpace(opts.CodexHome) != "" {
		change, err := installCodexHome(opts)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	} else {
		result.Warnings = append(result.Warnings,
			"Codex CLI reads ~/.codex/config.toml, not a project .codex/config.toml; run `codex mcp add novelgen -- <novelgen> mcp serve` or pass --codex-home to install it")
	}
	return changes, nil
}

func installCodexHome(opts Options) (FileChange, error) {
	home := strings.TrimSpace(opts.CodexHome)
	if home == "" {
		return FileChange{}, fmt.Errorf("codex home is empty")
	}
	target := filepath.Join(home, "config.toml")
	change := FileChange{Path: filepath.ToSlash(target)}
	binary := strings.ReplaceAll(opts.NovelgenPath, `\`, `\\`)
	block := fmt.Sprintf("[mcp_servers.novelgen]\ncommand = \"%s\"\nargs = [\"mcp\", \"serve\"]\n", binary)
	existing, err := os.ReadFile(target)
	if err != nil && !os.IsNotExist(err) {
		return change, err
	}
	text := string(existing)
	if strings.Contains(text, "[mcp_servers.novelgen]") {
		change.Action = "unchanged"
		return change, nil
	}
	if strings.TrimSpace(text) == "" {
		text = block
		change.Action = "create"
	} else {
		text = strings.TrimRight(text, "\r\n") + "\n\n" + block
		change.Action = "update"
	}
	if !opts.Apply {
		return change, nil
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return change, err
	}
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		return change, err
	}
	return change, nil
}

func installDSH(root string, opts Options, result *Result) ([]FileChange, error) {
	body := []byte("# Novelgen DSH adapter\n\n" +
		"The DSH studio is installed as a Cordis bundle, not copied into the project.\n" +
		"Use the existing dsh-novelgen plugin for the Novel/Flow/Resources/Graph UI.\n\n" +
		"The shared workflow notes are under `.novelgen/workflows/`. The DSH plugin wraps\n" +
		"the same CLI write path (`tool replace`, `tool patch`, `tool refresh`).\n")
	change, err := writeFile(root, ".novelgen/adapters/dsh/README.md", body, opts)
	if err != nil {
		return nil, err
	}
	result.Warnings = append(result.Warnings,
		"DSH is installed as a Cordis bundle; this target writes only the shared workflow notes and a pointer")
	return []FileChange{change}, nil
}

func copyTree(src fs.FS, root, relDir string, opts Options, skip func(string) bool) ([]FileChange, error) {
	var changes []FileChange
	err := fs.WalkDir(src, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if skip != nil && skip(name) {
			return nil
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		change, err := writeFile(root, path.Join(relDir, filepath.ToSlash(name)), data, opts)
		if err != nil {
			return err
		}
		changes = append(changes, change)
		return nil
	})
	return changes, err
}

func writeFile(root, rel string, data []byte, opts Options) (FileChange, error) {
	rel = filepath.ToSlash(path.Clean(rel))
	data = renderTemplate(data, opts)
	target := filepath.Join(root, filepath.FromSlash(rel))
	change := FileChange{Path: rel}
	existing, err := os.ReadFile(target)
	switch {
	case err == nil && string(existing) == string(data):
		change.Action = "unchanged"
		return change, nil
	case err == nil && !opts.Force:
		change.Action = "skip"
		return change, nil
	case err == nil:
		change.Action = "update"
	case os.IsNotExist(err):
		change.Action = "create"
	default:
		return change, err
	}
	if !opts.Apply {
		return change, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return change, err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return change, err
	}
	return change, nil
}

func renderTemplate(data []byte, opts Options) []byte {
	binary := strings.TrimSpace(opts.NovelgenPath)
	if binary == "" {
		binary = "novelgen"
	}
	// The placeholder is used inside JSON/TOML string literals, so escape
	// backslashes for Windows paths.
	escaped := strings.ReplaceAll(binary, `\`, `\\`)
	return []byte(strings.ReplaceAll(string(data), "{{NOVELGEN}}", escaped))
}

func upsertManagedBlock(file, name, body string, opts Options) (FileChange, error) {
	change := FileChange{Path: "AGENTS.md"}
	existing, readErr := os.ReadFile(file)
	if readErr != nil && !os.IsNotExist(readErr) {
		return change, readErr
	}
	text := string(existing)
	begin := "<!-- " + name + ":begin -->"
	end := "<!-- " + name + ":end -->"
	block := begin + "\n" + strings.TrimSpace(body) + "\n" + end
	start := strings.Index(text, begin)
	stop := strings.Index(text, end)
	switch {
	case start >= 0 && stop > start:
		stop += len(end)
		text = text[:start] + block + text[stop:]
		change.Action = "update"
	case strings.TrimSpace(text) == "":
		text = block + "\n"
		change.Action = "create"
	default:
		text = strings.TrimRight(text, "\r\n") + "\n\n" + block + "\n"
		change.Action = "update"
	}
	if !opts.Apply {
		return change, nil
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		return change, err
	}
	return change, nil
}
