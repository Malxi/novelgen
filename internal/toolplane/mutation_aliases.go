package toolplane

import "novelgen/internal/fiction/mutation"

// Compatibility aliases keep the existing tool-plane API and JSON DTOs stable
// while the canonical write service lives in fiction/mutation.
type (
	MutationService = mutation.MutationService
	MutationOptions = mutation.MutationOptions
	MutationMode    = mutation.MutationMode
	PatchRequest    = mutation.PatchRequest
	ToolPatchResult = mutation.ToolPatchResult
	ToolPatchChange = mutation.ToolPatchChange
)

const (
	MutationMerge   = mutation.MutationMerge
	MutationReplace = mutation.MutationReplace
)

var (
	NewMutationService             = mutation.NewMutationService
	ValidateToolPatchBytes         = mutation.ValidateToolPatchBytes
	RunToolPatchValidationCheck    = mutation.RunToolPatchValidationCheck
	RunToolPatchValidationCheckAt  = mutation.RunToolPatchValidationCheckAt
	DiffStructFields               = mutation.DiffStructFields
	CloneOutline                   = mutation.CloneOutline
	CloneStorySetup                = mutation.CloneStorySetup
	ApplyChapterPatch              = mutation.ApplyChapterPatch
	ApplyVolumePatch               = mutation.ApplyVolumePatch
	ApplyCraftElementPatch         = mutation.ApplyCraftElementPatch
	ApplyCraftElementReplace       = mutation.ApplyCraftElementReplace
	NormalizeCraftPatchObject      = mutation.NormalizeCraftPatchObject
	ApplySetupPatchObject          = mutation.ApplySetupPatchObject
	NormalizeChapterPatchContent   = mutation.NormalizeChapterPatchContent
	ValidateToolChapterPatchLength = mutation.ValidateToolChapterPatchLength
	OutlinePatchPostCheckCommand   = mutation.OutlinePatchPostCheckCommand
	BuildChapterPatchNextActions   = mutation.BuildChapterPatchNextActions
)

func MergeJSONPatchObject[T any](original T, patchBytes []byte) (T, error) {
	return mutation.MergeJSONPatchObject(original, patchBytes)
}
