package agents

import (
	"strings"
	"testing"

	"novelgen/internal/models"
)

func adjacentTestOutline() models.Outline {
	return models.Outline{
		Parts: []models.Part{
			{ID: "P1", Volumes: []models.Volume{{ID: "P1-V1"}, {ID: "P1-V2"}}},
			{ID: "P2", Volumes: []models.Volume{{ID: "P2-V1"}, {ID: "P2-V2"}}},
		},
	}
}

func TestComposeAdjacentVolumeIDs(t *testing.T) {
	outline := adjacentTestOutline()
	cases := map[string][]string{
		"P1-V1": {"P1-V2"},
		"P1-V2": {"P1-V1", "P2-V1"},
		"P2-V1": {"P1-V2", "P2-V2"},
		"P2-V2": {"P2-V1"},
		"P9-V9": nil,
	}
	for volumeID, want := range cases {
		got := composeAdjacentVolumeIDs(outline, volumeID)
		if len(got) != len(want) {
			t.Fatalf("composeAdjacentVolumeIDs(%s) = %#v, want %#v", volumeID, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("composeAdjacentVolumeIDs(%s) = %#v, want %#v", volumeID, got, want)
			}
		}
	}
}

func TestComposeAdjacentVolumeQueryAllowlist(t *testing.T) {
	outline := adjacentTestOutline()
	got := composeAdjacentVolumeQueryAllowlist(outline, "P1-V2")
	if len(got) != 2 {
		t.Fatalf("allowlist length = %d, want 2", len(got))
	}
	joined := strings.Join(got, "\n")
	for _, wantID := range []string{"P1-V1", "P2-V1"} {
		if !strings.Contains(joined, wantID) {
			t.Fatalf("allowlist missing adjacent volume %s: %s", wantID, joined)
		}
	}
	for _, entry := range got {
		if !strings.Contains(entry, "--fields payoff_contract,summary --view brief") {
			t.Fatalf("adjacent query should be a compact payoff/summary read: %s", entry)
		}
	}
	if extra := composeAdjacentVolumeQueryAllowlist(outline, "P9-V9"); len(extra) != 0 {
		t.Fatalf("unknown volume should produce no adjacent queries, got %#v", extra)
	}
}

func TestComposeAdjacentVolumeQueryAllowlistExcludesTarget(t *testing.T) {
	outline := adjacentTestOutline()
	got := composeAdjacentVolumeQueryAllowlist(outline, "P1-V2")
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, `--id "P1-V2"`) {
		t.Fatalf("target volume must not be included in adjacent allowlist: %s", joined)
	}
}
