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
)

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicReq struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Thinking    *ThinkingConfig    `json:"thinking,omitempty"`
}

type anthropicContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

type anthropicDelta struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

type anthropicEvent struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index,omitempty"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *anthropicDelta        `json:"delta,omitempty"`
	Message      *struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// isAnthropicURL checks if the endpoint is Anthropic's direct API.
func (c *Client) isAnthropicURL() bool {
	return strings.Contains(c.BaseURL, "api.anthropic.com")
}

// StreamAnthropic streams responses from Anthropic /v1/messages.
func (c *Client) streamAnthropic(ctx context.Context, req ChatRequest, onChunk func(chunk StreamChunk) error) (*Usage, error) {
	areq := convertToAnthropicReq(req, true)
	bodyBytes, err := json.Marshal(areq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode anthropic request: %w", err)
	}

	url := c.BaseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var totalUsage Usage
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return &totalUsage, fmt.Errorf("stream read error: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var ev anthropicEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		if ev.Error != nil && ev.Error.Message != "" {
			return &totalUsage, fmt.Errorf("anthropic stream error: %s", ev.Error.Message)
		}

		if ev.Message != nil && ev.Message.Usage.InputTokens > 0 {
			totalUsage.PromptTokens = ev.Message.Usage.InputTokens
			totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
		}
		if ev.Usage != nil && ev.Usage.OutputTokens > 0 {
			totalUsage.CompletionTokens = ev.Usage.OutputTokens
			totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
		}

		if ev.Delta != nil {
			chunk := StreamChunk{
				Model: req.Model,
			}
			if ev.Delta.Type == "thinking_delta" && ev.Delta.Thinking != "" {
				chunk.Choices = []StreamChoice{
					{
						Delta: StreamDelta{
							Reasoning: ev.Delta.Thinking,
						},
					},
				}
				if err := onChunk(chunk); err != nil {
					return &totalUsage, err
				}
			} else if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
				chunk.Choices = []StreamChoice{
					{
						Delta: StreamDelta{
							Content: ev.Delta.Text,
						},
					},
				}
				if err := onChunk(chunk); err != nil {
					return &totalUsage, err
				}
			}
		}
	}

	return &totalUsage, nil
}

// chatAnthropic performs non-streaming completion via Anthropic /v1/messages.
func (c *Client) chatAnthropic(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	areq := convertToAnthropicReq(req, false)
	bodyBytes, err := json.Marshal(areq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode anthropic request: %w", err)
	}

	url := c.BaseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var raw struct {
		ID      string                  `json:"id"`
		Content []anthropicContentBlock `json:"content"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var fullContent strings.Builder
	var fullReasoning strings.Builder

	for _, block := range raw.Content {
		if block.Type == "thinking" {
			fullReasoning.WriteString(block.Thinking)
		} else if block.Type == "text" {
			fullContent.WriteString(block.Text)
		}
	}

	return &ChatResponse{
		ID:    raw.ID,
		Model: req.Model,
		Choices: []ChatChoice{
			{
				Message: RespMsg{
					Role:      "assistant",
					Content:   fullContent.String(),
					Reasoning: fullReasoning.String(),
				},
			},
		},
		Usage: &Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      raw.Usage.InputTokens + raw.Usage.OutputTokens,
		},
	}, nil
}

func convertToAnthropicReq(req ChatRequest, stream bool) anthropicReq {
	var systemParts []string
	var messages []anthropicMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok {
				systemParts = append(systemParts, s)
			}
		} else {
			messages = append(messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	} else if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		maxTokens = *req.MaxCompletionTokens
	}

	var thinking *ThinkingConfig
	if req.Thinking != nil {
		thinking = req.Thinking
	} else if req.ReasoningEffort != "" {
		budget := 2048
		switch strings.ToLower(req.ReasoningEffort) {
		case "low":
			budget = 1024
		case "medium":
			budget = 2048
		case "high":
			budget = 4096
		}
		thinking = &ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: budget,
		}
	}

	if thinking != nil && maxTokens <= thinking.BudgetTokens {
		maxTokens = thinking.BudgetTokens + 2048
	}

	return anthropicReq{
		Model:       req.Model,
		Messages:    messages,
		System:      strings.Join(systemParts, "\n\n"),
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      stream,
		Thinking:    thinking,
	}
}
