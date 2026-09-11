package export

import (
	"encoding/json"
	"strings"
	"testing"

	"callm/internal/client"
)

func sampleModels() []client.ModelInfo {
	return []client.ModelInfo{
		{
			ID:            "deepseek/deepseek-v4-flash",
			DisplayName:   "DeepSeek V4 Flash",
			ContextLength: 128000,
			Architecture: &client.Architecture{
				InputModalities:  []string{"text", "image"},
				OutputModalities: []string{"text"},
			},
			Pricing:             &client.ModelPricing{Prompt: "0.00000027", Completion: 0.0000011},
			SupportedParameters: []string{"tools", "reasoning"},
		},
		{ID: "plain"},
	}
}

func TestNormalizeFormat(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", FormatZed},
		{"zed", FormatZed},
		{"table", FormatTable},
		{"JSON", FormatJSON},
		{"Kilo", FormatKilo},
		{"continue", FormatContinue},
		{"vscode", FormatContinue},
		{"vs_code", FormatContinue},
	} {
		got, err := NormalizeFormat(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeFormat(%q)=%q,%v want %q", tc.in, got, err, tc.want)
		}
	}
	if _, err := NormalizeFormat("zedd"); err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Straitly", "straitly"},
		{"DeepSeek Direct", "deepseek-direct"},
		{"api.example.com", "api-example-com"},
		{"My Gateway!", "my-gateway"},
	} {
		if got := Slug(tc.in); got != tc.want {
			t.Fatalf("Slug(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderJSONPreservesProviderFields(t *testing.T) {
	models := []client.ModelInfo{{ID: "a", Raw: json.RawMessage(`{"id":"a","provider_extension":true}`)}}
	data, warnings, err := Render(FormatJSON, models, Meta{})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	if !strings.Contains(string(data), `"provider_extension": true`) {
		t.Fatalf("raw fields dropped: %s", data)
	}
	var entries []map[string]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if entries[0]["id"] != "a" {
		t.Fatalf("entry=%v", entries[0])
	}
}

func TestRenderJSONEmptyCatalog(t *testing.T) {
	data, _, err := Render(FormatJSON, nil, Meta{})
	if err != nil || string(data) != "[]\n" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestRenderZed(t *testing.T) {
	data, warnings, err := Render(FormatZed, sampleModels(), Meta{BaseURL: "https://api.example.com/v1", ProviderID: "straitly"})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"plain"`) {
		t.Fatalf("warnings=%v", warnings)
	}
	var settings zedSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	provider := settings.LanguageModels.OpenAICompatible["straitly"]
	if provider.APIURL != "https://api.example.com/v1" || len(provider.AvailableModels) != 2 {
		t.Fatalf("provider=%+v", provider)
	}
	first := provider.AvailableModels[0]
	if first.Name != "deepseek/deepseek-v4-flash" || first.DisplayName != "DeepSeek V4 Flash" ||
		first.MaxTokens != 128000 || first.Capabilities == nil || !first.Capabilities.Images {
		t.Fatalf("first=%+v", first)
	}
	if provider.AvailableModels[1].MaxTokens != 0 {
		t.Fatalf("missing context emitted max_tokens: %+v", provider.AvailableModels[1])
	}
}

func TestRenderKilo(t *testing.T) {
	data, warnings, err := Render(FormatKilo, sampleModels(), Meta{BaseURL: "https://api.example.com/v1", ProviderID: "straitly"})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	var config kiloConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	provider := config.Provider["straitly"]
	if provider.NPM != "@ai-sdk/openai-compatible" || provider.Options["baseURL"] != "https://api.example.com/v1" {
		t.Fatalf("provider=%+v", provider)
	}
	first := provider.Models["deepseek/deepseek-v4-flash"]
	if !first.Reasoning || !first.Attachment || !first.ToolCall || first.Limit == nil || first.Limit.Context != 128000 {
		t.Fatalf("first=%+v", first)
	}
	if first.Cost == nil || first.Cost.Input == nil || *first.Cost.Input != 0.27 || first.Cost.Output == nil || *first.Cost.Output != 1.1 {
		t.Fatalf("cost=%+v", first.Cost)
	}
	if first.Modalities == nil || strings.Join(first.Modalities.Input, ",") != "text,image" {
		t.Fatalf("modalities=%+v", first.Modalities)
	}
	second := provider.Models["plain"]
	if second.Limit != nil || second.Cost != nil || second.Name != "" {
		t.Fatalf("second=%+v", second)
	}
}

func TestRenderContinue(t *testing.T) {
	data, warnings, err := Render(FormatContinue, sampleModels(), Meta{BaseURL: "https://api.example.com/v1"})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err=%v warnings=%v", err, warnings)
	}
	var config continueConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Models) != 2 {
		t.Fatalf("models=%+v", config.Models)
	}
	first := config.Models[0]
	if first.Name != "deepseek/deepseek-v4-flash" || first.Provider != "openai" ||
		first.Model != first.Name || first.APIBase != "https://api.example.com/v1" {
		t.Fatalf("first=%+v", first)
	}
	if strings.Contains(string(data), "apiKey") {
		t.Fatalf("export must not carry keys: %s", data)
	}
}
