package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIEmbedderUsesConfiguredEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		var request embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "embed-model" || len(request.Input) != 2 {
			t.Fatalf("unexpected request: %#v", request)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"index": 1, "embedding": []float32{0, 1}},
				{"index": 0, "embedding": []float32{1, 0}},
			},
		})
	}))
	defer server.Close()

	embedder, err := NewOpenAIEmbedder(&ProviderConfig{
		Name: "test", APIKey: "token", BaseURL: server.URL + "/v1", Timeout: 5,
	}, "embed-model")
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := embedder.Embed(context.Background(), []string{"blue", "red"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
}
