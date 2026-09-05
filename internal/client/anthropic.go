package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	TopP        *float64           `json:"top_p,omitempty"`
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
type anthropicUsage struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}
type anthropicEvent struct {
	Type         string                 `json:"type"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *anthropicDelta        `json:"delta,omitempty"`
	Message      *struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message,omitempty"`
	Usage *anthropicUsage `json:"usage,omitempty"`
	Error *APIError       `json:"error,omitempty"`
}

func (c *Client) streamAnthropic(ctx context.Context, req ChatRequest, onChunk func(StreamChunk) error) (*Usage, error) {
	areq, err := convertToAnthropicReq(req, true)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(areq)
	if err != nil {
		return nil, err
	}
	resp, err := c.openStream(ctx, "/messages", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reader := newSSEReader(resp.Body)
	var usage *Usage
	for {
		data, err := reader.next()
		if err != nil {
			return usage, fmt.Errorf("Anthropic stream ended before message_stop: %w", err)
		}
		var ev anthropicEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return usage, fmt.Errorf("invalid Anthropic stream JSON: %w", err)
		}
		if ev.Error != nil {
			return usage, fmt.Errorf("Anthropic stream error: %s", ev.Error.Message)
		}
		if ev.Message != nil {
			u := ev.Message.Usage
			usage = &Usage{PromptTokens: u.InputTokens, CompletionTokens: u.OutputTokens, TotalTokens: u.InputTokens + u.OutputTokens, CacheReadInputTokens: u.CacheReadInputTokens}
		}
		if ev.Usage != nil {
			if usage == nil {
				usage = &Usage{}
			}
			usage.CompletionTokens = ev.Usage.OutputTokens
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if ev.Type == "message_stop" {
			return usage, nil
		}
		delta := StreamDelta{}
		if ev.ContentBlock != nil {
			delta.Content = ev.ContentBlock.Text
			delta.Reasoning = ev.ContentBlock.Thinking
		}
		if ev.Delta != nil {
			delta.Content = ev.Delta.Text
			delta.Reasoning = ev.Delta.Thinking
		}
		if delta.Content != "" || delta.Reasoning != "" {
			if err := onChunk(StreamChunk{Model: req.Model, Choices: []StreamChoice{{Delta: delta}}}); err != nil {
				return usage, err
			}
		}
	}
}

func (c *Client) chatAnthropic(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	areq, err := convertToAnthropicReq(req, false)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(areq)
	if err != nil {
		return nil, err
	}
	request, err := c.request(ctx, http.MethodPost, "/messages", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	body, err = responseBytes(resp)
	if err != nil {
		return nil, err
	}
	var raw struct {
		ID         string                  `json:"id"`
		Model      string                  `json:"model"`
		StopReason string                  `json:"stop_reason"`
		Content    []anthropicContentBlock `json:"content"`
		Usage      *anthropicUsage         `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid Anthropic response JSON: %w", err)
	}
	var content, reasoning strings.Builder
	for _, block := range raw.Content {
		switch block.Type {
		case "thinking":
			reasoning.WriteString(block.Thinking)
		case "text":
			content.WriteString(block.Text)
		}
	}
	model := raw.Model
	if model == "" {
		model = req.Model
	}
	result := &ChatResponse{Raw: body, ID: raw.ID, Model: model, Choices: []ChatChoice{{FinishReason: raw.StopReason, Message: RespMsg{Role: "assistant", Content: content.String(), Reasoning: reasoning.String()}}}}
	if raw.Usage != nil {
		u := raw.Usage
		result.Usage = &Usage{PromptTokens: u.InputTokens, CompletionTokens: u.OutputTokens, TotalTokens: u.InputTokens + u.OutputTokens, CacheReadInputTokens: u.CacheReadInputTokens}
	}
	return result, nil
}

func convertToAnthropicReq(req ChatRequest, stream bool) (anthropicReq, error) {
	if err := validateRequest(req); err != nil {
		return anthropicReq{}, err
	}
	if req.ResponseFormat != nil {
		return anthropicReq{}, fmt.Errorf("json-object is not supported by Anthropic Messages; use raw with an Anthropic structured-output schema")
	}
	if req.Temperature != nil && *req.Temperature > 1 {
		return anthropicReq{}, fmt.Errorf("Anthropic temperature must be between 0 and 1")
	}
	var system []string
	var messages []anthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			text, ok := msg.Content.(string)
			if !ok {
				return anthropicReq{}, fmt.Errorf("Anthropic system content must be text")
			}
			system = append(system, text)
			continue
		}
		content, err := anthropicContent(msg.Content)
		if err != nil {
			return anthropicReq{}, err
		}
		messages = append(messages, anthropicMessage{Role: msg.Role, Content: content})
	}
	maxTokens := 4096
	explicit := req.MaxTokens != nil || req.MaxCompletionTokens != nil
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	} else if req.MaxCompletionTokens != nil {
		maxTokens = *req.MaxCompletionTokens
	}
	thinking := req.Thinking
	if req.ReasoningEffort != "" {
		budget := map[string]int{"low": 1024, "medium": 2048, "high": 4096}[strings.ToLower(req.ReasoningEffort)]
		thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: budget}
	}
	if thinking != nil {
		if thinking.BudgetTokens < 1024 {
			return anthropicReq{}, fmt.Errorf("Anthropic thinking-budget must be at least 1024")
		}
		if maxTokens <= thinking.BudgetTokens {
			if explicit {
				return anthropicReq{}, fmt.Errorf("thinking-budget must be smaller than the explicit token cap (%d)", maxTokens)
			}
			if thinking.BudgetTokens > int(^uint(0)>>1)-2048 {
				return anthropicReq{}, fmt.Errorf("thinking-budget is too large")
			}
			maxTokens = thinking.BudgetTokens + 2048
		}
		if req.Temperature != nil && *req.Temperature != 1 {
			return anthropicReq{}, fmt.Errorf("Anthropic thinking requires temperature 1 or omission")
		}
		if req.TopP != nil && *req.TopP < 0.95 {
			return anthropicReq{}, fmt.Errorf("Anthropic thinking requires top-p between 0.95 and 1")
		}
	}
	return anthropicReq{Model: req.Model, Messages: messages, System: strings.Join(system, "\n\n"), MaxTokens: maxTokens, Temperature: req.Temperature, TopP: req.TopP, Stream: stream, Thinking: thinking}, nil
}

func anthropicContent(content interface{}) (interface{}, error) {
	if text, ok := content.(string); ok {
		return text, nil
	}
	data, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	var parts []ContentPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return nil, fmt.Errorf("invalid image/text content: %w", err)
	}
	blocks := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": part.Text})
			continue
		}
		if part.Type != "image_url" || part.ImageURL == nil {
			return nil, fmt.Errorf("unsupported Anthropic content type %q", part.Type)
		}
		image := part.ImageURL.URL
		source := map[string]interface{}{}
		if strings.HasPrefix(image, "data:") {
			metadata, payload, ok := strings.Cut(strings.TrimPrefix(image, "data:"), ",")
			if !ok || !strings.HasSuffix(metadata, ";base64") {
				return nil, fmt.Errorf("image must be a base64 data URI")
			}
			media := strings.TrimSuffix(metadata, ";base64")
			switch media {
			case "image/png", "image/jpeg", "image/gif", "image/webp":
			default:
				return nil, fmt.Errorf("unsupported image type %q", media)
			}
			if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
				return nil, fmt.Errorf("invalid base64 image: %w", err)
			}
			source = map[string]interface{}{"type": "base64", "media_type": media, "data": payload}
		} else {
			u, err := url.Parse(image)
			if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
				return nil, fmt.Errorf("image URL must use http or https")
			}
			source = map[string]interface{}{"type": "url", "url": image}
		}
		blocks = append(blocks, map[string]interface{}{"type": "image", "source": source})
	}
	return blocks, nil
}
