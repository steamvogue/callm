package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPipelineInputAndSchemaProtocols(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	prompt := write("prompt.txt", "  unwrapped prompt\n")
	system := write("system.txt", "system\nverbatim")
	schema := write("schema.json", `{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"],"additionalProperties":false}`)
	for _, provider := range []string{"oa", "or", "ant"} {
		t.Run(provider, func(t *testing.T) {
			received := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				received <- request
				if provider == "ant" {
					io.WriteString(w, `{"model":"returned","content":[{"type":"text","text":"{\"n\":2}"}],"stop_reason":"end_turn"}`)
				} else {
					io.WriteString(w, `{"model":"returned","choices":[{"message":{"content":"{\"n\":2}"},"finish_reason":"stop"}]}`)
				}
			}))
			defer server.Close()
			out, err := testCLI(t, "--"+provider, "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--prompt-file", prompt, "--system-file", system, "--schema", schema, "--result-json").CombinedOutput()
			if err != nil {
				t.Fatalf("%v %s", err, out)
			}
			request := <-received
			var result map[string]any
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			if result["status"] != "ok" || result["schema_valid"] != true || result["returned_model"] != "returned" {
				t.Fatalf("%s", out)
			}
			if provider == "ant" {
				if request["system"] != "system\nverbatim" || request["output_config"].(map[string]any)["format"].(map[string]any)["type"] != "json_schema" {
					t.Fatalf("%+v", request)
				}
			} else {
				if provider == "or" && request["provider"].(map[string]any)["require_parameters"] != true {
					t.Fatalf("missing routing guard: %+v", request)
				}
				if provider == "oa" && request["provider"] != nil {
					t.Fatalf("routing options leaked: %+v", request)
				}
				if request["response_format"].(map[string]any)["json_schema"].(map[string]any)["strict"] != true {
					t.Fatalf("%+v", request)
				}
				if request["messages"].([]any)[0].(map[string]any)["content"] != "system\nverbatim" {
					t.Fatalf("%+v", request)
				}
			}
			messages := request["messages"].([]any)
			if messages[len(messages)-1].(map[string]any)["content"] != "  unwrapped prompt\n" {
				t.Fatalf("%+v", messages)
			}
		})
	}
}

func TestPipelineFailureOutputAndLocalOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, []byte(`{"type":"object","required":["n"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body   string
		result bool
		want   string
	}{
		{`{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`, false, "match schema"},
		{`{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`, true, "invalid_schema_output"},
		{`{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, true, "invalid_completion"},
		{`{"error":{"message":"fixture error"}}`, true, "request_failed"},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			if bytes.Contains(data, []byte("response_format")) {
				t.Error("local validation sent schema")
			}
			io.WriteString(w, tc.body)
		}))
		args := []string{"--oa", "--api", server.URL, "--api-key", "dummy", "--no-stdin", "--validate-schema", path}
		if tc.result {
			args = append(args, "--result-json")
		}
		args = append(args, "prompt")
		cmd := testCLI(t, args...)
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		server.Close()
		if err == nil {
			t.Fatal("failure accepted")
		}
		if tc.result {
			var result struct {
				Status string
				Error  struct{ Code string }
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("%v %s", err, out.String())
			}
			if result.Status != "error" || result.Error.Code != tc.want {
				t.Fatalf("%s", out.String())
			}
		} else if out.Len() != 0 || !strings.Contains(errOut.String(), tc.want) {
			t.Fatalf("stdout=%s stderr=%s", out.String(), errOut.String())
		}
	}
}

func TestPipelinePreflightAndRawBody(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "body.json")
	if err := os.WriteFile(file, []byte(`{"n":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	badSchema := filepath.Join(dir, "invalid-schema.json")
	if err := os.WriteFile(badSchema, []byte(`{"type":17}`), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"n":2}` {
			t.Errorf("%s", body)
		}
		io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	for _, from := range []string{file, "-"} {
		cmd := testCLI(t, "raw", "--oa", "--api", server.URL, "--api-key", "dummy", "--body-file", from, "/fixture")
		if from == "-" {
			cmd.Stdin = strings.NewReader(`{"n":2}`)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	before := calls.Load()
	for _, args := range [][]string{
		{"--max-input-bytes", "32", "--no-stdin", "long prompt taking up the request body"},
		{"--max-input-bytes", "64", "--no-stdin", "short"},                   // Input fits; serialized wire body does not.
		{"--max-input-bytes", "128", "--no-stdin", strings.Repeat("\"", 50)}, // JSON escaping counts.
		{"--system-file", file, "--system", "conflict", "prompt"},
		{"--schema", badSchema, "--no-stdin", "prompt"},
		{"--result-json", "--stream", "prompt"},
		{"--max-input-bytes", "67108865", "prompt"},
		{"raw", "--body-file", file, "/fixture", `{}`},
	} {
		base := []string{"--oa", "--api", server.URL, "--api-key", "dummy"}
		out, err := testCLI(t, append(base, args...)...).CombinedOutput()
		if err == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
	if calls.Load() != before {
		t.Fatal("preflight contacted provider")
	}
}

func TestRawBodyStdinDeadline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	cmd := testCLI(t, "raw", "--oa", "--api", "http://127.0.0.1:1", "--api-key", "dummy", "--body-file", "-", "--stdin-timeout", "20ms", "/fixture")
	cmd.Stdin = r
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err == nil || !strings.Contains(errOut.String(), "deadline") {
		t.Fatalf("%v %s", err, errOut.String())
	}
}
