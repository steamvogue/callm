package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderAPICallMinimal(t *testing.T) {
	providers := []struct {
		name     string
		model    string
		endpoint string
	}{
		{"straitly", "deepseek/deepseek-v4-flash-0731", "/chat/completions"},
		{"openrouter", "deepseek/deepseek-v4-flash-0731", "/chat/completions"},
		{"deepseek", "deepseek-chat", "/chat/completions"},
		{"openai", "gpt-4o", "/chat/completions"},
		{"moonshot", "moonshot-v1-auto", "/chat/completions"},
		{"zhipu", "glm-4-flash", "/chat/completions"},
		{"qwen", "qwen-plus", "/chat/completions"},
		{"groq", "llama-3.3-70b-versatile", "/chat/completions"},
		{"ollama", "deepseek-r1", "/chat/completions"},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != p.endpoint {
					t.Errorf("[%s] expected path %s, got %s", p.name, p.endpoint, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("[%s] expected Bearer test-key, got %s", p.name, r.Header.Get("Authorization"))
				}

				var req ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("[%s] failed to decode body: %v", p.name, err)
				}
				if req.Model != p.model {
					t.Errorf("[%s] expected model %s, got %s", p.name, p.model, req.Model)
				}

				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"OK_\"}}]}\n\n")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"MINIMAL\"}}]}\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()

			c := NewClient(server.URL, "test-key")
			var received strings.Builder

			usage, err := c.StreamChat(context.Background(), ChatRequest{
				Model: p.model,
				Messages: []Message{
					{Role: "user", Content: "Ping"},
				},
			}, func(chunk StreamChunk) error {
				for _, ch := range chunk.Choices {
					received.WriteString(ch.Delta.Content)
				}
				return nil
			})

			if err != nil {
				t.Fatalf("[%s] StreamChat error: %v", p.name, err)
			}
			_ = usage

			if received.String() != "OK_MINIMAL" {
				t.Fatalf("[%s] expected 'OK_MINIMAL', got %q", p.name, received.String())
			}
		})
	}
}

func TestProviderAPICallReasoning(t *testing.T) {
	testCases := []struct {
		name            string
		model           string
		effort          string
		simulatedDelta  string
		expectReasoning string
		checkPayload    func(t *testing.T, req ChatRequest)
	}{
		{
			name:            "deepseek_reasoner",
			model:           "deepseek-reasoner",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Comparing 9.11 vs 9.9...\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"9.9 is larger.\"}}]}\n\n",
			expectReasoning: "Comparing 9.11 vs 9.9...",
		},
		{
			name:            "openrouter_reasoning",
			model:           "deepseek/deepseek-r1",
			effort:          "high",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning\":\"OpenRouter step-by-step thinking\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"Done.\"}}]}\n\n",
			expectReasoning: "OpenRouter step-by-step thinking",
			checkPayload: func(t *testing.T, req ChatRequest) {
				if !req.IncludeReasoning {
					t.Errorf("expected include_reasoning=true")
				}
				if req.Reasoning == nil || req.Reasoning.Effort != "high" {
					t.Errorf("expected reasoning effort high, got %v", req.Reasoning)
				}
			},
		},
		{
			name:            "openai_o3_mini_reasoning",
			model:           "o3-mini",
			effort:          "high",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"content\":\"o3-mini direct solution.\"}}]}\n\n",
			expectReasoning: "",
			checkPayload: func(t *testing.T, req ChatRequest) {
				if req.ReasoningEffort != "high" {
					t.Errorf("expected reasoning_effort='high', got %s", req.ReasoningEffort)
				}
				if req.Temperature != nil {
					t.Errorf("temperature must be nil for o3-mini")
				}
			},
		},
		{
			name:            "moonshot_reasoning",
			model:           "moonshot-v1-auto",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Kimi thinking step\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"Kimi answer.\"}}]}\n\n",
			expectReasoning: "Kimi thinking step",
		},
		{
			name:            "zhipu_glm_reasoning",
			model:           "glm-4-flash",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"GLM zero reasoning\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"GLM answer.\"}}]}\n\n",
			expectReasoning: "GLM zero reasoning",
		},
		{
			name:            "qwen_qwq_reasoning",
			model:           "qwq-32b",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"QwQ reasoning tokens\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"QwQ math answer.\"}}]}\n\n",
			expectReasoning: "QwQ reasoning tokens",
		},
		{
			name:            "groq_reasoning",
			model:           "deepseek-r1-distill-llama-70b",
			simulatedDelta:  "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Groq fast reasoning\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"Groq answer.\"}}]}\n\n",
			expectReasoning: "Groq fast reasoning",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("[%s] json decode: %v", tc.name, err)
				}
				if tc.checkPayload != nil {
					tc.checkPayload(t, req)
				}

				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, tc.simulatedDelta)
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()

			baseURL := server.URL
			if tc.name == "openrouter_reasoning" {
				// simulate OpenRouter endpoint check in prepareRequest
				baseURL = server.URL + "/openrouter.ai/api/v1"
			}

			provider := ""
			if tc.name == "openrouter_reasoning" {
				provider = "or"
			}
			c := NewClient(baseURL, "test-key", provider)
			var reasoning strings.Builder
			var content strings.Builder

			_, err := c.StreamChat(context.Background(), ChatRequest{
				Model:           tc.model,
				ReasoningEffort: tc.effort,
				Messages: []Message{
					{Role: "user", Content: "Solve"},
				},
			}, func(chunk StreamChunk) error {
				for _, ch := range chunk.Choices {
					if ch.Delta.Reasoning != "" {
						reasoning.WriteString(ch.Delta.Reasoning)
					}
					if ch.Delta.ReasoningContent != "" {
						reasoning.WriteString(ch.Delta.ReasoningContent)
					}
					if ch.Delta.Content != "" {
						content.WriteString(ch.Delta.Content)
					}
				}
				return nil
			})

			if err != nil {
				t.Fatalf("[%s] StreamChat failed: %v", tc.name, err)
			}

			if tc.expectReasoning != "" && !strings.Contains(reasoning.String(), tc.expectReasoning) {
				t.Fatalf("[%s] expected reasoning %q, got %q", tc.name, tc.expectReasoning, reasoning.String())
			}
		})
	}
}
