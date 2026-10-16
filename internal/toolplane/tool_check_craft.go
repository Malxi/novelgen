// The craft target check: saved craft objects validated against the typed schema, name/metadata
// safety rules and cross-object consistency.

package toolplane

import (
	"fmt"
	"strings"

	"novelgen/internal/models"
	"novelgen/internal/quality"
	"novelgen/internal/store"
)

// RunToolCraftSchemaCheck is the string-root entry point for callers that hold a
// resolved project directory. It exists so the store-based implementation stays
// the only one that spells project paths.
func RunToolCraftSchemaCheck(rootDir, scope, id string) (*ToolCheckResult, error) {
	return runToolCraftSchemaCheckAt(store.At(rootDir), scope, id)
}

func runToolCraftSchemaCheckAt(root *store.Root, scope, id string) (*ToolCheckResult, error) {
	scope = normalizeCraftPatchTarget(scope)
	if scope == "" {
		return nil, fmt.Errorf("--scope is required for craft schema check (character/item/location/organization)")
	}
	filename, err := craftPatchFilename(scope)
	if err != nil {
		return nil, err
	}
	values, err := loadRawCraftMap(root, filename)
	if err != nil {
		return nil, err
	}
	gate := quality.QualityGateResult{}
	if values == nil {
		values = map[string]map[string]interface{}{}
	}
	knownCharacters := map[string]bool{}
	if scope == "item" {
		knownCharacters = loadCraftCharacterNames(root)
	}
	if strings.TrimSpace(id) != "" {
		key, ok := resolveCraftPatchKey(values, id)
		if !ok {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "schema",
				TargetID:   id,
				TargetName: id,
				Issue:      "craft element not found",
				Suggestion: fmt.Sprintf("Create or patch craft %s %q before checking it.", scope, id),
				Priority:   models.PriorityHigh,
			})
			gate.Blocking = true
			return MakeToolCheckResult("schema", "craft", scope, id, gate), nil
		}
		validateCraftSchemaObject(&gate, scope, key, values[key])
		validateCraftConsistencyObject(&gate, scope, key, values[key], knownCharacters)
		validateCraftQualityObject(&gate, scope, key, values[key])
		gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
		return MakeToolCheckResult("schema", "craft", scope, key, gate), nil
	}
	for key, value := range values {
		validateCraftSchemaObject(&gate, scope, key, value)
		validateCraftConsistencyObject(&gate, scope, key, value, knownCharacters)
		validateCraftQualityObject(&gate, scope, key, value)
	}
	gate.Blocking = quality.HasBlockingSuggestions(gate.Suggestions)
	return MakeToolCheckResult("schema", "craft", scope, "", gate), nil
}

func validateCraftSchemaObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	if gate == nil {
		return
	}
	if _, err := NormalizeCraftPatchObject(scope, key, copyStringInterfaceMap(value)); err != nil {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "schema",
			TargetID:   key,
			TargetName: key,
			Issue:      fmt.Sprintf("craft %s does not match schema: %v", scope, err),
			Suggestion: fmt.Sprintf("Patch craft %s %q with fields that match the typed craft schema, then rerun tool check schema.", scope, key),
			Priority:   models.PriorityHigh,
		})
	}
}

func loadCraftCharacterNames(root *store.Root) map[string]bool {
	names := map[string]bool{}
	values, err := loadRawCraftMap(root, "characters.json")
	if err != nil {
		return names
	}
	for key, value := range values {
		addCraftKnownName(names, key)
		addCraftKnownName(names, StringMapValue(value, "name"))
	}
	return names
}

func validateCraftConsistencyObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}, knownCharacters map[string]bool) {
	if gate == nil || normalizeCraftPatchTarget(scope) != "item" || len(knownCharacters) == 0 {
		return
	}
	owner := strings.TrimSpace(StringMapValue(value, "owner"))
	if owner == "" || isGenericCraftOwner(owner) || knownCharacters[normalizeCraftLookupName(owner)] {
		return
	}
	gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
		Category:   "consistency",
		TargetID:   key,
		TargetName: key,
		Issue:      fmt.Sprintf("item owner %q is not present in craft characters", owner),
		Suggestion: "Patch the item owner to an existing character name, a stable generic owner such as 主角, or leave owner empty when ownership is not established.",
		Priority:   models.PriorityMedium,
	})
}

func validateCraftQualityObject(gate *quality.QualityGateResult, scope, key string, value map[string]interface{}) {
	if gate == nil || normalizeCraftPatchTarget(scope) != "character" || value == nil {
		return
	}
	isLead := isLeadCraftCharacter(value)
	personalityCount := craftStringListCount(value["personality"])
	skillsCount := craftStringListCount(value["skills"])
	abilitiesCount := craftStringListCount(value["abilities"])
	motivation := StringMapValue(value, "motivation")
	voice := StringMapValue(value, "voice")
	notes := StringMapValue(value, "notes")
	notesLen := len([]rune(notes))

	if isLead {
		if personalityCount < 3 {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft has too few structured personality traits",
				Suggestion: "Patch personality with 3-6 concrete traits that affect decisions, dialogue, and failure modes.",
				Priority:   models.PriorityMedium,
			})
		}
		if motivation == "" {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft is missing a structured motivation",
				Suggestion: "Patch motivation with the character's durable desire, fear, and story-facing pressure in one concise paragraph.",
				Priority:   models.PriorityMedium,
			})
		}
		if skillsCount+abilitiesCount == 0 {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft has no structured skills or abilities",
				Suggestion: "Patch skills for mundane/tactical competencies and abilities for special powers, including limits or blind spots.",
				Priority:   models.PriorityMedium,
			})
		}
		if voice == "" {
			gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
				Category:   "craft_quality",
				TargetID:   key,
				TargetName: key,
				Issue:      "protagonist craft is missing voice guidance",
				Suggestion: "Patch voice with speech rhythm, humor/seriousness balance, and recurring verbal habits useful for drafting.",
				Priority:   models.PriorityLow,
			})
		}
	}
	if notesLen > 1200 && isLead {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "craft_quality",
			TargetID:   key,
			TargetName: key,
			Issue:      "protagonist craft concentrates too much writable structure in notes",
			Suggestion: "Move reusable facts from notes into personality, motivation, skills, abilities, and voice; keep notes for concise writer-only constraints.",
			Priority:   models.PriorityLow,
		})
	} else if notesLen > 1800 {
		gate.Suggestions = append(gate.Suggestions, models.ReviewSuggestion{
			Category:   "craft_quality",
			TargetID:   key,
			TargetName: key,
			Issue:      "craft notes are too long for stable focused prompting",
			Suggestion: "Shorten notes and move repeated structured facts into typed craft fields.",
			Priority:   models.PriorityLow,
		})
	}
}

func isLeadCraftCharacter(value map[string]interface{}) bool {
	role := strings.ToLower(StringMapValue(value, "role_in_story"))
	rpgRole := strings.ToLower(StringMapValue(value, "rpg_role"))
	if strings.Contains(role, "protagonist") || strings.Contains(role, "lead") || strings.Contains(role, "主角") || rpgRole == "player" {
		return true
	}
	for _, tag := range craftStringListValues(value["dsl_tags"]) {
		tag = strings.ToLower(tag)
		if tag == "protagonist" || tag == "player" || tag == "lead" {
			return true
		}
	}
	return false
}

func craftStringListCount(raw interface{}) int {
	return len(craftStringListValues(raw))
}

func craftStringListValues(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				out = append(out, strings.TrimSpace(value))
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func addCraftKnownName(names map[string]bool, name string) {
	name = normalizeCraftLookupName(name)
	if name != "" {
		names[name] = true
	}
}

func normalizeCraftLookupName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isGenericCraftOwner(owner string) bool {
	switch normalizeCraftLookupName(owner) {
	case "主角", "protagonist", "unknown", "未知", "无人", "none", "n/a":
		return true
	default:
		return false
	}
}
