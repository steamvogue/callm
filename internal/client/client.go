package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client holds protocol selection separately from the endpoint, so proxies preserve it.
type Client struct {
	BaseURL           string
	APIKey            string
	Provider          string
	HTTPClient        *http.Client
	StreamIdleTimeout time.Duration
	IncludeCost       bool
	UserAgent         string
}

const DefaultTimeout = 300 * time.Second
const DefaultUserAgent = "CallM (Call-LLM; +https://github.com/steamvogue/callm)"
const maxResponseBytes = 64 << 20

// NewClient accepts an explicit provider; otherwise only exact known hostnames are detected.
func NewClient(baseURL, apiKey string, provider ...string) *Client {
	selected := ""
	u, _ := url.Parse(baseURL)
	if u != nil {
		switch u.Hostname() {
		case "api.anthropic.com":
			selected = "ant"
		case "openrouter.ai":
			selected = "or"
		case "api.orcarouter.ai":
			selected = "orca"
		case "api.straitly.ai":
			selected = "st"
		}
	}
	if len(provider) > 0 && provider[0] != "" {
		selected = provider[0]
	}
	transport := http.DefaultTransport
	if base, ok := transport.(*http.Transport); ok {
		clone := base.Clone()
		clone.ResponseHeaderTimeout = DefaultTimeout
		transport = clone
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Provider: selected,
		UserAgent:         DefaultUserAgent,
		StreamIdleTimeout: DefaultTimeout,
		HTTPClient: &http.Client{Timeout: DefaultTimeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			original := via[0].URL
			if req.URL.Scheme != original.Scheme || !strings.EqualFold(req.URL.Host, original.Host) {
				return fmt.Errorf("refusing cross-origin redirect")
			}
			return nil
		}},
	}
}

func (c *Client) isAnthropicURL() bool { return c.Provider == "ant" }

func validateRequest(req ChatRequest) error {
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return fmt.Errorf("max-tokens must be positive")
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens <= 0 {
		return fmt.Errorf("max-completion-tokens must be positive")
	}
	if req.MaxTokens != nil && req.MaxCompletionTokens != nil {
		return fmt.Errorf("choose either max-tokens or max-completion-tokens")
	}
	if req.Temperature != nil && (math.IsNaN(*req.Temperature) || math.IsInf(*req.Temperature, 0) || *req.Temperature < 0 || *req.Temperature > 2) {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if req.TopP != nil && (math.IsNaN(*req.TopP) || math.IsInf(*req.TopP, 0) || *req.TopP < 0 || *req.TopP > 1) {
		return fmt.Errorf("top-p must be between 0 and 1")
	}
	if req.ReasoningEffort != "" {
		switch strings.ToLower(req.ReasoningEffort) {
		case "low", "medium", "high":
		default:
			return fmt.Errorf("effort must be low, medium, or high")
		}
	}
	if req.Thinking != nil {
		if req.Thinking.BudgetTokens <= 0 {
			return fmt.Errorf("thinking-budget must be positive")
		}
		if req.ReasoningEffort != "" {
			return fmt.Errorf("choose either effort or thinking-budget")
		}
	}
	return nil
}

func (c *Client) prepareRequest(req *ChatRequest) error {
	if err := validateRequest(*req); err != nil {
		return err
	}
	req.ReasoningEffort = strings.ToLower(req.ReasoningEffort)
	model := strings.TrimPrefix(req.Model, "openai/")
	isOModel := false
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			isOModel = true
		}
	}
	if isOModel {
		if req.Temperature != nil {
			return fmt.Errorf("temperature is unsupported for %s", req.Model)
		}
		if req.MaxTokens != nil {
			req.MaxCompletionTokens = req.MaxTokens
			req.MaxTokens = nil
		}
	}
	if c.Provider == "or" || c.Provider == "st" {
		if req.ReasoningEffort != "" {
			req.Reasoning = &ReasoningConfig{Effort: req.ReasoningEffort}
			req.ReasoningEffort = ""
		}
		if req.Thinking != nil {
			req.Reasoning = &ReasoningConfig{MaxTokens: req.Thinking.BudgetTokens}
			req.Thinking = nil
		}
		if req.Reasoning != nil {
			req.IncludeReasoning = true
		}
	} else if req.Thinking != nil {
		if c.Provider == "orca" {
			return fmt.Errorf("use --effort for OrcaRouter reasoning; --thinking-budget is not supported by this preset")
		}
		return fmt.Errorf("thinking-budget requires an Anthropic, OpenRouter, or Straitly provider")
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, body []byte) (*http.Request, error) {
	for _, ch := range c.UserAgent {
		if ch < 32 || ch == 127 {
			return nil, fmt.Errorf("user-agent must not contain control characters")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// An explicit empty value suppresses net/http's automatic Go user agent.
	req.Header.Set("User-Agent", c.UserAgent)
	if c.isAnthropicURL() {
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Provider == "orca" && c.IncludeCost {
		req.Header.Set("X-OrcaRouter-Include-Cost", "true")
	}
	return req, nil
}

func responseBytes(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(b) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	var envelope struct {
		Error *APIError `json:"error"`
	}
	if json.Unmarshal(b, &envelope) == nil && envelope.Error != nil {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, envelope.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error (status %d): %s", resp.StatusCode, b)
	}
	return b, nil
}

func (c *Client) StreamChat(ctx context.Context, req ChatRequest, onChunk func(StreamChunk) error) (*Usage, error) {
	if c.isAnthropicURL() {
		return c.streamAnthropic(ctx, req, onChunk)
	}
	if err := c.prepareRequest(&req); err != nil {
		return nil, err
	}
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := c.openStream(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reader := newSSEReader(resp.Body)
	var usage *Usage
	for {
		data, err := reader.next()
		if err != nil {
			return usage, fmt.Errorf("stream ended before [DONE]: %w", err)
		}
		if data == "[DONE]" {
			return usage, nil
		}
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return usage, fmt.Errorf("invalid stream JSON: %w", err)
		}
		if chunk.Error != nil {
			return usage, fmt.Errorf("stream API error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if err := onChunk(chunk); err != nil {
			return usage, err
		}
	}
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if c.isAnthropicURL() {
		return c.chatAnthropic(ctx, req)
	}
	if err := c.prepareRequest(&req); err != nil {
		return nil, err
	}
	req.Stream = false
	req.StreamOptions = nil
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	request, err := c.request(ctx, http.MethodPost, "/chat/completions", b)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	b, err = responseBytes(resp)
	if err != nil {
		return nil, err
	}
	var result ChatResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("invalid response JSON: %w", err)
	}
	result.Raw = append([]byte(nil), b...)
	return &result, nil
}

func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Keep pagination under the same overall operation deadline.
	if c.HTTPClient.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.HTTPClient.Timeout)
		defer cancel()
	}
	var all []ModelInfo
	cursor := ""
	seen := map[string]bool{}
	for {
		endpoint := "/models"
		if cursor != "" {
			endpoint += "?after_id=" + url.QueryEscape(cursor)
		}
		req, err := c.request(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		b, err := responseBytes(resp)
		if err != nil {
			return nil, err
		}
		var page ModelListResponse
		if err := json.Unmarshal(b, &page); err != nil {
			return nil, fmt.Errorf("invalid model catalog: %w", err)
		}
		for _, model := range page.Data {
			if model.Name == "" {
				model.Name = model.DisplayName
			}
			if model.ContextLength == 0 {
				model.ContextLength = model.MaxInputTokens
			}
			all = append(all, model)
		}
		if !page.HasMore {
			return all, nil
		}
		if page.LastID == "" || seen[page.LastID] {
			return nil, fmt.Errorf("catalog pagination did not advance")
		}
		seen[page.LastID] = true
		cursor = page.LastID
	}
}

func (c *Client) RawRequest(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("raw request body must be valid JSON")
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	req, err := c.request(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	return responseBytes(resp)
}

// openStream applies content-type validation and an independently configurable idle timer.
func (c *Client) openStream(ctx context.Context, endpoint string, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	req, err := c.request(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		cancel(err)
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		cancel(err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, err = responseBytes(resp)
		cancel(err)
		return nil, err
	}
	if err = checkSSEType(resp.Header.Get("Content-Type")); err != nil {
		resp.Body.Close()
		cancel(err)
		return nil, err
	}
	resp.Body = &idleBody{ReadCloser: resp.Body, timeout: c.StreamIdleTimeout, cancel: cancel, ctx: ctx}
	return resp, nil
}

type idleBody struct {
	io.ReadCloser
	timeout time.Duration
	cancel  context.CancelCauseFunc
	ctx     context.Context
}

func (b *idleBody) Read(p []byte) (int, error) {
	var timer *time.Timer
	if b.timeout > 0 {
		timer = time.AfterFunc(b.timeout, func() { b.cancel(fmt.Errorf("stream idle timeout after %s", b.timeout)) })
	}
	n, err := b.ReadCloser.Read(p)
	if timer != nil {
		timer.Stop()
	}
	if err != nil && context.Cause(b.ctx) != nil {
		err = context.Cause(b.ctx)
	}
	return n, err
}
func (b *idleBody) Close() error { b.cancel(context.Canceled); return b.ReadCloser.Close() }
