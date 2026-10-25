// Normalizers for the tool plane: alias-to-canonical key/type mapping, mojibake repair for
// agent-supplied lookup text, and result limit helpers.

package toolplane

import (
	"strings"

	"novelgen/internal/platform/utils"
)

func StringMapValue(value map[string]interface{}, key string) string {
	return utils.StringMapValue(value, key)
}

func RepairToolLookupText(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	repaired := utils.RepairLikelyMojibakeText(trimmed)
	return repaired, repaired != trimmed
}

func NormalizeKey(value string) string {
	return utils.NormalizeKey(value)
}

func normalizeContextType(value string) string {
	switch NormalizeKey(value) {
	case "", "craft-character", "character-craft", "character", "char":
		return "craft-character"
	case "craft-item", "item-craft", "item":
		return "craft-item"
	case "craft-location", "location-craft", "location", "loc":
		return "craft-location"
	case "craft-organization", "organization-craft", "organization", "org":
		return "craft-organization"
	case "outline-volume", "volume-outline", "volume", "compose-volume":
		return "outline-volume"
	case "outline-repair", "repair-outline", "repair", "issue", "issue-repair":
		return "outline-repair"
	case "outline-global-repair", "global-outline-repair", "outline-global", "global-repair":
		return "outline-global-repair"
	case "recap-repair", "repair-recap", "recap", "recap-context":
		return "recap-repair"
	case "chapter-repair", "repair-chapter", "final-chapter-repair", "chapter-issue", "chapter-check-repair":
		return "chapter-repair"
	case "chapter-write", "write-chapter", "chapter-draft", "draft-chapter", "write", "chapter-context":
		return "chapter-write"
	default:
		return NormalizeKey(value)
	}
}

func normalizeEntityType(value string) string {
	switch NormalizeKey(value) {
	case "characters", "char":
		return "character"
	case "items":
		return "item"
	case "locations", "loc":
		return "location"
	case "storylines":
		return "storyline"
	default:
		return NormalizeKey(value)
	}
}

func limitOutlineHitResults(results interface{}, max int) []OutlineHit {
	values, _ := results.([]OutlineHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func limitChapterHitResults(results interface{}, max int) []chapterHit {
	values, _ := results.([]chapterHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func limitEventHitResults(results interface{}, max int) []EventHit {
	values, _ := results.([]EventHit)
	if max > 0 && len(values) > max {
		return values[:max]
	}
	return values
}

func normalizeSetupType(value string) string {
	switch NormalizeKey(value) {
	case "storylines", "arc", "arcs":
		return "storyline"
	case "premises":
		return "premise"
	case "cast", "core_cast", "corecast":
		return "core-cast"
	case "resources", "world-resource", "world-resources":
		return "resource"
	case "world-timeline":
		return "timeline"
	case "rule", "rules", "setup-rules", "iron-rules":
		return "rules"
	case "longform", "long_form_plan", "long-form":
		return "long-form-plan"
	default:
		return NormalizeKey(value)
	}
}

func normalizeOutlineType(value string) string {
	switch NormalizeKey(value) {
	case "parts":
		return "part"
	case "volumes":
		return "volume"
	case "chapters", "outline-chapter", "outline-chapters":
		return "chapter"
	case "reference", "references":
		return "refs"
	default:
		return NormalizeKey(value)
	}
}

func normalizeCraftType(value string) string {
	switch NormalizeKey(value) {
	case "characters", "char", "chars":
		return "character"
	case "items":
		return "item"
	case "locations", "loc", "locs":
		return "location"
	case "organizations", "org", "orgs":
		return "organization"
	default:
		return NormalizeKey(value)
	}
}

func normalizeChapterType(value string) string {
	switch NormalizeKey(value) {
	case "reference", "references":
		return "refs"
	case "event":
		return "events"
	default:
		return NormalizeKey(value)
	}
}

func prefixedReasons(prefix string, reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, prefix+"."+reason)
	}
	return out
}

// appendToolQueryFlagWarnings keeps filters from being silently ignored. A
// model that passes --name to `outline --type all` should be told to use
// refs/events instead of receiving the entire outline as if the filter ran.
func appendToolQueryFlagWarnings(resp *ToolResponse, section, queryType, id, name, chapterID, volumeID string) {
	if resp == nil {
		return
	}
	add := func(message string) {
		resp.Warnings = append(resp.Warnings, message)
	}
	contentUsed := section == "logs" || (section == "chapter" && queryType != "events" && queryType != "refs")
	if ToolQueryFlags.IncludeContent && !contentUsed {
		add("--content is only used by chapter records and logs and was ignored")
	}
	categoryUsed := section == "ideas" && queryType == "itemcard"
	if strings.TrimSpace(ToolQueryFlags.Category) != "" && !categoryUsed {
		add("--category is only used by itemcard idea queries and was ignored")
	}
	entityUsed := (section == "outline" && (queryType == "refs" || queryType == "events")) ||
		(section == "chapter" && (queryType == "refs" || queryType == "events" ||
			(strings.TrimSpace(id) == "" && (queryType == "" || queryType == "all"))))
	if strings.TrimSpace(ToolQueryFlags.EntityType) != "" && !entityUsed {
		add("--entity-type is only used by outline/chapter refs or events queries and was ignored")
	}
	eventScopeUsed := (section == "outline" || section == "chapter") && queryType == "events"
	if strings.TrimSpace(chapterID) != "" && !eventScopeUsed {
		add("--chapter-id is only used by event queries and was ignored")
	}
	if strings.TrimSpace(volumeID) != "" && !eventScopeUsed {
		add("--volume-id is only used by event queries and was ignored")
	}
	if strings.TrimSpace(id) != "" {
		switch section {
		case "story-setup":
			add("--id is not used by story-setup queries and was ignored")
		case "craft":
			add("--id is not used by craft queries; use --name")
		case "outline":
			if queryType == "refs" || queryType == "events" {
				add("--id is ignored for outline refs/events; use --chapter-id/--volume-id or --entity-type/--name")
			}
		case "context":
			if queryType == "outline-global-repair" {
				add("--id is not used by outline-global-repair and was ignored")
			}
		}
	}
	if strings.TrimSpace(name) == "" {
		return
	}
	switch section {
	case "story-setup":
		if queryType == "" || queryType == "all" || queryType == "index" || queryType == "long-form-plan" {
			add("--name is ignored for this story-setup --type; use --type search or an object type (storyline/premise/core-cast/resource/timeline/rules)")
		}
	case "outline":
		if queryType != "refs" && queryType != "events" {
			add("--name is only used by outline refs/events queries and was ignored")
		}
	case "craft":
		if queryType == "" || queryType == "all" {
			add("--name is ignored for craft --type all; use --type character|item|location|organization")
		}
	case "chapter":
		if strings.TrimSpace(id) != "" && queryType != "events" {
			add("--name is ignored when --id selects a chapter record")
		}
	case "context":
		if (queryType == "outline-volume" && strings.TrimSpace(id) != "") || queryType == "recap-repair" || queryType == "chapter-write" {
			add("--name is ignored for this context type")
		}
	}
}
