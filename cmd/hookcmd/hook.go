// Package hookcmd implements harness hook entry points for Novelgen.
package hookcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"

	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Agent harness hook entry points",
	Long: `Hook entry points used by the Claude Code adapter.

These commands read the harness hook JSON on stdin and print a hook response on
stdout. They are intentionally small: project writes still go through the
novelgen tool plane.`,
}

var guardWriteCmd = &cobra.Command{
	Use:   "guard-write",
	Short: "Deny direct edits to canonical story files",
	RunE:  runGuardWrite,
}

var sessionStartCmd = &cobra.Command{
	Use:   "session-start",
	Short: "Print a short Novelgen project status for a new agent session",
	RunE:  runSessionStart,
}

func init() {
	hookCmd.AddCommand(guardWriteCmd, sessionStartCmd)
}

// NewCommand returns the `novelgen hook` command tree.
func NewCommand() *cobra.Command {
	return hookCmd
}

type hookInput struct {
	ToolName  string                 `json:"tool_name"`
	ToolInput map[string]interface{} `json:"tool_input"`
	CWD       string                 `json:"cwd"`
}

func runGuardWrite(cmd *cobra.Command, _ []string) error {
	if strings.TrimSpace(os.Getenv("NOVELGEN_ALLOW_DIRECT_WRITE")) == "1" {
		return writeHookDecision(cmd, "allow", "")
	}
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return writeHookDecision(cmd, "allow", "")
	}
	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return writeHookDecision(cmd, "allow", "")
	}
	switch input.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
	default:
		return writeHookDecision(cmd, "allow", "")
	}
	path := hookTargetPath(input)
	if path == "" {
		return writeHookDecision(cmd, "allow", "")
	}
	book, err := project.ForCommand(cmd)
	if err != nil {
		return writeHookDecision(cmd, "allow", "")
	}
	root := book.Root().Dir()
	if !filepath.IsAbs(path) {
		base := input.CWD
		if base == "" {
			base = root
		}
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return writeHookDecision(cmd, "allow", "")
	}
	rel = filepath.ToSlash(rel)
	if !isCanonicalWrite(book.Root(), path) {
		return writeHookDecision(cmd, "allow", "")
	}
	reason := fmt.Sprintf(
		"Direct write to %s is blocked. Use `novelgen tool replace` for complete setup/outline artifacts, `novelgen tool patch` for incremental repairs, or `novelgen tool refresh` for derived files. Set NOVELGEN_ALLOW_DIRECT_WRITE=1 only for an intentional migration.",
		rel,
	)
	return writeHookDecision(cmd, "deny", reason)
}

func isCanonicalWrite(root *store.Root, target string) bool {
	if root == nil {
		return false
	}
	if filepath.Clean(target) == filepath.Clean(root.NovelJSON()) {
		return true
	}
	for _, dir := range []string{root.StoryDir(), root.ChaptersDir()} {
		rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(target))
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func hookTargetPath(input hookInput) string {
	for _, key := range []string{"file_path", "path", "notebook_path"} {
		if value, ok := input.ToolInput[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeHookDecision(cmd *cobra.Command, decision, reason string) error {
	payload := map[string]interface{}{
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":      "PreToolUse",
			"permissionDecision": decision,
		},
	}
	if reason != "" {
		payload["hookSpecificOutput"].(map[string]interface{})["permissionDecisionReason"] = reason
	}
	return writeJSON(cmd, payload)
}

func runSessionStart(cmd *cobra.Command, _ []string) error {
	context := "Novelgen project not found from the current directory."
	if book, err := project.ForCommand(cmd); err == nil {
		context = sessionContext(book)
	}
	payload := map[string]interface{}{
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":     "SessionStart",
			"additionalContext": context,
		},
	}
	return writeJSON(cmd, payload)
}

func sessionContext(book *project.Book) string {
	if book == nil || book.Root() == nil {
		return "Novelgen project: unknown"
	}
	status, err := book.Status()
	if err != nil {
		return fmt.Sprintf("Novelgen project: %s\nStatus unavailable: %v", book.Root().Dir(), err)
	}
	written := 0
	for _, chapter := range status.Chapters {
		if chapter.Written {
			written++
		}
	}
	context := fmt.Sprintf(
		"Novelgen project: %s\nSetup: %t\nStructure: %d parts / %d volumes / %d chapters, %d written\nNext: %s\nUse `novelgen tool query|check|replace|patch|refresh`; do not edit story/ or chapters/ directly.",
		status.Root, status.Setup, status.Outline.Parts, status.Outline.Volumes, status.Outline.Chapters, written, status.Next.Stage,
	)
	if len(status.Warnings) > 0 {
		context += "\nWarnings: " + strings.Join(status.Warnings, "; ")
	}
	return context
}

func writeJSON(cmd *cobra.Command, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
