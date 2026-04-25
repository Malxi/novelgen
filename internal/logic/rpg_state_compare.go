package logic

import (
	"fmt"
	"sort"
	"strings"

	"novelgen/internal/models"
)

type RPGStateComparison struct {
	ChapterID string          `json:"chapter_id,omitempty"`
	Drifts    []RPGStateDrift `json:"drifts,omitempty"`
}

type RPGStateDrift struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Observed string `json:"observed,omitempty"`
	Note     string `json:"note,omitempty"`
}

// FilterRPGStateComparison keeps only drifts at or above the requested severity.
// Unknown severities are treated as info so experimental checks stay visible
// when callers explicitly request the lowest level.
func FilterRPGStateComparison(comparison RPGStateComparison, minSeverity string) RPGStateComparison {
	minRank := rpgDriftSeverityRank(minSeverity)
	if minRank <= 0 {
		minRank = rpgDriftSeverityRank("warning")
	}
	filtered := RPGStateComparison{ChapterID: comparison.ChapterID}
	for _, drift := range comparison.Drifts {
		if rpgDriftSeverityRank(drift.Severity) >= minRank {
			filtered.Drifts = append(filtered.Drifts, drift)
		}
	}
	return filtered
}

func rpgDriftSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 3
	case "warning", "warn":
		return 2
	case "info", "informational":
		return 1
	default:
		return 0
	}
}

// CompareRPGStates compares outline-expected state with DSL-observed state.
func CompareRPGStates(expected, observed *models.RPGState, chapter *models.Chapter) RPGStateComparison {
	comparison := RPGStateComparison{}
	if chapter != nil {
		comparison.ChapterID = chapter.ID
	}
	if expected == nil || observed == nil {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "missing_state",
			Target:   comparison.ChapterID,
			Note:     "expected or observed RPG state is missing",
		})
		return comparison
	}

	relevantChars := relevantRPGCompareCharacters(expected, observed, chapter)
	for _, name := range relevantChars {
		compareRPGCharacter(&comparison, name, expected.Characters[name], observed.Characters[name])
	}

	for _, key := range sortedCompareResourceKeys(expected.Resources, observed.Resources) {
		compareRPGResource(&comparison, key, expected.Resources[key], observed.Resources[key])
	}

	return comparison
}

func compareRPGCharacter(comparison *RPGStateComparison, name string, expected, observed *models.RPGCharacterState) {
	if expected == nil && observed == nil {
		return
	}
	if expected == nil {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "info",
			Kind:     "extra_character_state",
			Target:   name,
			Observed: summarizeRPGCharacter(observed),
			Note:     "observed DSL introduced character state not present in outline expectation",
		})
		return
	}
	if observed == nil {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "missing_character_state",
			Target:   name,
			Expected: summarizeRPGCharacter(expected),
		})
		return
	}
	if expected.Alive != observed.Alive {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "critical",
			Kind:     "character_life_mismatch",
			Target:   name,
			Field:    "alive",
			Expected: fmt.Sprintf("%t", expected.Alive),
			Observed: fmt.Sprintf("%t", observed.Alive),
		})
	}
	if expected.Level > 0 && observed.Level > 0 && expected.Level != observed.Level {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "character_level_mismatch",
			Target:   name,
			Field:    "level",
			Expected: fmt.Sprintf("%d", expected.Level),
			Observed: fmt.Sprintf("%d", observed.Level),
		})
	}
	if expected.Realm != "" && observed.Realm != "" && expected.Realm != observed.Realm && !compatibleRealm(expected, observed) {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "character_realm_mismatch",
			Target:   name,
			Field:    "realm",
			Expected: expected.Realm,
			Observed: observed.Realm,
		})
	}
	compareStringMap(comparison, "character_status_mismatch", name, expected.Status, observed.Status)
	compareIntMap(comparison, "character_inventory_mismatch", name, expected.Inventory, observed.Inventory)
}

func compatibleRealm(expected, observed *models.RPGCharacterState) bool {
	if expected == nil || observed == nil {
		return false
	}
	if expected.Level > 0 && observed.Level > 0 && expected.Level != observed.Level {
		return false
	}
	return strings.Contains(expected.Realm, observed.Realm) || strings.Contains(observed.Realm, expected.Realm)
}

func compareRPGResource(comparison *RPGStateComparison, key string, expected, observed *models.RPGResourceState) {
	if expected == nil && observed == nil {
		return
	}
	if expected == nil {
		if observed.Owner == "" && observed.Quantity == 0 {
			return
		}
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "info",
			Kind:     "extra_resource_state",
			Target:   key,
			Observed: summarizeRPGResource(observed),
			Note:     "observed DSL introduced resource state not present in outline expectation",
		})
		return
	}
	if observed == nil {
		if expected.Owner == "" && expected.Quantity == 0 {
			return
		}
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "missing_resource_state",
			Target:   key,
			Expected: summarizeRPGResource(expected),
		})
		return
	}
	if expected.Owner != "" && observed.Owner != "" && expected.Owner != observed.Owner {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     "resource_owner_mismatch",
			Target:   key,
			Field:    "owner",
			Expected: expected.Owner,
			Observed: observed.Owner,
		})
	}
	if expected.Quantity != 0 && observed.Quantity != 0 && expected.Quantity != observed.Quantity {
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "info",
			Kind:     "resource_quantity_mismatch",
			Target:   key,
			Field:    "quantity",
			Expected: fmt.Sprintf("%d", expected.Quantity),
			Observed: fmt.Sprintf("%d", observed.Quantity),
		})
	}
}

func compareStringMap(comparison *RPGStateComparison, kind, target string, expected, observed map[string]string) {
	for _, key := range sortedStringMapUnion(expected, observed) {
		exp := expected[key]
		obs := observed[key]
		if exp == "" || obs == "" || exp == obs {
			continue
		}
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "warning",
			Kind:     kind,
			Target:   target,
			Field:    key,
			Expected: exp,
			Observed: obs,
		})
	}
}

func compareIntMap(comparison *RPGStateComparison, kind, target string, expected, observed map[string]int) {
	for _, key := range sortedIntMapUnion(expected, observed) {
		exp, hasExp := expected[key]
		obs, hasObs := observed[key]
		if !hasExp || !hasObs || exp == obs {
			continue
		}
		comparison.Drifts = append(comparison.Drifts, RPGStateDrift{
			Severity: "info",
			Kind:     kind,
			Target:   target,
			Field:    key,
			Expected: fmt.Sprintf("%d", exp),
			Observed: fmt.Sprintf("%d", obs),
		})
	}
}

func relevantRPGCompareCharacters(expected, observed *models.RPGState, chapter *models.Chapter) []string {
	seen := map[string]bool{}
	if chapter != nil {
		for _, name := range chapter.Characters {
			seen[name] = true
		}
		for _, event := range chapter.Events {
			if actor := event.GetActor(); actor != "" {
				seen[actor] = true
			}
			if target := event.GetTarget(); target != "" && event.GetTargetType() == models.TargetTypeCharacter {
				seen[target] = true
			}
			for _, name := range event.Characters {
				seen[name] = true
			}
		}
	}
	if len(seen) == 0 {
		for name := range expected.Characters {
			seen[name] = true
		}
		for name := range observed.Characters {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedCompareResourceKeys(expected, observed map[string]*models.RPGResourceState) []string {
	seen := map[string]bool{}
	for key, value := range expected {
		if value != nil && (value.Owner != "" || value.Quantity != 0 || value.Status != "") {
			seen[key] = true
		}
	}
	for key, value := range observed {
		if value != nil && (value.Owner != "" || value.Quantity != 0 || value.Status != "") {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringMapUnion(a, b map[string]string) []string {
	seen := map[string]bool{}
	for key := range a {
		seen[key] = true
	}
	for key := range b {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntMapUnion(a, b map[string]int) []string {
	seen := map[string]bool{}
	for key := range a {
		seen[key] = true
	}
	for key := range b {
		seen[key] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func summarizeRPGCharacter(char *models.RPGCharacterState) string {
	if char == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("alive=%t", char.Alive)}
	if char.Realm != "" {
		parts = append(parts, "realm="+char.Realm)
	}
	if char.Level > 0 {
		parts = append(parts, fmt.Sprintf("level=%d", char.Level))
	}
	return strings.Join(parts, " ")
}

func summarizeRPGResource(resource *models.RPGResourceState) string {
	if resource == nil {
		return ""
	}
	return fmt.Sprintf("owner=%s quantity=%d status=%s", resource.Owner, resource.Quantity, resource.Status)
}
