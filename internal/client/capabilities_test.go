package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeAdaptiveProfiles(t *testing.T) {
	cap := 200
	temperature := 0.2
	for _, model := range []string{"claude-sonnet-5-5", "claude-sonnet-5-5-20261001", "claude-haiku-5-5", "claude-opus-5-5", "claude-fable-5-1", "claude-sonnet-4-6"} {
		req, err := convertToAnthropicReq(ChatRequest{Model: model, ReasoningEffort: "HIGH", MaxTokens: &cap}, false)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		body, _ := json.Marshal(req)
		if req.MaxTokens != cap || req.OutputConfig == nil || req.OutputConfig.Effort != "high" || !strings.Contains(string(body), `"thinking":{"type":"adaptive"}`) || strings.Contains(string(body), "budget_tokens") {
			t.Fatalf("%s: %s", model, body)
		}
	}
	for _, tc := range []struct {
		model, effort string
		valid         bool
	}{
		{"claude-sonnet-4-6", "max", true}, {"claude-sonnet-4-6", "xhigh", false}, {"claude-sonnet-5-5", "xhigh", true}, {"claude-haiku-5-5", "max", true}, {"claude-sonnet-5-5", "none", false}, {"custom-model", "high", true}, {"custom-model", "max", false},
	} {
		_, err := convertToAnthropicReq(ChatRequest{Model: tc.model, ReasoningEffort: tc.effort}, false)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v error=%v", tc, err)
		}
	}
	for _, req := range []ChatRequest{{Model: "claude-sonnet-5-5", Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 1024}}, {Model: "claude-opus-4-8", Temperature: &temperature}, {Model: "claude-sonnet-4-6", ReasoningEffort: "high", Temperature: &temperature}} {
		if _, err := convertToAnthropicReq(req, false); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	legacy, err := convertToAnthropicReq(ChatRequest{Model: "claude-sonnet-4-5", ReasoningEffort: "high"}, false)
	if err != nil || legacy.Thinking.BudgetTokens != 4096 || legacy.MaxTokens != 6144 {
		t.Fatalf("legacy %+v %v", legacy, err)
	}
}

func TestOpenAIModelProfiles(t *testing.T) {
	cap := 200
	temp := 0.2
	top := 0.9
	for _, tc := range []struct {
		model, effort            string
		sample, valid, normalize bool
	}{
		{"gpt-6.1-sol", "high", false, true, true}, {"gpt-6.1-sol", "none", false, false, true}, {"gpt-6.1-sol", "minimal", false, false, true},
		{"gpt-6-astra", "max", false, true, true}, {"gpt-6-astra", "none", false, false, true},
		{"gpt-6-luna", "none", true, true, true}, {"gpt-6-sol", "none", true, true, true}, {"gpt-6-luna", "high", true, false, true}, {"gpt-6-luna", "", true, false, true},
		{"gpt-6.1-sol-2026-10-01", "xhigh", false, true, true}, {"openai/gpt-6-luna", "max", false, true, true},
		{"o3-mini", "medium", false, true, true}, {"o3-mini", "medium", true, false, true},
		{"custom-model", "high", true, true, false}, {"custom-model", "none", false, false, false}, {"gpt-6-future", "high", false, true, false},
	} {
		t.Run(tc.model+"/"+tc.effort, func(t *testing.T) {
			req := ChatRequest{Model: tc.model, ReasoningEffort: tc.effort, MaxTokens: &cap}
			if tc.sample {
				req.Temperature = &temp
				req.TopP = &top
			}
			err := NewClient("https://api.openai.com/v1", "dummy", "oa").prepareRequest(&req)
			if (err == nil) != tc.valid {
				t.Fatalf("%+v error=%v", tc, err)
			}
			if tc.valid {
				if tc.normalize {
					if req.MaxTokens != nil || req.MaxCompletionTokens == nil || *req.MaxCompletionTokens != 200 {
						t.Fatalf("cap %+v", req)
					}
				} else if req.MaxTokens == nil || req.MaxCompletionTokens != nil {
					t.Fatalf("custom cap %+v", req)
				}
			}
		})
	}
	req := ChatRequest{Model: "openai/gpt-6-luna", ReasoningEffort: "none", MaxTokens: &cap}
	if err := NewClient("https://openrouter.ai/api/v1", "dummy", "or").prepareRequest(&req); err != nil || req.Reasoning == nil || req.Reasoning.Effort != "none" || req.ReasoningEffort != "" {
		t.Fatalf("gateway %+v %v", req, err)
	}
}
