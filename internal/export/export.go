// Package export renders model catalogs as raw JSON or as ready-to-paste
// provider configuration fragments for editors and coding agents.
package export

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"callm/internal/client"
)

// Supported formats. Render handles the non-table formats; "table" is resolved
// by the caller before rendering.
const (
	FormatTable    = "table"
	FormatJSON     = "json"
	FormatZed      = "zed"
	FormatKilo     = "kilo"
	FormatContinue = "continue"
)

// Meta carries the active endpoint details needed by provider config snippets.
type Meta struct {
	BaseURL      string
	ProviderID   string
	ProviderName string
	KeyEnv       string
}

// NormalizeFormat maps user input to a supported format name. An empty value
// selects the default provider target (zed).
func NormalizeFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", FormatZed:
		return FormatZed, nil
	case FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatKilo:
		return FormatKilo, nil
	case FormatContinue, "vscode", "vs_code", "vs-code":
		return FormatContinue, nil
	default:
		return "", fmt.Errorf("unknown format %q (want table, json, zed, kilo, or continue; vscode is an alias for continue)", value)
	}
}

// Slug converts a display name or host into a lowercase config identifier.
func Slug(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// Render serializes models in the requested format. Warnings are non-fatal
// notes intended for stderr; only invalid input returns an error.
func Render(format string, models []client.ModelInfo, meta Meta) ([]byte, []string, error) {
	switch format {
	case FormatJSON:
		data, err := renderJSON(models)
		return data, nil, err
	case FormatZed:
		return renderZed(models, meta)
	case FormatKilo:
		return renderKilo(models, meta)
	case FormatContinue:
		return renderContinue(models, meta)
	default:
		return nil, nil, fmt.Errorf("unsupported format %q", format)
	}
}

func marshalIndent(value interface{}) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func renderJSON(models []client.ModelInfo) ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(models))
	for _, model := range models {
		if len(model.Raw) > 0 {
			raw = append(raw, json.RawMessage(append([]byte(nil), model.Raw...)))
			continue
		}
		encoded, err := json.Marshal(model)
		if err != nil {
			return nil, err
		}
		raw = append(raw, encoded)
	}
	return marshalIndent(raw)
}

func displayName(model client.ModelInfo) string {
	for _, candidate := range []string{model.DisplayName, model.Name} {
		if candidate != "" && candidate != model.ID {
			return candidate
		}
	}
	return ""
}

func supportsImages(model client.ModelInfo) bool {
	if model.Architecture == nil {
		return false
	}
	for _, modality := range model.Architecture.InputModalities {
		if strings.EqualFold(modality, "image") {
			return true
		}
	}
	return false
}

func supportsParameter(model client.ModelInfo, names ...string) bool {
	for _, parameter := range model.SupportedParameters {
		for _, name := range names {
			if strings.EqualFold(parameter, name) {
				return true
			}
		}
	}
	return false
}

func supportsReasoning(model client.ModelInfo) bool {
	return supportsParameter(model, "reasoning", "include_reasoning", "reasoning_effort", "thinking")
}

func supportsTools(model client.ModelInfo) bool {
	return supportsParameter(model, "tools", "tool_choice")
}

// pricePerMillion converts per-token pricing to USD per million tokens.
func pricePerMillion(value interface{}) (float64, bool) {
	if value == nil {
		return 0, false
	}
	var price float64
	switch v := value.(type) {
	case float64:
		price = v
	case float32:
		price = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		price = parsed
	default:
		return 0, false
	}
	if math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
		return 0, false
	}
	return price * 1000000, true
}

type zedSettings struct {
	LanguageModels zedLanguageModels `json:"language_models"`
}

type zedLanguageModels struct {
	OpenAICompatible map[string]zedProvider `json:"openai_compatible"`
}

type zedProvider struct {
	APIURL          string     `json:"api_url"`
	AvailableModels []zedModel `json:"available_models"`
}

type zedModel struct {
	Name         string           `json:"name"`
	DisplayName  string           `json:"display_name,omitempty"`
	MaxTokens    int64            `json:"max_tokens,omitempty"`
	Capabilities *zedCapabilities `json:"capabilities,omitempty"`
}

type zedCapabilities struct {
	Images bool `json:"images,omitempty"`
}

func renderZed(models []client.ModelInfo, meta Meta) ([]byte, []string, error) {
	var warnings []string
	entries := make([]zedModel, 0, len(models))
	for _, model := range models {
		entry := zedModel{
			Name:        model.ID,
			DisplayName: displayName(model),
			MaxTokens:   model.ContextLength,
		}
		if entry.MaxTokens <= 0 {
			warnings = append(warnings, fmt.Sprintf("model %q has no context length; zed max_tokens omitted", model.ID))
		}
		if supportsImages(model) {
			entry.Capabilities = &zedCapabilities{Images: true}
		}
		entries = append(entries, entry)
	}

	settings := zedSettings{}
	settings.LanguageModels.OpenAICompatible = map[string]zedProvider{
		meta.ProviderID: {APIURL: meta.BaseURL, AvailableModels: entries},
	}
	data, err := marshalIndent(settings)
	return data, warnings, err
}

type kiloConfig struct {
	Provider map[string]kiloProvider `json:"provider"`
}

type kiloProvider struct {
	Name    string               `json:"name,omitempty"`
	NPM     string               `json:"npm"`
	Options map[string]string    `json:"options"`
	Models  map[string]kiloModel `json:"models"`
}

type kiloModel struct {
	Name       string          `json:"name,omitempty"`
	Reasoning  bool            `json:"reasoning,omitempty"`
	Attachment bool            `json:"attachment,omitempty"`
	ToolCall   bool            `json:"tool_call,omitempty"`
	Cost       *kiloCost       `json:"cost,omitempty"`
	Limit      *kiloLimit      `json:"limit,omitempty"`
	Modalities *kiloModalities `json:"modalities,omitempty"`
}

type kiloCost struct {
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}

type kiloLimit struct {
	Context int64 `json:"context,omitempty"`
	Output  int64 `json:"output,omitempty"`
}

type kiloModalities struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

func kiloCostOf(model client.ModelInfo) *kiloCost {
	if model.Pricing == nil {
		return nil
	}
	cost := &kiloCost{}
	found := false
	if price, ok := pricePerMillion(model.Pricing.Prompt); ok {
		cost.Input = &price
		found = true
	}
	if price, ok := pricePerMillion(model.Pricing.Completion); ok {
		cost.Output = &price
		found = true
	}
	if price, ok := pricePerMillion(model.Pricing.InputCacheRead); ok {
		cost.CacheRead = &price
		found = true
	}
	if price, ok := pricePerMillion(model.Pricing.InputCacheWrite); ok {
		cost.CacheWrite = &price
		found = true
	}
	if !found {
		return nil
	}
	return cost
}

func renderKilo(models []client.ModelInfo, meta Meta) ([]byte, []string, error) {
	providerModels := make(map[string]kiloModel, len(models))
	for _, model := range models {
		entry := kiloModel{
			Name:       displayName(model),
			Reasoning:  supportsReasoning(model),
			Attachment: supportsImages(model),
			ToolCall:   supportsTools(model),
			Cost:       kiloCostOf(model),
		}
		if model.ContextLength > 0 {
			entry.Limit = &kiloLimit{Context: model.ContextLength}
		}
		if model.Architecture != nil && (len(model.Architecture.InputModalities) > 0 || len(model.Architecture.OutputModalities) > 0) {
			entry.Modalities = &kiloModalities{
				Input:  model.Architecture.InputModalities,
				Output: model.Architecture.OutputModalities,
			}
		}
		providerModels[model.ID] = entry
	}

	config := kiloConfig{Provider: map[string]kiloProvider{
		meta.ProviderID: {
			Name:    providerNameOrID(meta),
			NPM:     "@ai-sdk/openai-compatible",
			Options: map[string]string{"baseURL": meta.BaseURL},
			Models:  providerModels,
		},
	}}
	data, err := marshalIndent(config)
	return data, nil, err
}

func providerNameOrID(meta Meta) string {
	if meta.ProviderName != "" {
		return meta.ProviderName
	}
	return meta.ProviderID
}

type continueConfig struct {
	Models []continueModel `json:"models"`
}

type continueModel struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIBase  string `json:"apiBase"`
}

func renderContinue(models []client.ModelInfo, meta Meta) ([]byte, []string, error) {
	entries := make([]continueModel, 0, len(models))
	for _, model := range models {
		entries = append(entries, continueModel{
			Name:     model.ID,
			Provider: "openai",
			Model:    model.ID,
			APIBase:  meta.BaseURL,
		})
	}
	data, err := marshalIndent(continueConfig{Models: entries})
	return data, nil, err
}
