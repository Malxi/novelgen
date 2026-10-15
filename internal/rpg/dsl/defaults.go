package dsl

import "strings"

// ApplySimulationDefaults fills the minimum metadata and player fields the
// simulator needs so a freshly parsed DSL can be simulated. It is used by the
// `simulate-dsl` command and the standalone checkers, which previously carried
// identical private copies of this logic.
func ApplySimulationDefaults(bookName string, dsl *DSL) {
	if dsl == nil {
		return
	}
	if dsl.Metadata == nil {
		dsl.Metadata = &Metadata{}
	}
	if strings.TrimSpace(dsl.Metadata.Title) == "" {
		dsl.Metadata.Title = bookName
	}
	if strings.TrimSpace(dsl.Metadata.DSLVersion) == "" {
		dsl.Metadata.DSLVersion = "0.1.0"
	}
	if dsl.Characters == nil {
		dsl.Characters = &Characters{}
	}
	if dsl.Characters.Player == nil {
		playerName := "主角"
		if len(dsl.Characters.NPCs) > 0 && strings.TrimSpace(dsl.Characters.NPCs[0].Name) != "" {
			playerName = dsl.Characters.NPCs[0].Name
		}
		dsl.Characters.Player = &Player{
			ID:   "char_player",
			Name: playerName,
		}
	}
}
