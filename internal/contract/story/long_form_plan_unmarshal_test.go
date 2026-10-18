package models

import (
	"encoding/json"
	"testing"
)

func TestLongFormPlanUnmarshalAcceptsWeakModelScalarShapes(t *testing.T) {
	raw := `{
		"target_chapters": "300章",
		"target_volumes": "12",
		"main_loop": "pressure -> win -> bigger game",
		"escalation_ladder": "个人\n城市\n文明",
		"reader_promises": "爽点一,爽点二；爽点三",
		"payoff_cadence": "每卷一次",
		"volume_pattern": ["hook", "pressure", "payoff"],
		"midpoint_mutation": "盟友变成债主",
		"endgame_promise": "夺回时间"
	}`
	var plan LongFormPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if plan.TargetChapters != 300 || plan.TargetVolumes != 12 {
		t.Fatalf("targets = %d/%d", plan.TargetChapters, plan.TargetVolumes)
	}
	if len(plan.EscalationLadder) != 3 || plan.EscalationLadder[2] != "文明" {
		t.Fatalf("escalation = %#v", plan.EscalationLadder)
	}
	if len(plan.ReaderPromises) != 3 || plan.ReaderPromises[1] != "爽点二" {
		t.Fatalf("promises = %#v", plan.ReaderPromises)
	}
}
