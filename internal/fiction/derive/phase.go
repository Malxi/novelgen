package derive

import (
	"novelgen/internal/fiction/rpg/dsl"
)

// InferPhaseFromFilename is kept as the derive-package spelling for callers
// that already import this package. The implementation lives in the RPG-DSL
// package so lower layers can use it without pulling in agent orchestration.
func InferPhaseFromFilename(filename string) dsl.MergePhase {
	return dsl.InferPhaseFromFilename(filename)
}
