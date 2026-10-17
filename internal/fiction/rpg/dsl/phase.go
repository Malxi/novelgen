package dsl

import "strings"

// InferPhaseFromFilename maps a phased RPG-DSL filename to its merge phase.
func InferPhaseFromFilename(filename string) MergePhase {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "setup") {
		return PhaseSetup
	}
	if strings.Contains(lower, "outline") {
		return PhaseOutline
	}
	if strings.Contains(lower, "craft") {
		return PhaseCraft
	}
	if strings.Contains(lower, "system") {
		return PhaseSystems
	}
	return PhaseFinal
}
