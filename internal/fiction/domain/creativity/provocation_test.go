package creativity

import (
	"reflect"
	"strings"
	"testing"
)

func TestGenerateIsDeterministic(t *testing.T) {
	req := Request{
		Level: LevelAxiom,
		Genre: "仙侠",
		Mode:  ModeWild,
		Seed:  42,
		Count: 6,
	}
	first, err := Generate(req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	second, err := Generate(req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different results:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestGenerateProducesUniqueProvocationsWithContracts(t *testing.T) {
	result, err := Generate(Request{
		Level: LevelEvent,
		Genre: "都市",
		Mode:  ModeBalanced,
		Seed:  7,
		Count: 8,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.Provocations) != 8 {
		t.Fatalf("provocation count = %d, want 8", len(result.Provocations))
	}
	seen := map[string]bool{}
	for _, item := range result.Provocations {
		if strings.TrimSpace(item.Sentence) == "" {
			t.Fatal("empty sentence")
		}
		if seen[item.Sentence] {
			t.Fatalf("duplicate sentence %q", item.Sentence)
		}
		seen[item.Sentence] = true
		if item.RequiredRole == "" || len(item.MustAnswer) == 0 || len(item.OutputContract) == 0 {
			t.Fatalf("incomplete contract: %#v", item)
		}
		if item.AxiomBudget != 1 || len(item.DerivationSteps) == 0 || len(item.RejectIf) == 0 {
			t.Fatalf("incomplete causal contract: %#v", item)
		}
		if item.Level != LevelEvent || item.Genre != "都市" || item.Mode != ModeBalanced {
			t.Fatalf("unexpected metadata: %#v", item)
		}
	}
}

func TestGenerateLevels(t *testing.T) {
	for _, level := range []string{LevelAxiom, LevelEvent, LevelLaw, LevelName} {
		result, err := Generate(Request{Level: level, Seed: 11, Count: 2})
		if err != nil {
			t.Fatalf("Generate(%s): %v", level, err)
		}
		for _, item := range result.Provocations {
			if item.Level != level {
				t.Fatalf("item level = %q, want %q", item.Level, level)
			}
		}
	}
}

func TestGenerateSeedZeroReturnsReplayableSeed(t *testing.T) {
	result, err := Generate(Request{Level: LevelAxiom, Count: 1})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Seed == 0 {
		t.Fatal("generated seed must be non-zero")
	}
	replay, err := Generate(Request{Level: LevelAxiom, Seed: result.Seed, Count: 1})
	if err != nil {
		t.Fatalf("Generate replay: %v", err)
	}
	if !reflect.DeepEqual(result.Provocations, replay.Provocations) {
		t.Fatal("returned seed did not replay the same provocation")
	}
}

func TestGenerateRejectsUnknownModeAndLevel(t *testing.T) {
	if _, err := Generate(Request{Level: "nonsense"}); err == nil {
		t.Fatal("expected unknown level error")
	}
	if _, err := Generate(Request{Level: LevelAxiom, Mode: "chaos"}); err == nil {
		t.Fatal("expected unknown mode error")
	}
}

func TestGenerateCapsCount(t *testing.T) {
	result, err := Generate(Request{Level: LevelAxiom, Seed: 3, Count: maxProvocationCount + 5})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.Provocations) != maxProvocationCount {
		t.Fatalf("count = %d, want %d", len(result.Provocations), maxProvocationCount)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a count-cap warning")
	}
}

func TestGenerateGroundedLawUsesHardSFConstraints(t *testing.T) {
	result, err := Generate(Request{
		Level: LevelLaw,
		Genre: "科幻",
		Mode:  ModeGrounded,
		Seed:  12,
		Count: 8,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(result.Provocations) != 8 {
		t.Fatalf("count = %d, want 8", len(result.Provocations))
	}
	for _, item := range result.Provocations {
		if strings.Contains(item.Sentence, "某物") {
			t.Fatalf("grounded law contains unfilled slot: %q", item.Sentence)
		}
		if item.AxiomBudget != 1 || len(item.DerivationSteps) == 0 {
			t.Fatalf("missing contract: %#v", item)
		}
	}
}

func TestGroundedGenerationIncludesCausalChains(t *testing.T) {
	result, err := Generate(Request{
		Level: LevelLaw,
		Genre: "科幻",
		Mode:  ModeGrounded,
		Seed:  303,
		Count: 20,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	found := false
	for _, item := range result.Provocations {
		if strings.HasPrefix(item.TemplateID, "grounded_chain_") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("grounded generation did not produce any causal-chain provocation")
	}
}

func TestValidSlotCombinationRejectsSemanticDefects(t *testing.T) {
	cases := []struct {
		name     string
		template sentenceTemplate
		slots    map[string]string
		want     bool
	}{
		{
			name:     "tautological payment",
			template: sentenceTemplate{ID: "law_price"},
			slots:    map[string]string{"law_thing": "身份", "cost": "一份身份"},
			want:     false,
		},
		{
			name:     "oxygen does not decay",
			template: sentenceTemplate{ID: "grounded_fuel"},
			slots:    map[string]string{"fuel_resource": "氧气", "decay_verb": "衰变", "vehicle": "打捞船", "grounded_time": "日落", "source": "补给站"},
			want:     false,
		},
		{
			name:     "silent running does not stop a wormhole tide",
			template: sentenceTemplate{ID: "grounded_deadline"},
			slots:    map[string]string{"vehicle": "登陆舱", "grounded_time": "第7天", "deadline_action": "切换静默航行", "hazard": "虫洞潮汐", "hazard_verb": "扭曲导航"},
			want:     false,
		},
		{
			name:     "silent running can hide from a heat-seeking swarm",
			template: sentenceTemplate{ID: "grounded_swarm"},
			slots:    map[string]string{"creature": "虫族工蜂", "signature": "反应堆热签名", "countermeasure": "静默航行", "suppress_verb": "避开"},
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validSlotCombination(LevelLaw, tc.template, tc.slots); got != tc.want {
				t.Fatalf("validSlotCombination = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestGenerateExploresDistinctTemplatesBeforeRepeating(t *testing.T) {
	result, err := Generate(Request{
		Level: LevelAxiom,
		Genre: "悬疑",
		Mode:  ModeWild,
		Seed:  99,
		Count: 10,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	seen := map[string]bool{}
	for _, item := range result.Provocations {
		if item.TemplateID == "" {
			t.Fatalf("missing template id: %#v", item)
		}
		if seen[item.TemplateID] {
			t.Fatalf("template %q repeated before all eligible templates were used", item.TemplateID)
		}
		seen[item.TemplateID] = true
	}
}

func TestGenerateEventSentencesStayGrammaticalWhenGiving(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		result, err := Generate(Request{
			Level: LevelEvent,
			Genre: "都市",
			Mode:  ModeWild,
			Seed:  seed,
			Count: 5,
		})
		if err != nil {
			t.Fatalf("Generate(seed=%d): %v", seed, err)
		}
		for _, item := range result.Provocations {
			for _, malformed := range []string{"给给", "进给", "交给给", "寄给给", "藏进给了"} {
				if strings.Contains(item.Sentence, malformed) {
					t.Fatalf("malformed give construction in seed %d: %q", seed, item.Sentence)
				}
			}
			if item.TemplateID != "event_give" {
				for _, recipientVerb := range []string{"交给", "寄给", "快递", "藏进"} {
					if strings.Contains(item.Sentence, recipientVerb) {
						t.Fatalf("recipient verb %q leaked into %s: %q", recipientVerb, item.TemplateID, item.Sentence)
					}
				}
			}
		}
	}
}
