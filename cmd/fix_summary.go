package cmd

import "fmt"

type FixSummary struct {
	TeleportAttempted bool
	TeleportAttempts  int
	TeleportPassed    bool

	CharacterAttempted bool
	CharacterAttempts  int
	CharacterPassed    bool
}

func (s FixSummary) String() string {
	return fmt.Sprintf("teleport_fix=%v attempts=%d passed=%v | character_fix=%v attempts=%d passed=%v",
		s.TeleportAttempted, s.TeleportAttempts, s.TeleportPassed,
		s.CharacterAttempted, s.CharacterAttempts, s.CharacterPassed,
	)
}
