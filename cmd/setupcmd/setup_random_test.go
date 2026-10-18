package setupcmd

import (
	"reflect"
	"testing"
)

func TestDrawSetupProvocationIsDeterministic(t *testing.T) {
	first, err := drawSetupProvocation(true, 4217, "axiom", "wild", "仙侠")
	if err != nil {
		t.Fatalf("drawSetupProvocation: %v", err)
	}
	second, err := drawSetupProvocation(true, 4217, "axiom", "wild", "仙侠")
	if err != nil {
		t.Fatalf("drawSetupProvocation: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different provocations:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first == nil || first.Sentence == "" || first.RequiredRole == "" || len(first.MustAnswer) == 0 {
		t.Fatalf("incomplete provocation: %#v", first)
	}
}

func TestDrawSetupProvocationDisabled(t *testing.T) {
	got, err := drawSetupProvocation(false, 4217, "axiom", "wild", "仙侠")
	if err != nil {
		t.Fatalf("drawSetupProvocation: %v", err)
	}
	if got != nil {
		t.Fatalf("disabled draw returned %#v", got)
	}
}

func TestDrawSetupScenarioIsDeterministic(t *testing.T) {
	first, err := drawSetupScenario(true, 12, "科幻", 1)
	if err != nil {
		t.Fatalf("drawSetupScenario: %v", err)
	}
	second, err := drawSetupScenario(true, 12, "科幻", 1)
	if err != nil {
		t.Fatalf("drawSetupScenario: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different scenarios:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first == nil {
		t.Fatal("expected a scenario card")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("invalid scenario: %v", err)
	}
}

func TestDrawSetupScenarioDisabled(t *testing.T) {
	got, err := drawSetupScenario(false, 12, "科幻", 1)
	if err != nil {
		t.Fatalf("drawSetupScenario: %v", err)
	}
	if got != nil {
		t.Fatalf("disabled draw returned %#v", got)
	}
}

func TestDrawSetupScenarioIndexSelectsCard(t *testing.T) {
	first, err := drawSetupScenario(true, 12, "科幻", 1)
	if err != nil {
		t.Fatalf("drawSetupScenario: %v", err)
	}
	second, err := drawSetupScenario(true, 12, "科幻", 2)
	if err != nil {
		t.Fatalf("drawSetupScenario: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("index did not select a different card: %s", first.ID)
	}
}
