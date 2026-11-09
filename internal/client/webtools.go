package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Hosted web tools run on the provider, which returns the finished answer.
const (
	WebSearchTool = "web_search"
	WebFetchTool  = "web_fetch"
)

// webToolWire maps a provider to the hosted tools it executes and their request
// form. Providers use different request shapes and continuation protocols, so
// add an entry only after checking that provider's live response.
var webToolWire = map[string]map[string]string{
	"st": {
		WebSearchTool: `{"type":"web_search"}`,
		WebFetchTool:  `{"type":"web_fetch"}`,
	},
}

// WebToolDefinitions returns the request tools for a provider, or an error
// naming the first tool that provider cannot run.
func WebToolDefinitions(provider string, names ...string) ([]json.RawMessage, error) {
	definitions := make([]json.RawMessage, 0, len(names))
	for _, name := range names {
		wire, ok := webToolWire[provider][name]
		if !ok {
			flag := "--" + strings.ReplaceAll(name, "_", "-")
			return nil, fmt.Errorf("%s is not supported by this provider; use --st (Straitly), or callm raw for provider-specific tools", flag)
		}
		definitions = append(definitions, json.RawMessage(wire))
	}
	if len(definitions) == 0 {
		return nil, nil
	}
	return definitions, nil
}

type toolCallKind int

const (
	otherToolCall toolCallKind = iota
	webToolCall
	toolCallContinuation
)

// classifyToolCall identifies hosted web tools. Later stream fragments carry an
// index without a name and take the kind of the call they continue.
func classifyToolCall(raw json.RawMessage) toolCallKind {
	var call struct {
		Index    *int   `json:"index"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &call) != nil {
		return otherToolCall
	}
	for _, name := range []string{call.Type, call.Name, call.Function.Name} {
		if name == WebSearchTool || name == WebFetchTool {
			return webToolCall
		}
	}
	if call.Index != nil && call.Type == "" && call.Name == "" && call.Function.Name == "" {
		return toolCallContinuation
	}
	return otherToolCall
}

// Source is a page the provider reported for a web-tool answer.
type Source struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// AppendSources adds url_citation annotations, skipping repeated URLs. Other
// annotation forms are ignored so they cannot fail an otherwise valid response.
func AppendSources(sources []Source, annotations json.RawMessage) []Source {
	var parsed []json.RawMessage
	if json.Unmarshal(annotations, &parsed) != nil {
		return sources
	}
	for _, raw := range parsed {
		var annotation struct {
			URLCitation struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"url_citation"`
		}
		if json.Unmarshal(raw, &annotation) != nil || annotation.URLCitation.URL == "" {
			continue
		}
		duplicate := false
		for _, existing := range sources {
			duplicate = duplicate || existing.URL == annotation.URLCitation.URL
		}
		if !duplicate {
			sources = append(sources, Source{URL: annotation.URLCitation.URL, Title: annotation.URLCitation.Title})
		}
	}
	return sources
}
