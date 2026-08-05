package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	httpClient *http.Client
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

	return &OpenAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
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

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	startTime := time.Now()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(startTime)
	logger.Debug("Response received in %v", elapsed)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		logger.Debug("Response body: %s", string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ChatResponse{
		Content: openAIResp.Choices[0].Message.Content,
		Model:   openAIResp.Model,
		Usage: Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
