package creativity

import (
	"reflect"
	"strings"
	"testing"
)

func TestGenerateEnginesIsDeterministic(t *testing.T) {
	req := EngineRequest{Genre: "科幻", Seed: 42, Count: 6}
	first, err := GenerateEngines(req)
	if err != nil {
		t.Fatalf("GenerateEngines: %v", err)
	}
	second, err := GenerateEngines(req)
	if err != nil {
		t.Fatalf("GenerateEngines: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different engines:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if len(first.Engines) != 6 {
		t.Fatalf("engine count = %d, want 6", len(first.Engines))
	}
}

func TestGenerateEnginesProducesValidCards(t *testing.T) {
	result, err := GenerateEngines(EngineRequest{Genre: "科幻", Seed: 7, Count: 12})
	if err != nil {
		t.Fatalf("GenerateEngines: %v", err)
	}
	seen := map[string]bool{}
	for _, engine := range result.Engines {
		if issues := ValidateEngine(engine); len(issues) > 0 {
			t.Fatalf("invalid engine %#v: %v", engine, issues)
		}
		if seen[engine.Artifact+"|"+engine.HiddenFunction] {
			t.Fatalf("duplicate engine card: %s", engine.Artifact)
		}
		seen[engine.Artifact+"|"+engine.HiddenFunction] = true
		for _, value := range []string{engine.Artifact, engine.Issuer, engine.Holder, engine.ProtagonistRole} {
			if strings.Contains(value, "某物") || strings.Contains(value, "%s") || strings.Contains(value, "{") {
				t.Fatalf("unfilled slot in %q", value)
			}
		}
		if engine.PremiseScaffold == "" || len(engine.RuleSeeds) < 3 ||
			len(engine.StorylineSeeds) < 3 || len(engine.CastSeeds) < 4 {
			t.Fatalf("incomplete scaffold: %#v", engine)
		}
	}
}

func TestGenerateEnginesExploresFamiliesBeforeRepeating(t *testing.T) {
	result, err := GenerateEngines(EngineRequest{Genre: "都市", Seed: 99, Count: 8})
	if err != nil {
		t.Fatalf("GenerateEngines: %v", err)
	}
	seen := map[string]bool{}
	for _, engine := range result.Engines {
		if seen[engine.Family] {
			t.Fatalf("family %q repeated before all families were used", engine.Family)
		}
		seen[engine.Family] = true
	}
}

func TestValidateEngineRejectsMissingCausalSpine(t *testing.T) {
	bad := Engine{
		Artifact:           "保险单",
		Issuer:             "保险公司",
		Holder:             "舰长",
		StatedPurpose:      "承保损失",
		HiddenFunction:     "承保损失",
		CounterExpectation: "主角夺冠",
		DefaultExpectation: "主角夺冠",
		ConflictTypes:      []string{"资源", "资源"},
	}
	issues := ValidateEngine(bad)
	for _, want := range []string{
		"hidden_function must differ from stated_purpose",
		"need at least three distinct conflict types",
		"counter_expectation must differ from default_expectation",
	} {
		if !contains(issues, want) {
			t.Fatalf("missing issue %q in %#v", want, issues)
		}
	}
}

func TestGenerateEnginesQuotaArtifactIsNotDoubled(t *testing.T) {
	result, err := GenerateEngines(EngineRequest{Genre: "科幻", Seed: 3, Count: 12})
	if err != nil {
		t.Fatalf("GenerateEngines: %v", err)
	}
	for _, engine := range result.Engines {
		if strings.Contains(engine.Artifact, "配额配额") {
			t.Fatalf("doubled quota artifact: %q", engine.Artifact)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
