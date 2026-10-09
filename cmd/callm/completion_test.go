package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompletionExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, body, diagnostic string
		flags                  []string
		valid                  bool
	}{
		{"null tool fields", `{"choices":[{"message":{"content":"ok","function_call":null,"tool_calls":null},"finish_reason":"stop"}]}`, "", nil, true},
		{"complete", `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`, "", nil, true},
		{"length", `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, "token limit", nil, false},
		{"empty shape", `{}`, "no completion", nil, false},
		{"refusal", `{"choices":[{"message":{"content":null,"refusal":"denied"},"finish_reason":"stop"}]}`, "refused", nil, false},
		{"filtered", `{"choices":[{"message":{"content":null},"finish_reason":"content_filter"}]}`, "filtered", nil, false},
		{"tools", `{"choices":[{"message":{"content":"pending","tool_calls":[{"id":"a"}]},"finish_reason":"stop"}]}`, "tool", nil, false},
		{"empty", `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`, "no usable", nil, false},
		{"allowed empty", `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`, "", []string{"--strict", "--allow-empty"}, true},
		{"allow empty still truncated", `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`, "token limit", []string{"--allow-empty"}, false},
		{"legacy missing reason", `{"choices":[{"message":{"content":"ok"}}]}`, "", nil, true},
		{"strict missing reason", `{"choices":[{"message":{"content":"ok"}}]}`, "terminal", []string{"--strict"}, false},
		{"json diagnostics", `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, "", []string{"--json"}, true},
		{"json strict", `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, "token limit", []string{"--json", "--strict"}, false},
		{"reasoning only", `{"choices":[{"message":{"content":"","reasoning":"steps"},"finish_reason":"stop"}]}`, "", []string{"--only-reasoning"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tc.body) }))
			defer server.Close()
			args := []string{"--oa", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--no-stream"}
			args = append(args, tc.flags...)
			args = append(args, "prompt")
			cmd := testCLI(t, args...)
			var out, errOut bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errOut
			err := cmd.Run()
			if (err == nil) != tc.valid || (!tc.valid && !strings.Contains(errOut.String(), tc.diagnostic)) {
				t.Fatalf("error=%v stdout=%q stderr=%q", err, out.String(), errOut.String())
			}
			if !tc.valid && out.Len() != 0 {
				t.Fatalf("invalid buffered result published: %q", out.String())
			}
		})
	}
}

func TestStreamingCompletionExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, provider, body, want string
		valid                      bool
	}{
		{"complete", "oa", "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", "", true},
		{"truncated", "oa", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n", "token limit", false},
		{"done only", "oa", "data: [DONE]\n\n", "no completion", false},
		{"anthropic truncation", "ant", "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"partial\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", "token limit", false},
		{"anthropic tools", "ant", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\"}}\n\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", "tool", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			cmd := testCLI(t, "--"+tc.provider, "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--strict", "--stream", "prompt")
			var errOut bytes.Buffer
			cmd.Stderr = &errOut
			err := cmd.Run()
			if (err == nil) != tc.valid || (!tc.valid && !strings.Contains(errOut.String(), tc.want)) {
				t.Fatalf("error=%v stderr=%s", err, errOut.String())
			}
		})
	}
}

func TestParsedReasoningCompletion(t *testing.T) {
	for _, flags := range [][]string{{"--only-reasoning"}, {"--no-reasoning"}, {"--only-reasoning", "--stream"}} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(flags) == 2 {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"<think>steps</think>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			} else {
				io.WriteString(w, `{"choices":[{"message":{"content":"<think>steps</think>"},"finish_reason":"stop"}]}`)
			}
		}))
		args := []string{"--oa", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--parse-think", "--strict"}
		args = append(args, flags...)
		args = append(args, "prompt")
		output, err := testCLI(t, args...).CombinedOutput()
		server.Close()
		if flags[0] == "--only-reasoning" {
			if err != nil || !strings.Contains(string(output), "steps") {
				t.Fatalf("%v %s", err, output)
			}
		} else if err == nil {
			t.Fatalf("hidden-only result accepted: %s", output)
		}
	}
}
