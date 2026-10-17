package toolplane

import (
	"context"

	"github.com/spf13/cobra"

	"novelgen/internal/fiction/mutation"
	"novelgen/internal/platform/store"
)

// The per-section entry points remain for tests and in-process callers that
// predate RunToolPatch. They are thin adapters over the shared mutation service.
func RunToolSetupPatch(cmd *cobra.Command, root *store.Root, patchBytes []byte, apply bool, warnings []string) error {
	result, err := NewMutationService(root).Patch(cmd.Context(), PatchRequest{
		Section:  "setup",
		Patch:    patchBytes,
		Warnings: warnings,
		Options: MutationOptions{
			Apply:    apply,
			Validate: true,
			Mode:     MutationMerge,
			Source:   "tool-cli",
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func RunToolCraftPatch(cmd *cobra.Command, root *store.Root, target, id string, patchBytes []byte, apply bool, warnings []string) error {
	result, err := NewMutationService(root).Patch(cmd.Context(), PatchRequest{
		Section:  "craft",
		Target:   target,
		ID:       id,
		Patch:    patchBytes,
		Warnings: warnings,
		Options: MutationOptions{
			Apply:    apply,
			Validate: true,
			Mode:     MutationMerge,
			Source:   "tool-cli",
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func RunToolRecapPatch(cmd *cobra.Command, root *store.Root, id string, patchBytes []byte, apply bool, warnings []string) error {
	result, err := NewMutationService(root).Patch(cmd.Context(), PatchRequest{
		Section:  "recap",
		ID:       id,
		Patch:    patchBytes,
		Warnings: warnings,
		Options: MutationOptions{
			Apply:    apply,
			Validate: true,
			Mode:     MutationMerge,
			Source:   "tool-cli",
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func runToolChapterPatch(cmd *cobra.Command, root *store.Root, id string, patchBytes []byte, apply bool, warnings []string) error {
	result, err := NewMutationService(root).Patch(cmd.Context(), PatchRequest{
		Section:  "chapter",
		ID:       id,
		Patch:    patchBytes,
		Warnings: warnings,
		Options: MutationOptions{
			Apply:          apply,
			Validate:       true,
			Mode:           MutationMerge,
			TargetWords:    ToolPatchFlags.TargetWords,
			RefreshDerived: ToolPatchFlags.RefreshDerived,
			RefreshChapter: toolplaneChapterRefresher(),
			Source:         "tool-cli",
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(cmd, result)
}

func toolplaneChapterRefresher() mutation.ChapterRefresher {
	return func(ctx context.Context, root *store.Root, id string, batchSize int) (*mutation.DerivedRefreshResult, error) {
		restoreLogger := SuppressToolRefreshLogs()
		defer restoreLogger()
		refresh, err := RefreshToolChapterDSL(ctx, root, id, batchSize)
		if err != nil {
			return nil, err
		}
		return &mutation.DerivedRefreshResult{DSLPath: refresh.DSLPath, CacheDir: refresh.CacheDir}, nil
	}
}
