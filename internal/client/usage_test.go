package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicCacheUsage(t *testing.T) {
	for _, stream := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"cache_read_input_tokens\":100,\"cache_creation_input_tokens\":20}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"answer\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
			} else {
				io.WriteString(w, `{"content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":20}}`)
			}
		}))
		c := NewClient(server.URL, "dummy", "ant")
		var u *Usage
		var err error
		if stream {
			u, err = c.StreamChat(context.Background(), ChatRequest{}, func(StreamChunk) error { return nil })
		} else {
			var result *ChatResponse
			result, err = c.Chat(context.Background(), ChatRequest{})
			if result != nil {
				u = result.Usage
			}
		}
		server.Close()
		if err != nil || u == nil || u.PromptTokens != 130 || u.TotalTokens != 135 || u.CompletionTokens != 5 || u.UncachedInputTokens != 10 || u.CacheReadInputTokens != 100 || u.CacheCreationInputTokens != 20 {
			t.Fatalf("stream=%v usage=%+v error=%v", stream, u, err)
		}
	}
}
func TestOpenAIUsageDoesNotAddCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":130,"completion_tokens":5,"total_tokens":135,"cache_read_input_tokens":100}}`)
	}))
	defer server.Close()
	r, err := NewClient(server.URL, "dummy").Chat(context.Background(), ChatRequest{})
	if err != nil || r.Usage.PromptTokens != 130 || r.Usage.TotalTokens != 135 {
		t.Fatalf("result=%+v error=%v", r, err)
	}
}
