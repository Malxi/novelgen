package logic

import "novelgen/internal/models"

// ChapterContinuityBuilder builds writer-facing continuity snapshots without
// exposing the legacy StateMatrix type to write-stage callers.
type ChapterContinuityBuilder struct {
	stateManager *StateMatrixManager
}

// NewChapterContinuityBuilder creates a continuity builder for a project root.
func NewChapterContinuityBuilder(projectRoot string) *ChapterContinuityBuilder {
	return &ChapterContinuityBuilder{
		stateManager: NewStateMatrixManager(projectRoot),
	}
}

// SetUseRPGDSL controls whether generated RPG DSL state_delta facts are folded
// into the continuity snapshot.
func (b *ChapterContinuityBuilder) SetUseRPGDSL(enabled bool) {
	if b == nil || b.stateManager == nil {
		return
	}
	b.stateManager.SetUseRPGDSL(enabled)
}

// BuildBefore returns the continuity state before targetChapter begins. The
// target chapter's own planned events remain future instructions, not already
// true state.
func (b *ChapterContinuityBuilder) BuildBefore(outline *models.Outline, targetChapter *models.Chapter) *models.ChapterContinuity {
	if b == nil || b.stateManager == nil {
		return &models.ChapterContinuity{}
	}
	return chapterContinuityFromStateMatrix(b.stateManager.CalculateStateMatrixBefore(outline, targetChapter))
}

// BuildAfter returns the continuity state after targetChapter has been applied.
// It is retained for analysis and legacy parity; write generation should prefer
// BuildBefore.
func (b *ChapterContinuityBuilder) BuildAfter(outline *models.Outline, targetChapter *models.Chapter) *models.ChapterContinuity {
	if b == nil || b.stateManager == nil {
		return &models.ChapterContinuity{}
	}
	return chapterContinuityFromStateMatrix(b.stateManager.CalculateStateMatrix(outline, targetChapter))
}

func chapterContinuityFromStateMatrix(state *models.StateMatrix) *models.ChapterContinuity {
	if state == nil {
		return &models.ChapterContinuity{}
	}
	return &models.ChapterContinuity{
		RPG:        state.RPG,
		Characters: state.Characters,
		Locations:  state.Locations,
		Items:      state.Items,
		Premises:   state.Premises,
		Gates:      state.Gates,
		Status:     state.Status,
		Memories:   state.Memories,
	}
}
