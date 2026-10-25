package memory

import "testing"

func TestVectorFileRoundTripAndSearch(t *testing.T) {
	vectors := map[string][]float32{
		"scene:b": {0, 1, 0.2},
		"scene:a": {1, 0, 0.1},
	}
	data, err := encodeVectors(vectors)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeVectors(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || len(decoded["scene:a"]) != 3 || decoded["scene:b"][1] != 1 {
		t.Fatalf("unexpected decoded vectors: %#v", decoded)
	}
	hits := searchVectors([]float32{1, 0, 0.1}, decoded, map[string]bool{"scene:a": true, "scene:b": true}, 2)
	if len(hits) != 2 || hits[0].ID != "scene:a" {
		t.Fatalf("unexpected vector hits: %#v", hits)
	}
}
