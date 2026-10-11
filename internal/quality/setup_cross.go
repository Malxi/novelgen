package quality

import (
	"fmt"
	"novelgen/internal/models"
	"novelgen/internal/rpg"
	"strings"
)

func ValidateSetupOutlineCross(setup *models.StorySetup, outline *rpg.StoryOutline) (issues, warnings []string) {
	// Collect defined faction tiers from setup premises
	setupFactions := make(map[string]map[string]bool) // faction -> tier -> true
	for _, p := range setup.Premises {
		for _, faction := range setupPremiseFactionAliases(p) {
			if setupFactions[faction] == nil {
				setupFactions[faction] = make(map[string]bool)
			}
			for tier := range setupPremiseTierAliases(p, faction) {
				setupFactions[faction][tier] = true
			}
		}
	}

	// Collect defined resources from setup
	setupResources := make(map[string]bool)
	for _, r := range setup.WorldResources {
		setupResources[r.Name] = true
	}

	// Check outline enemies against setup faction definitions
	for _, part := range outline.Parts {
		for _, vol := range part.Volumes {
			for _, ch := range vol.Chapters {
				for _, enemy := range ch.Enemies {
					if enemy.Faction != "" && len(setupFactions) > 0 {
						if tiers, ok := setupFactions[enemy.Faction]; ok {
							if enemy.Tier != "" && !tiers[enemy.Tier] {
								issues = append(issues, fmt.Sprintf(
									"%s: enemy '%s' tier '%s' not defined in setup faction '%s'",
									ch.ID, enemy.Name, enemy.Tier, enemy.Faction))
							}
						} else {
							warnings = append(warnings, fmt.Sprintf(
								"%s: enemy faction '%s' not defined in setup premises",
								ch.ID, enemy.Faction))
						}
					}
				}
				for _, entry := range ch.ResourceLedger {
					if entry.Item != "" && len(setupResources) > 0 {
						if !setupResources[entry.Item] {
							// Find closest match in setup to suggest
							closest := findClosestResource(entry.Item, setupResources)
							hint := fmt.Sprintf("%s: resource '%s' not declared in setup.world_resources",
								ch.ID, entry.Item)
							if closest != "" {
								hint += fmt.Sprintf(" (setup has: '%s' — consider using that name)", closest)
							}
							warnings = append(warnings, hint)
						}
					}
				}
			}
		}
	}
	return
}

func findClosestResource(target string, candidates map[string]bool) string {
	// Check if any candidate is a substring of target or vice versa
	targetLower := strings.ToLower(target)
	for name := range candidates {
		nameLower := strings.ToLower(name)
		// Simple shared-character heuristic: if they share >50% of characters
		shared := 0
		for _, r := range targetLower {
			if strings.ContainsRune(nameLower, r) {
				shared++
			}
		}
		if float64(shared)/float64(len([]rune(targetLower))) > 0.4 {
			return name
		}
	}
	return ""
}

func setupFactionAliases(category string) []string {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil
	}
	seen := map[string]bool{}
	var aliases []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		aliases = append(aliases, value)
	}
	add(category)
	for _, part := range strings.FieldsFunc(category, func(r rune) bool {
		return r == '/' || r == '\\' || r == '|' || r == ',' || r == '，' || r == ';' || r == '；' || r == ' '
	}) {
		add(part)
	}
	return aliases
}

func setupPremiseFactionAliases(p models.Premise) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		aliases = append(aliases, value)
	}
	for _, alias := range setupFactionAliases(p.Category) {
		add(alias)
	}
	text := strings.ToLower(p.Name + " " + p.Description + " " + p.Category)
	for _, known := range []string{"zerg", "shen"} {
		if strings.Contains(text, known) {
			add(known)
		}
	}
	return aliases
}

func setupPremiseTierAliases(p models.Premise, faction string) map[string]bool {
	tiers := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			tiers[value] = true
		}
	}

	for _, stage := range p.Progression {
		add(stage.Name)
		text := stage.Name + " " + stage.Description + " " + stage.Requirements
		switch {
		case strings.Contains(text, "工虫"):
			add("drone")
		case strings.Contains(text, "兵虫"):
			add("soldier")
		case strings.Contains(text, "初级虫将"):
			add("captain")
		case strings.Contains(text, "高级虫统领"):
			add("commander")
		case strings.Contains(text, "母巢"):
			add("queen")
			add("hive")
		}
		if strings.Contains(text, "机甲") {
			add("mech")
		}
	}

	switch strings.ToLower(strings.TrimSpace(faction)) {
	case "zerg":
		add("drone")
		add("soldier")
		add("captain")
		add("commander")
		add("queen")
		add("hive")
	case "shen":
		add("agent")
		add("informant")
		add("soldier")
		add("mech")
	}

	return tiers
}
