package setupcmd

import (
	"encoding/json"
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/creativity"
	"novelgen/internal/platform/store"
)

func TestScenarioCoverageMissing(t *testing.T) {
	scenario := &creativity.ScenarioCard{
		Hazard: "引力剪切",
		ElementRoles: map[string]string{
			"机甲": "机甲负责在虫巢外围建立临时信标",
		},
	}
	setup := &models.StorySetup{
		Premise: "虫洞边缘的舰队战争。",
		Rules:   []string{"引力剪切每49小时发生。"},
	}
	missing := scenarioCoverageMissing(setup, scenario)
	for _, want := range []string{"机甲", "虫族"} {
		if !containsString(missing, want) {
			t.Fatalf("missing = %#v, want %s", missing, want)
		}
	}
	if containsString(missing, "虫洞") {
		t.Fatalf("虫洞 should be covered: %#v", missing)
	}
}

func TestScenarioCoverageComplete(t *testing.T) {
	scenario := &creativity.ScenarioCard{Hazard: "引力剪切"}
	setup := &models.StorySetup{
		Premise: "虫洞边缘的星战、战舰与舰队战争。",
		Rules:   []string{"引力剪切每49小时发生，机甲负责救援，虫族会被热签名吸引。"},
	}
	if missing := scenarioCoverageMissing(setup, scenario); len(missing) != 0 {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestScenarioCoverageMechaInProgressionOnlyIsMissing(t *testing.T) {
	scenario := &creativity.ScenarioCard{Hazard: "引力剪切"}
	setup := &models.StorySetup{
		Premise: "虫洞边缘的星战与舰队战争。",
		Rules:   []string{"引力剪切每49小时发生，虫族会被热签名吸引。"},
		Premises: []models.Premise{{
			Name:        "剪切机制",
			Description: "战舰必须规避。",
			Progression: []models.ProgressionStage{{
				Level:       1,
				Name:        "信标阶段",
				Description: "建立临时信标（机甲）。",
			}},
		}},
	}
	missing := scenarioCoverageMissing(setup, scenario)
	if !containsString(missing, "机甲") {
		t.Fatalf("mecha mentioned only in progression should be missing: %#v", missing)
	}
}

func TestWriteScenarioPlan(t *testing.T) {
	root := store.MemRoot()
	plan := creativity.ScenarioPlan{
		Seed:  12,
		Genre: "科幻",
		Selected: []creativity.ScenarioPlanItem{{
			Card:           creativity.ScenarioCard{ID: "scenario-12-01", PremiseKernel: "测试卡"},
			Family:         "family-a",
			Novelty:        90,
			Predictability: 10,
		}},
	}
	path, err := writeScenarioPlan(root, 12, plan)
	if err != nil {
		t.Fatalf("writeScenarioPlan: %v", err)
	}
	data, err := root.Read(path)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	var loaded creativity.ScenarioPlan
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if loaded.Seed != 12 || len(loaded.Selected) != 1 || loaded.Selected[0].Family != "family-a" {
		t.Fatalf("loaded plan = %#v", loaded)
	}
}
