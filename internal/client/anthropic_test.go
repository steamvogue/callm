package client

import (
	"testing"
)

func TestConvertToAnthropicReq(t *testing.T) {
	req := ChatRequest{
		Model: "claude-sonnet-4-6",
		Messages: []Message{
			{Role: "system", Content: "You are a master coder."},
			{Role: "user", Content: "Write hello world."},
		},
		ReasoningEffort: "high",
	}

	areq, err := convertToAnthropicReq(req, true)
	if err != nil {
		t.Fatal(err)
	}

	if areq.System != "You are a master coder." {
		t.Fatalf("expected system prompt extracted, got: %q", areq.System)
	}
	if len(areq.Messages) != 1 || areq.Messages[0].Role != "user" {
		t.Fatalf("expected 1 user message, got: %v", areq.Messages)
	}
	if areq.Thinking == nil || areq.Thinking.Type != "adaptive" || areq.Thinking.BudgetTokens != 0 || areq.OutputConfig == nil || areq.OutputConfig.Effort != "high" {
		t.Fatalf("adaptive effort lost: %+v", areq)
	}
	if areq.MaxTokens != 4096 {
		t.Fatalf("implicit adaptive cap changed: %d", areq.MaxTokens)
	}

}

func TestPrepareRequestOpenAIO1(t *testing.T) {
	c := NewClient("https://api.openai.com/v1", "sk-test")
	maxTokens := 1000
	req := ChatRequest{
		Model:           "o3-mini",
		Messages:        []Message{{Role: "user", Content: "hello"}},
		MaxTokens:       &maxTokens,
		ReasoningEffort: "medium",
	}

	c.prepareRequest(&req)

	if req.MaxTokens != nil {
		t.Fatalf("max_tokens must be nil for o-series model")
	}
	if req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 1000 {
		t.Fatalf("expected max_completion_tokens to be 1000, got: %v", req.MaxCompletionTokens)
	}
	if req.Temperature != nil {
		t.Fatalf("temperature must be omitted for o-series model")
	}
}

func TestPrepareRequestOpenRouterReasoning(t *testing.T) {
	c := NewClient("https://openrouter.ai/api/v1", "sk-test")
	req := ChatRequest{
		Model:           "anthropic/claude-sonnet-4.6",
		Messages:        []Message{{Role: "user", Content: "hello"}},
		ReasoningEffort: "high",
	}

	c.prepareRequest(&req)

	if req.Reasoning == nil || req.Reasoning.Effort != "high" {
		t.Fatalf("expected reasoning effort high, got: %v", req.Reasoning)
	}
	if !req.IncludeReasoning {
		t.Fatalf("expected include_reasoning to be true")
	}
}
