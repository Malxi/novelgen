package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/project"
	"novelgen/internal/platform/store"
	"novelgen/internal/platform/utils"
)

// MutationMode controls how a proposed artifact is combined with the project's
// current state. Merge is the agent-facing patch semantic; replace is used by
// generation/regeneration paths that produce a complete artifact.
type MutationMode string

const (
	MutationMerge   MutationMode = "merge"
	MutationReplace MutationMode = "replace"
)

// MutationOptions is the shared control surface for project mutations.
//
// Validate is deliberately separate from Apply. The CLI tool plane always
// validates an agent patch before --apply. Host pipelines that have already run
// their own quality gate can commit a generated artifact without paying for a
// second full check, while still getting normalization, checkpoints and the
// store's atomic write.
type MutationOptions struct {
	Apply          bool
	Validate       bool
	Mode           MutationMode
	TargetWords    int
	RefreshDerived bool
	Source         string
	// Normalize and WriteDerived control the outline replace path. The normal
	// ReplaceOutline method enables both; raw outline commits used by scoped
	// Agent SDK edits disable them so unselected volumes are not rewritten as a
	// side effect.
	Normalize    bool
	WriteDerived bool
	// RefreshChapter is injected by the adapter that owns the derived-DSL
	// pipeline. Keeping it as a callback prevents the canonical write service
	// from depending on the agent/derive layer.
	RefreshChapter ChapterRefresher
}

// DerivedRefreshResult is the small result shape the chapter mutation needs from
// a derived-DSL refresh.
type DerivedRefreshResult struct {
	DSLPath  string
	CacheDir string
}

// ChapterRefresher regenerates derived chapter RPG-DSL after a chapter write.
type ChapterRefresher func(ctx context.Context, root *store.Root, id string, batchSize int) (*DerivedRefreshResult, error)

func (o MutationOptions) normalized() MutationOptions {
	if o.Mode == "" {
		o.Mode = MutationMerge
	}
	if o.Source == "" {
		o.Source = "host"
	}
	return o
}

// MutationService is the single write surface for project artifacts produced by
// agents and pipelines. The Cobra `novelgen tool` commands are one adapter over
// it; in-process chat/typed-JSON stages call the typed methods below.
type MutationService struct {
	root *store.Root
}

// PatchRequest is the generic, transport-independent form of a tool patch.
// The Cobra command parses flags/stdin into this shape, while in-process
// callers can use it directly when their proposal is already JSON.
type PatchRequest struct {
	Section  string
	Target   string
	ID       string
	Mode     MutationMode
	Patch    []byte
	Warnings []string
	Options  MutationOptions
}

// NewMutationService returns a mutation service over an explicit project root.
// New pipeline code must pass the root explicitly rather than relying on the
// ambient store.Default root.
func NewMutationService(root *store.Root) *MutationService {
	return &MutationService{root: root}
}

// Root returns the project root the service writes to.
func (s *MutationService) Root() *store.Root {
	if s == nil {
		return nil
	}
	return s.root
}

func (s *MutationService) requireRoot() (*store.Root, error) {
	if s == nil || s.root == nil || s.root.Dir() == "" {
		return nil, fmt.Errorf("project root is required for a project mutation")
	}
	return s.root, nil
}

// Patch applies a generic mutation request. It is the shared implementation
// behind `novelgen tool patch`; typed Replace*/Upsert* methods are thin
// convenience wrappers for host pipelines.
func (s *MutationService) Patch(ctx context.Context, req PatchRequest) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	opts := req.Options
	if req.Mode != "" {
		opts.Mode = req.Mode
	}
	opts = opts.normalized()
	switch utils.NormalizeKey(req.Section) {
	case "setup":
		return runSetupMutation(root, req.Patch, opts, req.Warnings)
	case "craft":
		return runCraftMutation(root, req.Target, req.ID, req.Patch, opts, req.Warnings)
	case "chapter":
		return runChapterMutation(ctx, root, req.ID, req.Patch, opts, req.Warnings)
	case "recap":
		return runRecapMutation(root, req.ID, req.Patch, opts, req.Warnings)
	case "outline":
		return runOutlinePatchMutation(root, req.Target, req.ID, req.Patch, opts, req.Warnings)
	default:
		return nil, fmt.Errorf("unsupported mutation section %q", req.Section)
	}
}

// ReplaceSetup commits a complete story setup. The artifact is normalized and,
// when requested, checked before the canonical JSON and derived markdown are
// written.
func (s *MutationService) ReplaceSetup(_ context.Context, setup *models.StorySetup, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if setup == nil {
		return nil, fmt.Errorf("story setup is nil")
	}
	data, err := json.Marshal(setup)
	if err != nil {
		return nil, fmt.Errorf("encode story setup: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runSetupMutation(root, data, opts, nil)
}

// UpsertCraftElements commits a batch of generated craft elements. Existing
// elements with the same key are replaced; other elements in the file are
// preserved.
func (s *MutationService) UpsertCraftElements(_ context.Context, kind store.CraftKind, values interface{}, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode craft elements: %w", err)
	}
	var elements map[string]map[string]interface{}
	if err := json.Unmarshal(data, &elements); err != nil {
		return nil, fmt.Errorf("decode craft elements: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationMerge
	return runCraftBulkMutation(root, kind, elements, opts, nil, false)
}

// ReplaceCraftElements replaces one craft file with the supplied complete set of
// elements. Unlike UpsertCraftElements, elements omitted from values are
// removed from the file; this is the editor "save whole file" semantic.
func (s *MutationService) ReplaceCraftElements(_ context.Context, kind store.CraftKind, values interface{}, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode craft elements: %w", err)
	}
	var elements map[string]map[string]interface{}
	if err := json.Unmarshal(data, &elements); err != nil {
		return nil, fmt.Errorf("decode craft elements: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runCraftBulkMutation(root, kind, elements, opts, nil, true)
}

// UpsertCraftElement commits one generated craft element.
func (s *MutationService) UpsertCraftElement(_ context.Context, kind, key string, value interface{}, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode craft element: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runCraftMutation(root, kind, key, data, opts, nil)
}

// ReplaceChapter commits a complete final chapter body.
func (s *MutationService) ReplaceChapter(ctx context.Context, chapterID, content string, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(toolChapterPatch{ID: chapterID, Content: content})
	if err != nil {
		return nil, fmt.Errorf("encode chapter patch: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runChapterMutation(ctx, root, chapterID, data, opts, nil)
}

// ReplaceChapterObject commits chapter content using an already-loaded
// chapter. Host pipelines often have the outline chapter in hand and should not
// need a second outline read merely to normalize the title. The CLI patch path
// still uses ReplaceChapter and requires the chapter to exist in the outline.
func (s *MutationService) ReplaceChapterObject(ctx context.Context, chapter *models.Chapter, content string, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if chapter == nil {
		return nil, fmt.Errorf("chapter is nil")
	}
	data, err := json.Marshal(toolChapterPatch{ID: chapter.ID, Content: content})
	if err != nil {
		return nil, fmt.Errorf("encode chapter patch: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runChapterMutationForChapter(ctx, root, chapter.ID, chapter, data, opts, nil)
}

// ReplaceDraft commits a legacy draft through the same project mutation
// surface. Drafts are not canonical story state, so they do not carry a
// checkpoint or quality gate; the write still goes through store and the
// project mutation lock.
func (s *MutationService) ReplaceDraft(_ context.Context, chapter *models.Chapter, draft string, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if chapter == nil {
		return nil, fmt.Errorf("chapter is nil")
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	var result *ToolPatchResult
	err = root.WithMutationLock(func() error {
		before, readErr := root.LoadDraftContent(chapter.ID)
		if readErr != nil {
			return fmt.Errorf("failed to read draft %s: %w", chapter.ID, readErr)
		}
		if err := project.Open(root).SaveDraft(chapter, draft); err != nil {
			return err
		}
		result = &ToolPatchResult{
			OK:      true,
			Applied: true,
			DryRun:  false,
			Target:  "draft",
			ID:      chapter.ID,
			Files:   map[string]string{"draft": root.DraftFile(chapter.ID)},
			Meta: map[string]interface{}{
				"project_root": root.Dir(),
				"mode":         string(opts.Mode),
				"source":       opts.Source,
				"changed":      before != draft,
			},
		}
		return nil
	})
	return result, err
}

// ReplaceRecap commits a complete chapter recap.
func (s *MutationService) ReplaceRecap(_ context.Context, value *models.ChapterRecap, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("chapter recap is nil")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode recap: %w", err)
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	return runRecapMutation(root, value.ChapterID, data, opts, nil)
}

// ReplaceOutline commits a complete outline (generation/regeneration path).
// Incremental agent repairs should use Patch with MutationMerge instead.
func (s *MutationService) ReplaceOutline(_ context.Context, outline *models.Outline, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	opts.Normalize = true
	opts.WriteDerived = true
	return runOutlineReplaceMutation(root, outline, opts)
}

// ReplaceOutlineRaw commits a complete outline without re-normalizing it or
// rewriting derived markdown. Scoped Agent SDK improvement uses this when only
// selected volumes may change and a whole-outline normalization would rewrite
// untouched volumes as a side effect.
func (s *MutationService) ReplaceOutlineRaw(_ context.Context, outline *models.Outline, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	opts = opts.normalized()
	opts.Apply = true
	opts.Mode = MutationReplace
	opts.Normalize = false
	opts.WriteDerived = false
	return runOutlineReplaceMutation(root, outline, opts)
}

// CommitOutlineRaw writes a canonical outline snapshot without normalization,
// validation, checkpoints, or derived markdown. Compose uses it for incremental
// progress saves while the final outline is still being assembled.
func (s *MutationService) CommitOutlineRaw(_ context.Context, outline *models.Outline, opts MutationOptions) (*ToolPatchResult, error) {
	root, err := s.requireRoot()
	if err != nil {
		return nil, err
	}
	if outline == nil {
		return nil, fmt.Errorf("outline is nil")
	}
	opts = opts.normalized()
	var result *ToolPatchResult
	err = root.WithMutationLock(func() error {
		path := root.OutlineJSON()
		if err := root.SaveOutlineAt(path, outline); err != nil {
			return fmt.Errorf("failed to save outline progress: %w", err)
		}
		result = &ToolPatchResult{
			OK:      true,
			Applied: true,
			DryRun:  false,
			Target:  "outline",
			ID:      "outline",
			Files:   map[string]string{"outline": path},
			Meta: map[string]interface{}{
				"project_root": root.Dir(),
				"mode":         "progress",
				"source":       opts.Source,
			},
		}
		return nil
	})
	return result, err
}
