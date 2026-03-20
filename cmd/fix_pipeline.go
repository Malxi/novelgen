package cmd

import (
	"nolvegen/internal/logger"
	"nolvegen/internal/models"
)

// applyImproveFixesDraft runs enabled minimal-change fixers for draft improve.
// Order matters: teleport bridge first, then character presence.
func applyImproveFixesDraft(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	outline *models.Outline,
	original string,
	suggestions string,
	teleportEnabled bool,
	teleportRetries int,
	teleportGen bridgeGenFunc,
	charEnabled bool,
	charRetries int,
	charKnown []string,
	charGen patchGenFunc,
) (string, FixSummary) {
	out := original
	sum := FixSummary{}

	if teleportEnabled {
		prevRecap := loadPreviousRecapStruct(outline, chapter)
		fixed, attempted, attemptsUsed, passed := runTeleportBridgeAutoFix(log, workerID, chapter, prevRecap, out, suggestions, teleportRetries, teleportGen)
		sum.TeleportAttempted = attempted
		sum.TeleportAttempts = attemptsUsed
		sum.TeleportPassed = passed
		if attempted {
			out = fixed
		}
	}

	if charEnabled {
		fixed, attempted, attemptsUsed, passed := runCharacterPresenceAutoFix(log, workerID, chapter, out, charKnown, suggestions, charRetries, charGen)
		sum.CharacterAttempted = attempted
		sum.CharacterAttempts = attemptsUsed
		sum.CharacterPassed = passed
		if attempted {
			out = fixed
		}
	}

	return out, sum
}

// applyImproveFixesWrite runs enabled minimal-change fixers for write improve.
func applyImproveFixesWrite(
	log logger.LoggerInterface,
	workerID int,
	chapter *models.Chapter,
	outline *models.Outline,
	original string,
	suggestions string,
	teleportEnabled bool,
	teleportRetries int,
	teleportGen bridgeGenFunc,
	charEnabled bool,
	charRetries int,
	charKnown []string,
	charGen patchGenFunc,
) (string, FixSummary) {
	// Same fix order as draft.
	out := original
	sum := FixSummary{}
	if teleportEnabled {
		prevRecap := loadPreviousRecapStruct(outline, chapter)
		fixed, attempted, attemptsUsed, passed := runTeleportBridgeAutoFix(log, workerID, chapter, prevRecap, out, suggestions, teleportRetries, teleportGen)
		sum.TeleportAttempted = attempted
		sum.TeleportAttempts = attemptsUsed
		sum.TeleportPassed = passed
		if attempted {
			out = fixed
		}
	}
	if charEnabled {
		fixed, attempted, attemptsUsed, passed := runCharacterPresenceAutoFix(log, workerID, chapter, out, charKnown, suggestions, charRetries, charGen)
		sum.CharacterAttempted = attempted
		sum.CharacterAttempts = attemptsUsed
		sum.CharacterPassed = passed
		if attempted {
			out = fixed
		}
	}
	return out, sum
}
