package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testCLI(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestTimeoutCLIHelper$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "CALLM_TIMEOUT_TEST_HELPER=1")
	return cmd
}

func TestGlobalSubcommandDispatch(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		command string
		rest    string
	}{
		{[]string{"--oa", "models", "gpt"}, "models", "--oa gpt"},
		{[]string{"--api", "http://localhost", "--oa", "info", "gpt"}, "info", "--api http://localhost --oa gpt"},
		{[]string{"--system", "models", "hello"}, "chat", "--system models hello"},
		{[]string{"--", "models"}, "chat", "-- models"},
	} {
		name, rest := splitCommand(tc.args)
		if name != tc.command || strings.Join(rest, " ") != tc.rest {
			t.Fatalf("%v => %s %v", tc.args, name, rest)
		}
	}
}

func TestStdinWaitAndDeadline(t *testing.T) {
	old := os.Stdin
	defer func() { os.Stdin = old }()
	for _, delayed := range []bool{true, false} {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = read
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		if delayed {
			go func() { time.Sleep(20 * time.Millisecond); write.WriteString("late context"); write.Close() }()
		}
		got, err := readStdinIfAvailable(ctx)
		cancel()
		read.Close()
		write.Close()
		if delayed && (err != nil || got != "late context") {
			t.Fatalf("delayed stdin: %q %v", got, err)
		}
		if !delayed && err == nil {
			t.Fatal("open stdin did not time out")
		}
	}
}

func TestCLIOutputAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		valid bool
		want  string
	}{
		{"json stats", []string{"--json", "--stats"}, true, `"extension":42`},
		{"inline hidden", []string{"--no-stream", "--no-reasoning"}, true, "answer"},
		{"invalid temperature", []string{"--temp", "0.7junk"}, false, ""},
		{"invalid top-p", []string{"--top-p", "2"}, false, ""},
		{"conflicting providers", []string{"--or", "--st"}, false, ""},
		{"conflicting tokens", []string{"--max-tokens", "10", "--max-completion-tokens", "20"}, false, ""},
		{"conflicting stream", []string{"--stream", "--no-stream"}, false, ""},
		{"conflicting reasoning", []string{"--only-reasoning", "--no-reasoning"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				io.Copy(io.Discard, r.Body)
				io.WriteString(w, `{"choices":[{"message":{"content":"<think>secret</think>answer"}}],"extension":42,"usage":{"total_tokens":3,"cost":"0.125"}}`)
			}))
			defer server.Close()
			args := []string{"--api", server.URL, "--api-key", "dummy", "--no-stdin"}
			args = append(args, tc.flags...)
			args = append(args, "prompt")
			out, err := testCLI(t, args...).CombinedOutput()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v output=%s", tc.valid, err, out)
			}
			if tc.valid {
				if !strings.Contains(string(out), tc.want) {
					t.Fatalf("output=%s", out)
				}
				if tc.name == "json stats" && !strings.Contains(string(out), "[stats:") {
					t.Fatal("JSON stats lost")
				}
				if tc.name == "inline hidden" && strings.Contains(string(out), "secret") {
					t.Fatal("reasoning leaked")
				}
			} else if requests.Load() != 0 {
				t.Fatal("invalid request reached server")
			}
		})
	}
}

func TestCLIIndependentTimeouts(t *testing.T) {
	for _, knob := range []string{"header-timeout", "idle-timeout", "stdin-timeout"} {
		t.Run(knob, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				if knob == "idle-timeout" {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			cmd := testCLI(t, "--api", server.URL, "--api-key", "dummy", "--stream", "--timeout", "2", "--"+knob, "0.03", "prompt")
			if knob == "stdin-timeout" {
				pipe, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				defer pipe.Close()
			}
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("%s did not fire: %s", knob, out)
			}
			expected := map[string]string{"header-timeout": "timeout awaiting response headers", "idle-timeout": "idle timeout", "stdin-timeout": "stdin: context deadline exceeded"}[knob]
			if !strings.Contains(string(out), expected) {
				t.Fatalf("%s: %s", knob, out)
			}
		})
	}
}

func TestCLIAnthropicProxy(t *testing.T) {
	seen := make(chan map[string]interface{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" || r.Header.Get("x-api-key") != "dummy" {
			t.Error("wrong protocol for explicit Anthropic proxy")
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		seen <- body
		io.WriteString(w, `{"content":[{"type":"text","text":"answer"}],"usage":{"input_tokens":1,"output_tokens":1},"provider_extension":true}`)
	}))
	defer server.Close()
	out, err := testCLI(t, "--ant", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--no-stream", "--top-p", "0.8", "prompt").CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	body := <-seen
	if body["top_p"] != 0.8 || body["model"] != "claude-sonnet-4-6" {
		t.Fatal(body)
	}
}
