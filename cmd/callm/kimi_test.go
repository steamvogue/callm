package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"callm/internal/client"
)

// Local mocks validate the CLI contract; these do not exercise a live subscription.
func TestKimiCLI(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		args                               []string
		model, key, method, endpoint, want string
		stream, stats                      bool
	}{
		{"chat", []string{"--kimi", "chat", "--no-stdin", "--no-stream", "--stats", "--reasoning", "prompt"}, "kimi-for-coding", "kimi-dummy", "POST", "/chat/completions", "analysis", false, true},
		{"stream", []string{"chat", "--kimi", "--no-stdin", "--stream", "--stats", "--reasoning", "prompt"}, "kimi-for-coding", "kimi-dummy", "POST", "/chat/completions", "analysis", true, true},
		{"model override", []string{"--kimi", "-m", "custom-model", "--no-stdin", "--json", "prompt"}, "custom-model", "kimi-dummy", "POST", "/chat/completions", `"extension":42`, false, false},
		{"key override", []string{"--kimi", "--api-key", "explicit", "--no-stdin", "--no-stream", "prompt"}, "kimi-for-coding", "explicit", "POST", "/chat/completions", "answer", false, false},
		{"models", []string{"--kimi", "models"}, "", "kimi-dummy", "GET", "/models", "kimi-for-coding", false, false},
		{"info", []string{"info", "--kimi", "kimi-for-coding"}, "", "kimi-dummy", "GET", "/models", "kimi-for-coding", false, false},
		{"raw", []string{"--kimi", "raw", "/chat/completions", `{"model":"custom-model","messages":[]}`}, "custom-model", "kimi-dummy", "POST", "/chat/completions", "answer", false, false},
		{"moonshot", []string{"--moonshot", "--no-stdin", "--no-stream", "prompt"}, "moonshot-v1-auto", "moonshot-dummy", "POST", "/chat/completions", "answer", false, false},
		{"ms", []string{"--ms", "--no-stdin", "--no-stream", "prompt"}, "moonshot-v1-auto", "moonshot-dummy", "POST", "/chat/completions", "answer", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != tc.method || r.URL.Path != "/coding/v1"+tc.endpoint || r.Header.Get("Authorization") != "Bearer "+tc.key {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("User-Agent") != client.DefaultUserAgent || r.Header.Get("x-api-key") != "" || r.Header.Get("X-OrcaRouter-Include-Cost") != "" {
					t.Error("unexpected identity, authentication, or cost headers")
				}
				if tc.method == "GET" {
					io.WriteString(w, `{"data":[{"id":"kimi-for-coding"}]}`)
					return
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != tc.model {
					t.Errorf("model: %v", body["model"])
				}
				if tc.stream {
					options, ok := body["stream_options"].(map[string]interface{})
					if body["stream"] != true || !ok || options["include_usage"] != true {
						t.Errorf("stream options: %v", body)
					}
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"analysis\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")
				} else {
					if body["stream"] == true {
						t.Error("unexpected streaming")
					}
					io.WriteString(w, `{"choices":[{"message":{"content":"answer","reasoning_content":"analysis"}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5},"extension":42}`)
				}
			}))
			defer server.Close()
			cmd := testCLI(t, append([]string{"--api", server.URL + "/coding/v1"}, tc.args...)...)
			// Isolate configuration files and inherited provider/model overrides.
			cmd.Dir = t.TempDir()
			cmd.Env = []string{"CALLM_TIMEOUT_TEST_HELPER=1", "HOME=" + t.TempDir(), "KIMI_API_KEY=kimi-dummy", "MOONSHOT_API_KEY=moonshot-dummy"}
			out, err := cmd.CombinedOutput()
			if err != nil || calls.Load() != 1 || !strings.Contains(string(out), tc.want) {
				t.Fatalf("calls=%d err=%v output=%s", calls.Load(), err, out)
			}
			if tc.stats && (!strings.Contains(string(out), "answer") || !strings.Contains(string(out), "5 tokens (2 in / 3 out)") || strings.Contains(string(out), "$")) {
				t.Fatalf("subscription stats: %s", out)
			}
		})
	}
}

func TestKimiInvalidOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--ms"}, "select only one provider"},
		{[]string{"--moonshot"}, "select only one provider"},
		{[]string{"--orca"}, "select only one provider"},
		{[]string{"--claude"}, "--claude requires"},
	} {
		args := append([]string{"--kimi", "--api", "http://127.0.0.1:1", "--api-key", "dummy", "--no-stdin"}, tc.args...)
		out, err := testCLI(t, append(args, "prompt")...).CombinedOutput()
		if err == nil || !strings.Contains(string(out), tc.want) {
			t.Fatalf("err=%v output=%s", err, out)
		}
	}
}
