// The novelgen-owned patch buffer: how agent workflows hand long chapter prose to
// tool patch chapter without shell temp files or one huge patch-json argument.

package toolplane

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

func RunToolPatchBuffer(cmd *cobra.Command, args []string) error {
	op := NormalizeKey(args[0])
	if !allowedToolPatchBufferOps[op] {
		return fmt.Errorf("unsupported patch-buffer operation %q", op)
	}
	id := strings.TrimSpace(ToolPatchBufferFlags.ID)
	if id == "" {
		return fmt.Errorf("--id is required")
	}
	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root()
	path, err := ToolPatchBufferPath(root, id)
	if err != nil {
		return err
	}
	switch op {
	case "clear":
		if err := root.Remove(path); err != nil {
			return fmt.Errorf("failed to clear patch buffer: %w", err)
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": 0})
	case "append":
		text := ToolPatchBufferFlags.Text
		if ToolPatchBufferFlags.Stdin && text != "" {
			return fmt.Errorf("use either --text or --stdin, not both")
		}
		if text == "" || ToolPatchBufferFlags.Stdin {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read patch buffer stdin: %w", err)
			}
			text = string(utils.StripUTF8BOM(data))
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("buffer text is required via --text or stdin")
		}
		if err := utils.ValidateNoSuspiciousPatchText(map[string]interface{}{"text": text}); err != nil {
			return fmt.Errorf("patch buffer rejected: %w", err)
		}
		// root.Append creates the buffer's parent directory and appends through the
		// store, so the buffer plane shares one filesystem seam with story state.
		if err := root.Append(path, []byte(text), store.FileMode); err != nil {
			return fmt.Errorf("failed to append patch buffer: %w", err)
		}
		info, err := root.FS().Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat patch buffer: %w", err)
		}
		return writeJSON(cmd, map[string]interface{}{"ok": true, "id": id, "path": path, "bytes": info.Size()})
	case "show":
		data, err := root.Read(path)
		if err != nil {
			return fmt.Errorf("failed to read patch buffer: %w", err)
		}
		maxChars := ToolPatchBufferFlags.MaxChars
		if maxChars <= 0 {
			maxChars = 400
		}
		if maxChars > 2000 {
			maxChars = 2000
		}
		runes := []rune(string(data))
		preview := string(runes)
		truncated := false
		if len(runes) > maxChars {
			preview = string(runes[:maxChars])
			truncated = true
		}
		return writeJSON(cmd, map[string]interface{}{
			"ok":        true,
			"id":        id,
			"path":      path,
			"bytes":     len(data),
			"chars":     len(runes),
			"preview":   preview,
			"truncated": truncated,
		})
	}
	return fmt.Errorf("unsupported patch-buffer operation %q", op)
}

func LoadToolPatchBuffer(root *store.Root, id, section string) ([]byte, error) {
	path, err := ToolPatchBufferPath(root, id)
	if err != nil {
		return nil, err
	}
	data, err := root.Read(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read patch buffer %q: %w", id, err)
	}
	content := string(utils.StripUTF8BOM(data))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("patch buffer %q is empty", id)
	}
	if NormalizeKey(section) == "chapter" {
		payload, err := json.Marshal(map[string]string{"content": content})
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	return []byte(content), nil
}

// ToolPatchBufferPath returns the file `tool patch-buffer` keeps a chunked patch
// in. The buffer is novelgen-owned scratch state under the project's agent work
// directory, never story state.
func ToolPatchBufferPath(root *store.Root, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("patch buffer id is required")
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			return "", fmt.Errorf("patch buffer id %q contains unsupported character %q", id, r)
		}
	}
	clean := b.String()
	if clean == "" || clean != id {
		return "", fmt.Errorf("invalid patch buffer id %q", id)
	}
	return filepath.Join(root.AgentPatchDir(), clean+".txt"), nil
}
