package llm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"novelgen/internal/agentruntime"
	"novelgen/internal/logger"
)

// Client interface for LLM providers
type Client interface {
	ChatCompletion(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ThinkingMode represents the thinking mode for the model
type ThinkingMode string

const (
	ThinkingEnabled  ThinkingMode = "enabled"  // 强制开启深度思考
	ThinkingDisabled ThinkingMode = "disabled" // 强制关闭深度思考
	ThinkingAuto     ThinkingMode = "auto"     // 模型自行判断是否深度思考
)

// ChatOptions contains optional parameters for chat completion
type ChatOptions struct {
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Model       string       `json:"model,omitempty"`
	Thinking    ThinkingMode `json:"thinking,omitempty"` // enabled/disabled/auto
}

// ChatResponse represents the response from the LLM
type ChatResponse struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIClient implements Client for OpenAI-compatible APIs
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	sessionID  string
	httpClient *http.Client

	// maxAttempts includes the first try. Retries cover transient transport
	// failures and retryable HTTP status codes (429, 5xx, ...).
	maxAttempts    int
	retryBaseDelay time.Duration
	// sleep is swappable so tests do not have to wait for real backoff.
	sleep func(ctx context.Context, d time.Duration) error
}

// RuntimeClient is a placeholder used when agent execution is handled by an
// AgentRuntime instead of a chat-completions client.
type RuntimeClient struct {
	Name    string
	runtime agentruntime.Runtime
	initErr error
}

// NewRuntimeClient creates a placeholder runtime client.
func NewRuntimeClient(name string) *RuntimeClient {
	return &RuntimeClient{Name: name}
}

// NewRuntimeBackedClient creates a chat-completions compatible adapter over an
// agent runtime.
func NewRuntimeBackedClient(name string, runtime agentruntime.Runtime, initErr error) *RuntimeClient {
	return &RuntimeClient{Name: name, runtime: runtime, initErr: initErr}
}

// Runtime returns the underlying agent runtime when one is available.
func (c *RuntimeClient) Runtime() agentruntime.Runtime {
	if c == nil {
		return nil
	}
	return c.runtime
}

// ChatCompletion adapts simple chat-completion calls to the agent runtime.
func (c *RuntimeClient) ChatCompletion(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("runtime client is nil")
	}
	if c.runtime == nil {
		if c.initErr != nil {
			return nil, fmt.Errorf("provider %s agent runtime is unavailable: %w", c.Name, c.initErr)
		}
		return nil, fmt.Errorf("provider %s agent runtime is unavailable", c.Name)
	}
	if options == nil {
		options = &ChatOptions{}
	}

	systemPrompt, userPrompt := splitRuntimeMessages(messages)
	result, err := c.runtime.Invoke(ctx, agentruntime.Invocation{
		AgentName:     c.Name,
		Command:       "chat completion",
		WorkspaceRoot: runtimeWorkspaceRoot(),
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
		Options: agentruntime.Options{
			Model:       options.Model,
			Temperature: options.Temperature,
			MaxTokens:   options.MaxTokens,
		},
	})
	if err != nil {
		return nil, err
	}
	return &ChatResponse{
		Content: result.Content,
		Model:   result.Model,
		Usage: Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func runtimeWorkspaceRoot() string {
	if dir := strings.TrimSpace(logger.Default().ProjectDir()); dir != "" {
		return dir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func splitRuntimeMessages(messages []Message) (string, string) {
	var systemParts []string
	var userParts []string
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if strings.EqualFold(message.Role, "system") {
			systemParts = append(systemParts, content)
			continue
		}
		if strings.EqualFold(message.Role, "user") {
			userParts = append(userParts, content)
			continue
		}
		userParts = append(userParts, fmt.Sprintf("%s:\n%s", message.Role, content))
	}
	return strings.Join(systemParts, "\n\n"), strings.Join(userParts, "\n\n")
}

// OpenAIConfig contains configuration for OpenAI client
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout int // seconds
}

// NewOpenAIClient creates a new OpenAI-compatible client
func NewOpenAIClient(config *OpenAIConfig) *OpenAIClient {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 120
	}
	if config.Model == "" {
		config.Model = "gpt-3.5-turbo"
	}

	sessionID, err := newSessionID()
	if err != nil {
		logger.Error("Failed to generate session ID: %v", err)
	}

	return &OpenAIClient{
		apiKey:         config.APIKey,
		baseURL:        config.BaseURL,
		model:          config.Model,
		sessionID:      sessionID,
		maxAttempts:    defaultMaxAttempts(),
		retryBaseDelay: defaultRetryBaseDelay(),
		sleep:          sleepWithContext,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// newSessionID generates a stable random session ID (32 hex chars).
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// thinkingConfig represents the thinking configuration for the model
type thinkingConfig struct {
	Type string `json:"type"` // enabled/disabled/auto
}

// openAIRequest represents the request body for OpenAI API
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Thinking    *thinkingConfig `json:"thinking,omitempty"`
}

// openAIResponse represents the response from OpenAI API
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatCompletion sends a chat completion request to the OpenAI-compatible API
func (c *OpenAIClient) ChatCompletion(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	model := c.model
	temperature := 0.7
	maxTokens := 2000
	var thinking *thinkingConfig

	if options != nil {
		if options.Model != "" {
			model = options.Model
		}
		if options.Temperature != 0 {
			temperature = options.Temperature
		}
		if options.MaxTokens != 0 {
			maxTokens = options.MaxTokens
		}
		if options.Thinking != "" {
			thinking = &thinkingConfig{Type: string(options.Thinking)}
		}
	}

	logger.Debug("Temperature: %.2f", temperature)
	logger.Debug("Base URL: %s", c.baseURL)
	if thinking != nil {
		logger.Debug("Thinking mode: %s", thinking.Type)
	}

	// some strict providers (e.g. ox-alpha) reject temperature with >2 decimals
	temperature = math.Round(temperature*100) / 100

	reqBody := openAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Thinking:    thinking,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("Failed to marshal request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	attempts := c.maxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		// A fresh request per attempt: the request body is consumed by the
		// first try and must not be reused.
		req, err := c.newChatRequest(ctx, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		response, retryAfter, err := c.doChatCompletion(ctx, req)
		if err == nil {
			return response, nil
		}
		lastErr = err

		if attempt == attempts || !isRetryableError(err) {
			return nil, err
		}

		delay := c.backoffDelay(attempt, retryAfter)
		logger.Warn("LLM request failed (attempt %d/%d): %v; retrying in %s", attempt, attempts, err, delay.Round(time.Millisecond))
		if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
			return nil, fmt.Errorf("%w (last error: %v)", sleepErr, err)
		}
	}
	return nil, lastErr
}

// newChatRequest builds the HTTP request for one attempt.
func (c *OpenAIClient) newChatRequest(ctx context.Context, jsonData []byte) (*http.Request, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	if strings.Contains(strings.ToLower(c.baseURL), "opencode.ai") && c.sessionID != "" {
		req.Header.Set("x-opencode-session", c.sessionID)
	}
	return req, nil
}

// doChatCompletion performs one HTTP attempt. It returns the parsed response
// plus the server requested retry delay when the failure is retryable.
func (c *OpenAIClient) doChatCompletion(ctx context.Context, req *http.Request) (*ChatResponse, time.Duration, error) {
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("Response received in %v", time.Since(startTime))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, retryAfterFromHeader(resp.Header), &httpStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		logger.Debug("Response body: %s", string(body))
		return nil, 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, 0, fmt.Errorf("no choices in response")
	}

	return &ChatResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Model:   openAIResp.Model,
		Usage: Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, 0, nil
}

// httpStatusError carries the status code so the retry policy can decide.
type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Body)
}

// isRetryableError retries only failures that a second attempt can plausibly
// fix: transport errors, rate limits, and server-side problems.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return isRetryableStatus(statusErr.StatusCode)
	}
	// Anything without an HTTP status is a transport-level problem.
	return true
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooEarly, http.StatusTooManyRequests:
		return true
	}
	return code >= 500
}

// backoffDelay returns the wait before the next attempt, preferring the
// server's Retry-After hint and otherwise using exponential backoff with
// jitter.
func (c *OpenAIClient) backoffDelay(attempt int, serverHint time.Duration) time.Duration {
	base := c.retryBaseDelay
	if base <= 0 {
		base = defaultRetryBaseDelay()
	}
	if serverHint > 0 {
		if serverHint > maxRetryDelay {
			return maxRetryDelay
		}
		return serverHint
	}
	delay := base
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxRetryDelay {
			return maxRetryDelay
		}
	}
	return jitter(delay)
}

func retryAfterFromHeader(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

// jitter spreads retries so several workers do not hammer the API together.
func jitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	span := int64(delay) / 4
	if span <= 0 {
		return delay
	}
	buf := make([]byte, 1)
	if _, err := rand.Read(buf); err != nil {
		return delay
	}
	offset := int64(buf[0]) % (2*span + 1)
	return time.Duration(int64(delay) - span + offset)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

const maxRetryDelay = 60 * time.Second

// defaultMaxAttempts and defaultRetryBaseDelay can be tuned per deployment
// without a rebuild.
func defaultMaxAttempts() int {
	if value := strings.TrimSpace(os.Getenv("NOVELGEN_LLM_MAX_ATTEMPTS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 4
}

func defaultRetryBaseDelay() time.Duration {
	if value := strings.TrimSpace(os.Getenv("NOVELGEN_LLM_RETRY_BASE_MS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Millisecond
		}
	}
	return time.Second
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
