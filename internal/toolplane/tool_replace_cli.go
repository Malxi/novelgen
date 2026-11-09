package toolplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	models "novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/craft"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/fiction/mutation"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

// ToolReplaceFlags holds the flags for `novelgen tool replace`.
var ToolReplaceFlags struct {
	Target    string
	ID        string
	Apply     bool
	DryRun    bool
	MaxIssues int
}

var ToolReplaceCmd = &cobra.Command{
	Use:   "replace setup|outline|craft|chapter|recap",
	Short: "Replace one complete canonical artifact",
	Long: `Replace one complete canonical artifact (setup, outline, craft, chapter or recap).

Unlike "tool patch", this command does not merge fields. It parses, normalizes,
validates, checkpoints and writes the complete artifact through the same
MutationService used by the editor-facing "storage replace". It is intended for
generation/regeneration workflows where the agent owns the full artifact.

The command is a validated dry run by default and writes only with --apply.
For whole-outline replacements, the result warns about chapters the new outline
would remove (and whether saved prose or a recap exists).

Examples:
  printf '%s' '<complete setup json>' | novelgen tool replace setup --apply
  printf '%s' '<complete outline json>' | novelgen tool replace outline --apply
  printf '%s' '<craft map json>' | novelgen tool replace craft --target character --apply
  printf '%s' '<chapter markdown>' | novelgen tool replace chapter --id P1-V1-C1 --apply
  printf '%s' '<recap json>' | novelgen tool replace recap --id P1-V1-C1 --apply`,
	Args: cobra.ExactArgs(1),
	RunE: RunToolReplace,
}

// RunToolReplace is the CLI adapter for complete-artifact replacement.
func RunToolReplace(cmd *cobra.Command, args []string) error {
	section := NormalizeKey(args[0])
	switch section {
	case "setup", "outline", "craft", "chapter", "recap":
	default:
		return fmt.Errorf("unsupported replace section %q; valid sections: setup, outline, craft, chapter, recap", section)
	}

	book, err := project.ForCommand(cmd)
	if err != nil {
		return err
	}
	root := book.Root()
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("failed to read replace stdin: %w", err)
	}
	data = utils.StripUTF8BOM(data)
	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("complete artifact JSON is required on stdin")
	}

	target := NormalizeKey(ToolReplaceFlags.Target)
	id := strings.TrimSpace(ToolReplaceFlags.ID)
	apply := ToolReplaceFlags.Apply && !ToolReplaceFlags.DryRun
	warnings := []string{}
	if ToolReplaceFlags.Apply && ToolReplaceFlags.DryRun {
		warnings = append(warnings, "--dry-run forced a dry run; the replacement was not applied")
	}
	if !apply {
		warnings = append(warnings, "dry run: pass --apply to write the replacement")
	}
	opts := mutation.MutationOptions{
		Apply:    apply,
		DryRun:   !apply,
		Validate: true,
		Source:   "tool-replace",
	}
	service := NewMutationService(root)
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var result *mutation.ToolPatchResult
	switch section {
	case "setup":
		var setup models.StorySetup
		if err := json.Unmarshal(data, &setup); err != nil {
			return fmt.Errorf("invalid setup JSON: %w", err)
		}
		result, err = service.ReplaceSetup(ctx, &setup, opts)
	case "outline":
		var outline models.Outline
		if err := json.Unmarshal(data, &outline); err != nil {
			return fmt.Errorf("invalid outline JSON: %w", err)
		}
		result, err = service.ReplaceOutline(ctx, &outline, opts)
	case "craft":
		kind, kindErr := craftKind(ToolReplaceFlags.Target)
		if kindErr != nil {
			return kindErr
		}
		var elements map[string]json.RawMessage
		if err := json.Unmarshal(data, &elements); err != nil {
			return fmt.Errorf("invalid craft JSON object: %w", err)
		}
		result, err = service.ReplaceCraftElements(ctx, kind, elements, opts)
	case "chapter":
		if id == "" {
			return fmt.Errorf("--id is required for chapter replace")
		}
		result, err = service.ReplaceChapter(ctx, id, string(data), opts)
	case "recap":
		if id == "" {
			return fmt.Errorf("--id is required for recap replace")
		}
		var recap models.ChapterRecap
		if err := json.Unmarshal(data, &recap); err != nil {
			return fmt.Errorf("invalid recap JSON: %w", err)
		}
		chapterID, err := resolveRecapChapterID(recap.ChapterID, id)
		if err != nil {
			return err
		}
		recap.ChapterID = chapterID
		result, err = service.ReplaceRecap(ctx, &recap, opts)
	}
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("replace produced no result")
	}
	result.Warnings = append(warnings, result.Warnings...)
	if target != "" && section != "craft" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("--target %q is not used by %s replace and was ignored", target, section))
	}
	limitToolPatchResult(result, ToolReplaceFlags.MaxIssues)
	return writeJSON(cmd, result)
}

func craftKind(target string) (store.CraftKind, error) {
	switch craft.NormalizeTarget(target) {
	case "character":
		return store.CraftCharacters, nil
	case "item":
		return store.CraftItems, nil
	case "location":
		return store.CraftLocations, nil
	case "organization":
		return store.CraftOrganizations, nil
	default:
		return "", fmt.Errorf("--target is required for craft replace (character/item/location/organization)")
	}
}

// resolveRecapChapterID keeps --id and the recap body's chapter_id from
// disagreeing: a body that names one chapter and a flag that names another
// would otherwise write the body's chapter without warning.
func resolveRecapChapterID(bodyID, flagID string) (string, error) {
	bodyID = strings.TrimSpace(bodyID)
	flagID = strings.TrimSpace(flagID)
	switch {
	case bodyID == "":
		return flagID, nil
	case flagID == "" || bodyID == flagID:
		return bodyID, nil
	default:
		return "", fmt.Errorf("recap chapter_id %q does not match --id %q", bodyID, flagID)
	}
}
