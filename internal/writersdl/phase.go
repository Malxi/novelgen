package writersdl

import (
	"strings"

	"novelgen/internal/rpg/dsl"
)

// InferPhaseFromFilename maps a phased RPG-DSL filename to its merge phase.
func InferPhaseFromFilename(filename string) dsl.MergePhase {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "setup") {
		return dsl.PhaseSetup
	}
	if strings.Contains(lower, "outline") {
		return dsl.PhaseOutline
	}
	if strings.Contains(lower, "craft") {
		return dsl.PhaseCraft
	}
	if strings.Contains(lower, "system") {
		return dsl.PhaseSystems
	}
	return dsl.PhaseFinal
}
