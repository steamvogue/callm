package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebToolDefinitionsProviderMap(t *testing.T) {
	definitions, err := WebToolDefinitions("st", WebSearchTool, WebFetchTool)
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 2 || string(definitions[0]) != `{"type":"web_search"}` || string(definitions[1]) != `{"type":"web_fetch"}` {
		t.Fatalf("definitions=%s", definitions)
	}
	if definitions, err := WebToolDefinitions("or"); err != nil || definitions != nil {
		t.Fatalf("no tools requested: %v %v", definitions, err)
	}
	for _, provider := range []string{"", "or", "orca", "ds", "ant", "ms", "kimi", "zai", "qw", "oa", "groq", "pool", "ollama"} {
		_, err := WebToolDefinitions(provider, WebFetchTool)
		if err == nil || !strings.Contains(err.Error(), "--web-fetch") || !strings.Contains(err.Error(), "--st") {
			t.Errorf("provider %q: err=%v", provider, err)
		}
	}
}

func TestCompletionWebToolValidation(t *testing.T) {
	web := json.RawMessage(`{"id":"1","type":"function","function":{"name":"web_search","arguments":"{}"}}`)
	custom := json.RawMessage(`{"id":"2","type":"function","function":{"name":"lookup","arguments":"{}"}}`)
	fragment := json.RawMessage(`{"index":0,"function":{"arguments":"{}"}}`)
	stop, toolCalls := "stop", "tool_calls"
	for _, tc := range []struct {
		name   string
		allow  bool
		calls  []json.RawMessage
		finish *string
		valid  bool
	}{
		{"no tool records with web tools on", true, nil, &stop, true},
		{"requested web record", true, []json.RawMessage{web}, &stop, true},
		{"web record with stream fragment", true, []json.RawMessage{web, fragment}, &stop, true},
		{"web record not requested", false, []json.RawMessage{web}, &stop, false},
		{"custom tool alongside web", true, []json.RawMessage{web, custom}, &stop, false},
		{"unnamed fragment alone", true, []json.RawMessage{fragment}, &stop, false},
		{"unfinished web call", true, []json.RawMessage{web}, &toolCalls, false},
		{"anthropic placeholder", true, []json.RawMessage{json.RawMessage(`{}`)}, &stop, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			completion := Completion{AllowWebTools: tc.allow}
			completion.Add(StreamChoice{Delta: StreamDelta{Content: "answer", ToolCalls: tc.calls}, FinishReason: tc.finish})
			if err := completion.Validate(true, false, false); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestAppendSources(t *testing.T) {
	first := json.RawMessage(`[{"type":"url_citation","url_citation":{"url":"https://a.example/","title":"A"}},{"type":"other"},"text",{"type":"url_citation","url_citation":{"url":"https://b.example/"}}]`)
	sources := AppendSources(nil, first)
	sources = AppendSources(sources, json.RawMessage(`[{"url_citation":{"url":"https://a.example/","title":"again"}}]`))
	if len(sources) != 2 || sources[0] != (Source{URL: "https://a.example/", Title: "A"}) || sources[1].URL != "https://b.example/" {
		t.Fatalf("sources=%+v", sources)
	}
	for _, invalid := range []string{"", "null", `{"url":"x"}`, `"x"`} {
		if got := AppendSources(nil, json.RawMessage(invalid)); len(got) != 0 {
			t.Errorf("%q: %+v", invalid, got)
		}
	}
}
