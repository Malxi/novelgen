package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIEmbedder calls an OpenAI-compatible /embeddings endpoint. It is kept
// separate from Client so adding optional narrative-memory vectors does not
// break existing chat-only test doubles or Claude runtime clients.
type OpenAIEmbedder struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOpenAIEmbedder(provider *ProviderConfig, model string) (*OpenAIEmbedder, error) {
	if provider == nil {
		return nil, fmt.Errorf("embedding provider is nil")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("embedding model is not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(provider.EmbeddingBaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("embedding provider %q has no base_url", provider.Name)
	}
	timeout := provider.Timeout
	if timeout <= 0 {
		timeout = 120
	}
	return &OpenAIEmbedder{
		apiKey:  strings.TrimSpace(provider.APIKey),
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

func (e *OpenAIEmbedder) Model() string {
	if e == nil {
		return ""
	}
	return e.model
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e == nil {
		return nil, fmt.Errorf("embedding client is nil")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(embeddingRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	response, err := e.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding request failed with HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded embeddingResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response returned %d vectors for %d inputs", len(decoded.Data), len(texts))
	}
	vectors := make([][]float32, len(texts))
	useIndexes := true
	seenIndexes := map[int]bool{}
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(vectors) || seenIndexes[item.Index] {
			useIndexes = false
			break
		}
		seenIndexes[item.Index] = true
	}
	for position, item := range decoded.Data {
		index := position
		if useIndexes {
			index = item.Index
		}
		vectors[index] = item.Embedding
	}
	for i, vector := range vectors {
		if len(vector) == 0 {
			return nil, fmt.Errorf("embedding response index %d is empty", i)
		}
	}
	return vectors, nil
}
