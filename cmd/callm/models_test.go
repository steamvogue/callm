package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const modelsCatalog = `{"data":[
{"id":"deepseek/deepseek-v4-flash","name":"DeepSeek V4 Flash","context_length":128000,
 "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
 "pricing":{"prompt":"0.00000027","completion":"0.0000011"},
 "supported_parameters":["tools","reasoning"],"provider_extension":true},
{"id":"z-ai/glm-5.3","context_length":200000},
{"id":"qwen/qwen3.8-max","display_name":"Qwen 3.8 Max","context_length":1000000}
]}`

func runModelsCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := testCLI(t, args...)
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"CALLM_TIMEOUT_TEST_HELPER=1", "HOME=" + t.TempDir()}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestModelsFormatsAndFilter(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer dummy" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(modelsCatalog))
	}))
	defer server.Close()
	base := []string{"models", "--api", server.URL, "--api-key", "dummy"}

	t.Run("json raw fidelity", func(t *testing.T) {
		stdout, stderr, err := runModelsCLI(t, append(base, "--format=json", "--filter=deepseek,z.ai")...)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		if !strings.Contains(stdout, `"provider_extension": true`) {
			t.Fatalf("raw provider field dropped: %s", stdout)
		}
		var entries []map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
			t.Fatalf("json: %v: %s", err, stdout)
		}
		if len(entries) != 2 || entries[0]["id"] != "deepseek/deepseek-v4-flash" || entries[1]["id"] != "z-ai/glm-5.3" {
			t.Fatalf("entries=%v", entries)
		}
	})

	t.Run("json alias", func(t *testing.T) {
		stdout, _, err := runModelsCLI(t, append(base, "--json", "--filter=qwen")...)
		if err != nil || !strings.Contains(stdout, `"qwen/qwen3.8-max"`) || strings.Contains(stdout, "deepseek") {
			t.Fatalf("err=%v stdout=%s", err, stdout)
		}
	})

	t.Run("zed", func(t *testing.T) {
		stdout, stderr, err := runModelsCLI(t, append(base, "--format=zed", "--filter=deepseek")...)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		var settings struct {
			LanguageModels struct {
				OpenAICompatible map[string]struct {
					APIURL          string `json:"api_url"`
					AvailableModels []struct {
						Name         string `json:"name"`
						MaxTokens    int64  `json:"max_tokens"`
						Capabilities *struct {
							Images bool `json:"images"`
						} `json:"capabilities"`
					} `json:"available_models"`
				} `json:"openai_compatible"`
			} `json:"language_models"`
		}
		if err := json.Unmarshal([]byte(stdout), &settings); err != nil {
			t.Fatalf("json: %v: %s", err, stdout)
		}
		provider, ok := settings.LanguageModels.OpenAICompatible["127-0-0-1"]
		if !ok || provider.APIURL != server.URL || len(provider.AvailableModels) != 1 {
			t.Fatalf("provider=%+v", settings.LanguageModels.OpenAICompatible)
		}
		model := provider.AvailableModels[0]
		if model.Name != "deepseek/deepseek-v4-flash" || model.MaxTokens != 128000 || model.Capabilities == nil || !model.Capabilities.Images {
			t.Fatalf("model=%+v", model)
		}
	})

	t.Run("kilo", func(t *testing.T) {
		stdout, stderr, err := runModelsCLI(t, append(base, "--format=kilo", "--filter=deepseek")...)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		var config struct {
			Provider map[string]struct {
				NPM     string            `json:"npm"`
				Options map[string]string `json:"options"`
				Models  map[string]struct {
					Reasoning bool `json:"reasoning"`
					Limit     *struct {
						Context int64 `json:"context"`
					} `json:"limit"`
					Cost *struct {
						Input float64 `json:"input"`
					} `json:"cost"`
				} `json:"models"`
			} `json:"provider"`
		}
		if err := json.Unmarshal([]byte(stdout), &config); err != nil {
			t.Fatalf("json: %v: %s", err, stdout)
		}
		provider, ok := config.Provider["127-0-0-1"]
		if !ok || provider.NPM != "@ai-sdk/openai-compatible" || provider.Options["baseURL"] != server.URL {
			t.Fatalf("provider=%+v", config.Provider)
		}
		model, ok := provider.Models["deepseek/deepseek-v4-flash"]
		if !ok || !model.Reasoning || model.Limit == nil || model.Limit.Context != 128000 || model.Cost == nil || model.Cost.Input != 0.27 {
			t.Fatalf("model=%+v", model)
		}
	})

	t.Run("kilo provider name", func(t *testing.T) {
		stdout, _, err := runModelsCLI(t, append(base, "--format=kilo", "--provider-name", "My Gateway")...)
		if err != nil || !strings.Contains(stdout, `"My Gateway"`) || !strings.Contains(stdout, `"my-gateway"`) {
			t.Fatalf("err=%v stdout=%s", err, stdout)
		}
	})

	t.Run("continue", func(t *testing.T) {
		stdout, stderr, err := runModelsCLI(t, append(base, "--format=continue")...)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		if !strings.Contains(stderr, "hint:") {
			t.Fatalf("missing key hint: %s", stderr)
		}
		var config struct {
			Models []struct {
				Name     string `json:"name"`
				Provider string `json:"provider"`
				APIBase  string `json:"apiBase"`
			} `json:"models"`
		}
		if err := json.Unmarshal([]byte(stdout), &config); err != nil {
			t.Fatalf("json: %v: %s", err, stdout)
		}
		if len(config.Models) != 3 || config.Models[0].Provider != "openai" || config.Models[0].APIBase != server.URL {
			t.Fatalf("models=%+v", config.Models)
		}
	})

	t.Run("table filter", func(t *testing.T) {
		stdout, _, err := runModelsCLI(t, append(base, "--format=table", "--filter=qwen")...)
		if err != nil || !strings.Contains(stdout, "qwen/qwen3.8-max") || strings.Contains(stdout, "deepseek") {
			t.Fatalf("err=%v stdout=%s", err, stdout)
		}
	})

	t.Run("conflicting output modes", func(t *testing.T) {
		_, stderr, err := runModelsCLI(t, append(base, "--json", "--format=kilo")...)
		if err == nil || !strings.Contains(stderr, "conflicts") {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
	})

	t.Run("unknown format", func(t *testing.T) {
		before := requests.Load()
		_, stderr, err := runModelsCLI(t, append(base, "--format=zedd")...)
		if err == nil || !strings.Contains(stderr, "unknown format") {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		if requests.Load() != before {
			t.Fatal("invalid format reached the API")
		}
	})

	t.Run("invalid positional regex", func(t *testing.T) {
		_, stderr, err := runModelsCLI(t, append(base, "[")...)
		if err == nil || !strings.Contains(stderr, "invalid filter regex") {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
	})

	if requests.Load() == 0 {
		t.Fatal("no catalog requests were made")
	}
}

func TestModelsHelpFlags(t *testing.T) {
	output, err := testCLI(t, "models", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v: %s", err, output)
	}
	for _, want := range []string{"-filter", "-format", "-json", "-provider-name", "z.ai matches z-ai"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("models help lacks %q: %s", want, output)
		}
	}
}

func TestZedMissingContextWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"no-context"}]}`))
	}))
	defer server.Close()
	stdout, stderr, err := runModelsCLI(t, "models", "--api", server.URL, "--api-key", "dummy", "--format=zed")
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, stderr)
	}
	if !strings.Contains(stderr, "warning:") || !strings.Contains(stderr, "max_tokens omitted") {
		t.Fatalf("stderr=%s", stderr)
	}
	if !strings.Contains(stdout, `"name": "no-context"`) {
		t.Fatalf("stdout=%s", stdout)
	}
}
