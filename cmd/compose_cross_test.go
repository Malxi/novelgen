package cmd

import (
	"testing"

	"novelgen/internal/models"
	"novelgen/internal/rpg"
)

func TestValidateSetupOutlineCrossAcceptsFactionAliases(t *testing.T) {
	setup := &models.StorySetup{
		Premises: []models.Premise{
			{
				Name:     "虫族阶层与天敌体系",
				Category: "敌对势力体系/zerg",
				Progression: []models.ProgressionStage{
					{Name: "工虫层"},
					{Name: "作战兵虫层"},
					{Name: "初级虫将层"},
				},
			},
			{
				Name:     "沈氏简化机甲与技术垄断体系",
				Category: "技术科技体系/shen",
				Progression: []models.ProgressionStage{
					{Name: "组装调试"},
				},
			},
		},
	}
	outline := &rpg.StoryOutline{
		Parts: []rpg.StoryPart{{
			ID: "P1",
			Volumes: []rpg.StoryVolume{{
				ID: "P1-V1",
				Chapters: []rpg.StoryChapter{{
					ID: "P1-V1-C1",
					Enemies: []rpg.StoryOutlineEnemy{
						{Name: "工虫", Faction: "zerg", Tier: "drone", Count: 3},
						{Name: "沈氏线人", Faction: "shen", Tier: "informant", Count: 1},
						{Name: "简化机甲", Faction: "shen", Tier: "mech", Count: 1},
					},
				}},
			}},
		}},
	}

	issues, warnings := validateSetupOutlineCross(setup, outline)
	if len(issues) != 0 || len(warnings) != 0 {
		t.Fatalf("expected no cross warnings/issues, got issues=%v warnings=%v", issues, warnings)
	}
}
