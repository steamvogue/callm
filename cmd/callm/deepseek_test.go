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

// Local mocks validate the DeepSeek preset contract; these do not call the live API.
func TestDeepSeekCLI(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		check func(t *testing.T, body map[string]interface{})
		want  string
	}{
		{"default model", []string{"--ds", "--no-stdin", "--no-stream", "prompt"}, func(t *testing.T, body map[string]interface{}) {
			if body["model"] != "deepseek-flash" {
				t.Errorf("model: %v", body["model"])
			}
			if _, ok := body["thinking"]; ok {
				t.Errorf("unexpected thinking switch: %v", body["thinking"])
			}
		}, "answer"},
		{"reasoning display", []string{"--ds", "--no-stdin", "--no-stream", "--reasoning", "prompt"}, nil, "analysis"},
		{"model override", []string{"--ds", "-m", "deepseek-v4-pro", "--no-stdin", "--no-stream", "prompt"}, func(t *testing.T, body map[string]interface{}) {
			if body["model"] != "deepseek-v4-pro" {
				t.Errorf("model: %v", body["model"])
			}
		}, "answer"},
		{"effort", []string{"--ds", "--effort", "low", "--no-stdin", "--no-stream", "prompt"}, func(t *testing.T, body map[string]interface{}) {
			if body["reasoning_effort"] != "low" || body["reasoning"] != nil || body["include_reasoning"] != nil {
				t.Errorf("effort payload: %v", body)
			}
		}, "answer"},
		{"raw thinking switch", []string{"--ds", "raw", "/chat/completions", `{"model":"deepseek-flash","thinking":{"type":"disabled"},"messages":[]}`}, func(t *testing.T, body map[string]interface{}) {
			thinking, _ := body["thinking"].(map[string]interface{})
			if body["model"] != "deepseek-flash" || thinking["type"] != "disabled" {
				t.Errorf("raw body: %v", body)
			}
		}, "answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer ds-dummy" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if tc.check != nil {
					tc.check(t, body)
				}
				io.WriteString(w, `{"choices":[{"message":{"content":"answer","reasoning_content":"analysis"}}]}`)
			}))
			defer server.Close()
			cmd := testCLI(t, append([]string{"--api", server.URL}, tc.args...)...)
			// Isolate configuration files and inherited provider/model overrides.
			cmd.Dir = t.TempDir()
			cmd.Env = []string{"CALLM_TIMEOUT_TEST_HELPER=1", "HOME=" + t.TempDir(), "DEEPSEEK_API_KEY=ds-dummy"}
			out, err := cmd.CombinedOutput()
			if err != nil || calls.Load() != 1 || !strings.Contains(string(out), tc.want) {
				t.Fatalf("calls=%d err=%v output=%s", calls.Load(), err, out)
			}
		})
	}
}

// DeepSeek thinking is toggled by the API's thinking switch, not a token budget.
func TestDeepSeekThinkingBudgetRejected(t *testing.T) {
	out, err := testCLI(t, "--ds", "--api", "http://127.0.0.1:1", "--api-key", "dummy", "--no-stdin", "--thinking-budget", "2048", "prompt").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "thinking-budget requires") {
		t.Fatalf("err=%v output=%s", err, out)
	}
}
