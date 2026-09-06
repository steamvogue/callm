package main

import (
	"callm/internal/client"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUserAgentCLI(t *testing.T) {
	for _, mode := range []struct {
		name string
		args []string
	}{
		{"chat", []string{"--no-stdin", "--no-stream", "hello"}},
		{"stream", []string{"--no-stdin", "--stream", "hello"}},
		{"anthropic", []string{"--ant", "--no-stdin", "--no-stream", "hello"}},
		{"anthropic stream", []string{"--ant", "--no-stdin", "--stream", "hello"}},
		{"models", []string{"models"}},
		{"info", []string{"info", "example"}},
		{"raw", []string{"raw", "/endpoint", "{}"}},
	} {
		for _, tc := range []struct {
			name, env, want string
			flags           []string
			invalid         bool
		}{
			{"default", "", client.DefaultUserAgent, nil, false},
			{"environment", "Team/2.0", "Team/2.0", nil, false},
			{"flag wins", "Team/2.0", "models", []string{"--user-agent", "models"}, false},
			{"omit", "Team/2.0", "", []string{"--user-agent="}, false},
			{"invalid", "", "", []string{"--user-agent", "bad\r\nX-Injected: true"}, true},
		} {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if got := r.Header.Get("User-Agent"); got != tc.want {
						t.Errorf("user agent=%q want=%q", got, tc.want)
					}
					if tc.want == "" {
						if _, ok := r.Header["User-Agent"]; ok {
							t.Error("empty user agent was sent")
						}
					}
					io.Copy(io.Discard, r.Body)
					switch mode.name {
					case "models", "info":
						io.WriteString(w, `{"data":[{"id":"example"}]}`)
					case "anthropic":
						io.WriteString(w, `{"content":[{"type":"text","text":"answer"}]}`)
					case "anthropic stream":
						w.Header().Set("Content-Type", "text/event-stream")
						io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
					case "stream":
						w.Header().Set("Content-Type", "text/event-stream")
						io.WriteString(w, "data: [DONE]\n\n")
					default:
						io.WriteString(w, `{"choices":[{"message":{"content":"answer"}}]}`)
					}
				}))
				defer server.Close()
				args := append([]string{"--api", server.URL, "--api-key", "dummy"}, tc.flags...)
				cmd := testCLI(t, append(args, mode.args...)...)
				cmd.Env = append(cmd.Env, "CALLM_USER_AGENT="+tc.env, "GORACE=atexit_sleep_ms=0")
				out, err := cmd.CombinedOutput()
				if tc.invalid {
					if err == nil || calls.Load() != 0 || !strings.Contains(string(out), "user-agent must not contain control characters") {
						t.Fatalf("invalid header: calls=%d err=%v out=%s", calls.Load(), err, out)
					}
				} else if err != nil || calls.Load() != 1 {
					t.Fatalf("calls=%d err=%v out=%s", calls.Load(), err, out)
				}
			})
		}
	}
}
