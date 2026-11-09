package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Trimmed from a live Straitly response: tools run upstream, so the choice ends
// with "stop" and carries url_citation annotations rather than tool_calls.
const straitlyWebResponse = `{"id":"x","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"answer","annotations":[{"type":"url_citation","url_citation":{"url":"https://go.dev/dl/","title":"All releases","start_index":0,"end_index":0}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12,"cost":0.00721766}}`

func webServer(t *testing.T, body string, stream bool, seen *atomic.Value) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		if seen != nil {
			seen.Store(data)
		}
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func runWeb(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := testCLI(t, append(args, "prompt")...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestWebToolsTextOutput(t *testing.T) {
	var seen atomic.Value
	server := webServer(t, straitlyWebResponse, false, &seen)
	stdout, stderr, err := runWeb(t, "--st", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--no-stream", "--strict", "--stats", "--web-search", "--web-fetch")
	if err != nil {
		t.Fatalf("%v: %s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "answer" {
		t.Fatalf("stdout=%q", stdout)
	}
	for _, want := range []string{"Sources:", "All releases", "https://go.dev/dl/", "cost: $0.007218"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q: %s", want, stderr)
		}
	}
	var request struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(seen.Load().([]byte), &request); err != nil {
		t.Fatal(err)
	}
	if len(request.Tools) != 2 || request.Tools[0]["type"] != "web_search" || request.Tools[1]["type"] != "web_fetch" {
		t.Fatalf("tools=%v", request.Tools)
	}
}

func TestWebToolsResultJSON(t *testing.T) {
	server := webServer(t, straitlyWebResponse, false, nil)
	stdout, stderr, err := runWeb(t, "--st", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--result-json", "--web-search")
	if err != nil {
		t.Fatalf("%v: %s", err, stderr)
	}
	var result struct {
		Status  string `json:"status"`
		Sources []struct {
			URL string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	if result.Status != "ok" || len(result.Sources) != 1 || result.Sources[0].URL != "https://go.dev/dl/" {
		t.Fatalf("result=%+v", result)
	}
}

func TestWebToolsStream(t *testing.T) {
	body := `data: {"choices":[{"index":0,"delta":{"content":"answer"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{"annotations":[{"type":"url_citation","url_citation":{"url":"https://go.dev/dl/","title":"All releases"}}]},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`
	server := webServer(t, body, true, nil)
	stdout, stderr, err := runWeb(t, "--st", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--stream", "--strict", "--web-search")
	if err != nil {
		t.Fatalf("%v: %s", err, stderr)
	}
	if strings.TrimSpace(stdout) != "answer" || !strings.Contains(stderr, "https://go.dev/dl/") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestWebToolRecordsNeedRequest(t *testing.T) {
	body := `{"choices":[{"message":{"content":"answer","tool_calls":[{"id":"1","type":"function","function":{"name":"web_search","arguments":"{}"}}]},"finish_reason":"stop"}]}`
	server := webServer(t, body, false, nil)
	base := []string{"--st", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--no-stream", "--strict"}
	if _, stderr, err := runWeb(t, append(base, "--web-search")...); err != nil {
		t.Fatalf("requested web record rejected: %v: %s", err, stderr)
	}
	if _, stderr, err := runWeb(t, base...); err == nil || !strings.Contains(stderr, "unsupported tool calls") {
		t.Fatalf("unrequested web record: err=%v stderr=%s", err, stderr)
	}
}

func TestWebToolsWithoutFlagsOmitTools(t *testing.T) {
	var seen atomic.Value
	server := webServer(t, straitlyWebResponse, false, &seen)
	_, stderr, err := runWeb(t, "--st", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--no-stream")
	if err != nil {
		t.Fatalf("%v: %s", err, stderr)
	}
	if strings.Contains(stderr, "Sources:") || bytes.Contains(seen.Load().([]byte), []byte(`"tools"`)) {
		t.Fatalf("stderr=%s body=%s", stderr, seen.Load())
	}
}

func TestWebToolsUnsupportedProviderMakesNoRequest(t *testing.T) {
	for _, preset := range []string{"--or", "--orca", "--ds", "--ant", "--oa", "--ollama"} {
		t.Run(preset, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
			defer server.Close()
			_, stderr, err := runWeb(t, preset, "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--web-search")
			if err == nil || !strings.Contains(stderr, "--web-search is not supported") {
				t.Fatalf("err=%v stderr=%s", err, stderr)
			}
			if requests.Load() != 0 {
				t.Fatal("unsupported request reached server")
			}
		})
	}
}
