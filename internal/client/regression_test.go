package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamValidation(t *testing.T) {
	for _, tc := range []struct {
		name, typ, body string
		valid           bool
	}{
		{"space", "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n", true},
		{"no-space-cr", "text/event-stream", "data:{\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\r\rdata:[DONE]\r\r", true},
		{"multiline", "text/event-stream", "data: {\"choices\":\ndata: [{\"delta\":{\"content\":\"ok\"}}]}\n\ndata:[DONE]\n\n", true},
		{"malformed", "text/event-stream", "data: {broken}\n\ndata:[DONE]\n\n", false},
		{"truncated", "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n", false},
		{"wrong-type", "application/json", "{}", false},
		{"api-error", "text/event-stream", "data: {\"error\":{\"message\":\"failed\"}}\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.typ)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			c := NewClient(server.URL, "dummy")
			content := ""
			_, err := c.StreamChat(context.Background(), ChatRequest{}, func(chunk StreamChunk) error {
				for _, ch := range chunk.Choices {
					content += ch.Delta.Content
				}
				return nil
			})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && content != "ok" {
				t.Fatalf("content=%q", content)
			}
		})
	}
}

func TestStatusErrorsAndRawPreservation(t *testing.T) {
	for _, code := range []int{200, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			io.WriteString(w, `{"error":{"message":"rejected"}}`)
		}))
		c := NewClient(server.URL, "dummy")
		if _, err := c.Chat(context.Background(), ChatRequest{}); err == nil {
			t.Errorf("chat accepted error status %d", code)
		}
		if _, err := c.RawRequest(context.Background(), "/any", []byte("{}")); err == nil {
			t.Errorf("raw accepted error status %d", code)
		}
		server.Close()
	}
	body := `{"choices":[{"message":{"content":"ok","tool_calls":[{"id":"x"}]}}],"big":9007199254740993,"extension":{"x":1}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
	defer server.Close()
	result, err := NewClient(server.URL, "dummy").Chat(context.Background(), ChatRequest{})
	if err != nil || string(result.Raw) != body {
		t.Fatalf("raw response lost: %s %v", result.Raw, err)
	}
}

func TestRedirectCredentials(t *testing.T) {
	for _, provider := range []string{"ant", "oa"} {
		t.Run(provider, func(t *testing.T) {
			var received atomic.Bool
			dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Store(true) }))
			defer dest.Close()
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, strings.Replace(dest.URL, "127.0.0.1", "localhost", 1), 307)
			}))
			defer source.Close()
			_, err := NewClient(source.URL, "must-not-leave-origin", provider).Chat(context.Background(), ChatRequest{})
			if err == nil || received.Load() {
				t.Fatalf("redirect was not rejected: %v received=%v", err, received.Load())
			}
		})
	}
}

func TestProviderSelectionAndPagination(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("x-api-key") != "dummy" || r.Header.Get("Authorization") != "" {
			t.Error("wrong authentication")
		}
		if calls.Load() == 1 {
			io.WriteString(w, `{"data":[{"id":"a"}],"has_more":true,"last_id":"a"}`)
		} else {
			if r.URL.Query().Get("after_id") != "a" {
				t.Error("cursor missing")
			}
			io.WriteString(w, `{"data":[{"id":"b","display_name":"B","max_input_tokens":1000}],"has_more":false}`)
		}
	}))
	defer server.Close()
	models, err := NewClient(server.URL, "dummy", "ant").ListModels(context.Background())
	if err != nil || len(models) != 2 || calls.Load() != 2 || models[1].ContextLength != 1000 {
		t.Fatalf("models=%v calls=%d err=%v", models, calls.Load(), err)
	}
	if NewClient("https://example.invalid/api.anthropic.com", "dummy").isAnthropicURL() {
		t.Fatal("protocol inferred from path")
	}
	if !NewClient("http://proxy.invalid/v1", "dummy", "ant").isAnthropicURL() {
		t.Fatal("explicit protocol lost")
	}
}

func TestAnthropicOptions(t *testing.T) {
	top := 0.8
	cap := 100
	budget := &ThinkingConfig{Type: "enabled", BudgetTokens: 1024}
	parts := []ContentPart{{Type: "text", Text: "describe"}, {Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,YWJj"}}, {Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/a.png"}}}
	req, err := convertToAnthropicReq(ChatRequest{TopP: &top, Messages: []Message{{Role: "user", Content: parts}}}, false)
	if err != nil || req.TopP == nil || *req.TopP != top {
		t.Fatalf("top-p lost: %+v %v", req, err)
	}
	encoded, _ := json.Marshal(req)
	if strings.Contains(string(encoded), "image_url") || !strings.Contains(string(encoded), `"source":{"data":"YWJj","media_type":"image/png","type":"base64"}`) {
		t.Fatalf("image conversion: %s", encoded)
	}
	for _, invalid := range []ChatRequest{
		{MaxTokens: &cap, Thinking: budget},
		{ResponseFormat: &ResponseFormat{Type: "json_object"}},
		{Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 100}},
	} {
		if _, err := convertToAnthropicReq(invalid, false); err == nil {
			t.Errorf("accepted invalid request %+v", invalid)
		}
	}
}

func TestRequestValidationAndGatewayNormalization(t *testing.T) {
	a, b := 10, 20
	temp := 0.7
	top := 2.0
	for _, req := range []ChatRequest{{MaxTokens: &a, MaxCompletionTokens: &b}, {Model: "o3-mini", Temperature: &temp}, {TopP: &top}, {ReasoningEffort: "banana"}, {ReasoningEffort: "high", Thinking: &ThinkingConfig{BudgetTokens: 1024}}} {
		if err := NewClient("https://example.invalid", "dummy").prepareRequest(&req); err == nil {
			t.Errorf("accepted invalid request %+v", req)
		}
	}
	req := ChatRequest{Model: "o3-mini", MaxTokens: &a}
	if err := NewClient("https://example.invalid", "dummy").prepareRequest(&req); err != nil || req.MaxTokens != nil || *req.MaxCompletionTokens != 10 {
		t.Fatalf("o-model normalization: %+v %v", req, err)
	}
	req = ChatRequest{Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 1500}}
	if err := NewClient("http://proxy.invalid", "dummy", "or").prepareRequest(&req); err != nil || req.Reasoning == nil || req.Reasoning.MaxTokens != 1500 || req.Thinking != nil {
		t.Fatalf("gateway normalization: %+v %v", req, err)
	}
}

func TestAnthropicStopAndIdleTimeout(t *testing.T) {
	for _, stop := range []bool{true, false} {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			if stop {
				io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		c := NewClient(server.URL, "dummy", "ant")
		c.StreamIdleTimeout = 30 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := c.StreamChat(ctx, ChatRequest{}, func(StreamChunk) error { return nil })
		cancel()
		close(release)
		server.Close()
		if stop && err != nil {
			t.Fatalf("did not stop: %v", err)
		}
		if !stop && (err == nil || !strings.Contains(err.Error(), "idle timeout")) {
			t.Fatalf("idle timer failed: %v", err)
		}
	}
}
