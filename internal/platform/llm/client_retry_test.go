package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns a client pointed at the test server with real backoff
// replaced by an instant, recorded sleep.
func newTestClient(t *testing.T, baseURL string) (*OpenAIClient, *[]time.Duration) {
	t.Helper()
	client := NewOpenAIClient(&OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "test-model",
		Timeout: 5,
	})
	delays := &[]time.Duration{}
	client.retryBaseDelay = time.Millisecond
	client.sleep = func(_ context.Context, d time.Duration) error {
		*delays = append(*delays, d)
		return nil
	}
	return client, delays
}

func successResponse(content string) string {
	return fmt.Sprintf(`{"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%q}}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`, content)
}

func TestChatCompletionRetriesTransientStatus(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(successResponse("ok")))
	}))
	defer server.Close()

	client, delays := newTestClient(t, server.URL)
	resp, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
	if len(*delays) != 1 {
		t.Fatalf("delays = %v, want exactly one retry", *delays)
	}
}

func TestChatCompletionHonoursRetryAfter(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(successResponse("ok")))
	}))
	defer server.Close()

	client, delays := newTestClient(t, server.URL)
	if _, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil); err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if len(*delays) != 1 || (*delays)[0] != 2*time.Second {
		t.Fatalf("delays = %v, want [2s]", *delays)
	}
}

func TestChatCompletionDoesNotRetryClientErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client, delays := newTestClient(t, server.URL)
	_, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1 (400 must not be retried)", got)
	}
	if len(*delays) != 0 {
		t.Fatalf("delays = %v, want none", *delays)
	}
}

func TestChatCompletionGivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, _ := newTestClient(t, server.URL)
	client.maxAttempts = 3
	_, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
}

func TestChatCompletionStopsWhenContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := newTestClient(t, server.URL)
	client.maxAttempts = 5
	client.sleep = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ChatCompletion(ctx, []Message{{Role: "user", Content: "hi"}}, nil); err == nil {
		t.Fatalf("expected a cancellation error")
	}
}

func TestChatCompletionRetriesTransportErrors(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// Drop the connection without a response.
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("server does not support hijacking")
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(successResponse("recovered")))
	}))
	defer server.Close()

	client, _ := newTestClient(t, server.URL)
	resp, err := client.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if resp.Content != "recovered" {
		t.Fatalf("content = %q, want recovered", resp.Content)
	}
}
