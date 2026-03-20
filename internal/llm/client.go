package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nolvegen/internal/logger"
)

// Client interface for LLM providers
type Client interface {
	ChatCompletion(messages []Message, options *ChatOptions) (*ChatResponse, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions contains optional parameters for chat completion
type ChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Model       string  `json:"model,omitempty"`
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
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// openAIRequest represents the request body for OpenAI API
type openAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
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
func (c *OpenAIClient) ChatCompletion(messages []Message, options *ChatOptions) (*ChatResponse, error) {
	model := c.model
	temperature := 0.7
	maxTokens := 2000

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
	}

	// Log request
	logger.LLMRequest(model, len(messages), maxTokens)
	logger.Debug("Temperature: %.2f", temperature)
	logger.Debug("Base URL: %s", c.baseURL)

	// Log messages
	for i, msg := range messages {
		logger.Debug("Message %d [%s]: %s", i, msg.Role, truncateString(msg.Content, 200))
	}

	reqBody := openAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("Failed to marshal request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debug("Request JSON: %s", string(jsonData))

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	logger.Info("Sending request to %s", url)
	startTime := time.Now()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send request: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(startTime)
	logger.Info("Response received in %v", elapsed)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("API request failed with status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		logger.Error("Failed to unmarshal response: %v", err)
		logger.Debug("Response body: %s", string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		logger.Error("No choices in response")
		return nil, fmt.Errorf("no choices in response")
	}

	// Log response
	logger.LLMResponse(openAIResp.Model, openAIResp.Usage.TotalTokens, openAIResp.Choices[0].Message.Content)

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
