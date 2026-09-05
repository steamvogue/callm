package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client interacts with OpenAI-compatible APIs.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 0, // Streaming requests require no overall client timeout; request contexts are used instead.
		},
	}
}

func (c *Client) prepareRequest(req *ChatRequest) {
	isOModel := strings.HasPrefix(req.Model, "o1") || strings.HasPrefix(req.Model, "o3") ||
		strings.HasPrefix(req.Model, "openai/o1") || strings.HasPrefix(req.Model, "openai/o3")

	if isOModel {
		// OpenAI o-series models reject max_tokens and require max_completion_tokens
		if req.MaxTokens != nil && req.MaxCompletionTokens == nil {
			req.MaxCompletionTokens = req.MaxTokens
			req.MaxTokens = nil
		}
		// OpenAI o-series models reject custom temperature
		req.Temperature = nil
	}

	isOpenRouterOrStraitly := strings.Contains(c.BaseURL, "openrouter.ai") || strings.Contains(c.BaseURL, "straitly.ai")
	if isOpenRouterOrStraitly {
		if req.ReasoningEffort != "" {
			req.Reasoning = &ReasoningConfig{
				Effort: strings.ToLower(req.ReasoningEffort),
			}
			req.IncludeReasoning = true
		}
		if req.Thinking != nil && req.Reasoning == nil {
			req.Reasoning = &ReasoningConfig{
				MaxTokens: req.Thinking.BudgetTokens,
			}
			req.IncludeReasoning = true
		}
	}
}

// StreamChat initiates an SSE streaming completion and yields chunks to the callback.
func (c *Client) StreamChat(ctx context.Context, req ChatRequest, onChunk func(chunk StreamChunk) error) (*Usage, error) {
	if c.isAnthropicURL() {
		return c.streamAnthropic(ctx, req, onChunk)
	}

	c.prepareRequest(&req)
	req.Stream = true
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var lastUsage *Usage
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return lastUsage, fmt.Errorf("stream read error: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Error != nil {
			return lastUsage, fmt.Errorf("stream API error: %s", chunk.Error.Message)
		}

		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}

		if err := onChunk(chunk); err != nil {
			return lastUsage, err
		}
	}

	return lastUsage, nil
}

// Chat performs a non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c.isAnthropicURL() {
		return c.chatAnthropic(ctx, req)
	}

	c.prepareRequest(&req)
	req.Stream = false
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// Set a reasonable timeout for non-streaming completions
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	return &chatResp, nil
}

// ListModels retrieves available models from /v1/models.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := c.BaseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.isAnthropicURL() {
		httpReq.Header.Set("x-api-key", c.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var modelList ModelListResponse
	if err := json.Unmarshal(respBody, &modelList); err != nil {
		return nil, fmt.Errorf("failed to parse models JSON: %w", err)
	}

	if modelList.Error != nil {
		return nil, fmt.Errorf("API error: %s", modelList.Error.Message)
	}

	return modelList.Data, nil
}

// RawRequest sends an arbitrary payload to an endpoint under BaseURL.
func (c *Client) RawRequest(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	url := c.BaseURL + endpoint

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.isAnthropicURL() {
		httpReq.Header.Set("x-api-key", c.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
