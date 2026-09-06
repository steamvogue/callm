package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Exercise the public CLI with OrcaRouter's documented OpenAI-compatible shapes.
func TestOrcaCLI(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		args                           []string
		model, method, path, key, want string
		stats, stream                  bool
	}{
		{"chat", []string{"--orca", "chat", "--no-stdin", "--no-stream", "--stats", "--effort", "high", "--temp", "0.5", "--top-p", "0.8", "--max-tokens", "200", "--json-object", "prompt"}, "orcarouter/auto", "POST", "/v1/chat/completions", "orca-dummy", "$0.125000", true, false},
		{"stream", []string{"chat", "--orca", "--no-stdin", "--stream", "--stats", "prompt"}, "orcarouter/auto", "POST", "/v1/chat/completions", "orca-dummy", "$0.125000", true, true},
		{"claude", []string{"--orca", "--claude", "--no-stdin", "--no-stream", "--effort", "high", "prompt"}, "anthropic/claude-sonnet-4.6", "POST", "/v1/chat/completions", "orca-dummy", "answer", false, false},
		{"model override", []string{"--orca", "-m", "openai/gpt-4o-mini", "--no-stdin", "--json", "--stats", "prompt"}, "openai/gpt-4o-mini", "POST", "/v1/chat/completions", "orca-dummy", `"cost_usd":"0.125"`, true, false},
		{"models", []string{"--orca", "models"}, "", "GET", "/v1/models", "orca-dummy", "openai/gpt-4o-mini", false, false},
		{"info", []string{"info", "--orca", "openai/gpt-4o-mini"}, "", "GET", "/v1/models", "orca-dummy", "openai/gpt-4o-mini", false, false},
		{"raw", []string{"--orca", "raw", "/chat/completions", `{"model":"custom","messages":[]}`}, "custom", "POST", "/v1/chat/completions", "orca-dummy", "answer", false, false},
		{"openrouter unchanged", []string{"--or", "--no-stdin", "--no-stream", "prompt"}, "deepseek/deepseek-v4-flash-0731", "POST", "/v1/chat/completions", "openrouter-dummy", "answer", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != tc.method || r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer "+tc.key || r.Header.Get("x-api-key") != "" {
					t.Errorf("wrong request: %s %s", r.Method, r.URL.Path)
				}
				if (r.Header.Get("X-OrcaRouter-Include-Cost") == "true") != tc.stats {
					t.Error("cost opt-in differs from --stats")
				}
				if r.Method == "GET" {
					io.WriteString(w, `{"data":[{"id":"openai/gpt-4o-mini","context_length":128000,"architecture":{"input_modalities":["text","image"],"output_modalities":["text"]}}]}`)
					return
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != tc.model {
					t.Errorf("model: %v", body["model"])
				}
				if tc.name == "chat" || tc.name == "claude" {
					if body["reasoning_effort"] != "high" || body["reasoning"] != nil || body["thinking"] != nil {
						t.Errorf("reasoning shape: %v", body)
					}
				}
				if tc.name == "chat" && (body["temperature"] != 0.5 || body["top_p"] != 0.8 || body["max_tokens"] != float64(200) || body["response_format"].(map[string]interface{})["type"] != "json_object") {
					t.Errorf("options: %v", body)
				}
				if tc.stream {
					options, ok := body["stream_options"].(map[string]interface{})
					if body["stream"] != true || !ok || options["include_usage"] != true {
						t.Errorf("stream options: %v", body)
					}
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3,\"cost_usd\":0.125}}\n\ndata: [DONE]\n\n")
				} else {
					io.WriteString(w, `{"choices":[{"message":{"content":"answer"}}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"cost_usd":"0.125"}}`)
				}
			}))
			defer server.Close()
			cmd := testCLI(t, append([]string{"--api", server.URL + "/v1"}, tc.args...)...)
			// Nonempty values prevent .env files from supplying provider-specific credentials.
			cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "ORCA_API_KEY=orca-dummy", "OPENROUTER_API_KEY=openrouter-dummy", "CALLM_MODEL=")
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), tc.want) || calls.Load() != 1 {
				t.Fatalf("calls=%d err=%v output=%s", calls.Load(), err, out)
			}
		})
	}
}

func TestOrcaInvalidOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--or"}, "select only one provider"},
		{[]string{"--thinking-budget", "1024"}, "use --effort for OrcaRouter"},
	} {
		args := append([]string{"--orca", "--api", "http://127.0.0.1:1/v1", "--api-key", "dummy", "--no-stdin", "--no-stream"}, tc.args...)
		out, err := testCLI(t, append(args, "prompt")...).CombinedOutput()
		if err == nil || !strings.Contains(string(out), tc.want) {
			t.Fatalf("err=%v output=%s", err, out)
		}
	}
}
