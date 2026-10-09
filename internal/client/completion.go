package client

import (
	"fmt"
	"strings"
)

// Completion tracks result semantics independently of HTTP/SSE success.
// It retains only metadata; the renderer owns streamed text.
type Completion struct {
	ChoiceSeen   bool
	HasContent   bool
	HasReasoning bool
	HasTools     bool
	Refusal      string
	FinishReason string
}

func (s *Completion) Add(choice StreamChoice) {
	s.ChoiceSeen = true
	d := choice.Delta
	s.HasContent = s.HasContent || strings.TrimSpace(d.Content) != ""
	s.HasReasoning = s.HasReasoning || strings.TrimSpace(d.Reasoning+d.ReasoningContent+d.Thought) != ""
	s.HasTools = s.HasTools || len(d.ToolCalls) > 0 || len(d.FunctionCall) > 0 && strings.TrimSpace(string(d.FunctionCall)) != "null"
	s.Refusal += d.Refusal
	if choice.FinishReason != nil {
		s.FinishReason = *choice.FinishReason
	}
}

func (choice ChatChoice) Completion() Completion {
	s := Completion{}
	m := choice.Message
	s.Add(StreamChoice{Delta: StreamDelta{Content: m.Content, Reasoning: m.Reasoning, ReasoningContent: m.ReasoningContent, Thought: m.Thought, Refusal: m.Refusal, ToolCalls: m.ToolCalls, FunctionCall: m.FunctionCall}, FinishReason: &choice.FinishReason})
	return s
}

// Validate rejects known unsuccessful results. Strict also requires a recognized
// terminal reason; permissive missing reasons support older compatible proxies.
func (s Completion) Validate(strict, allowEmpty, onlyReasoning bool) error {
	if !s.ChoiceSeen {
		return fmt.Errorf("response has no completion choice")
	}
	if s.Refusal != "" {
		return fmt.Errorf("model refused the request")
	}
	switch s.FinishReason {
	case "length", "max_tokens":
		return fmt.Errorf("incomplete response: token limit reached (%s)", s.FinishReason)
	case "content_filter", "refusal":
		return fmt.Errorf("response refused or filtered (%s)", s.FinishReason)
	case "tool_calls", "function_call", "tool_use", "pause_turn":
		return fmt.Errorf("response requires unsupported tool/continuation handling (%s)", s.FinishReason)
	case "", "stop", "end_turn", "stop_sequence":
	default:
		return fmt.Errorf("unsupported completion reason %q", s.FinishReason)
	}
	if s.HasTools {
		return fmt.Errorf("response contains unsupported tool calls; use --json to inspect the envelope")
	}
	if strict && s.FinishReason == "" {
		return fmt.Errorf("response is missing a terminal finish/stop reason")
	}
	hasOutput := s.HasContent
	if onlyReasoning {
		hasOutput = s.HasReasoning
	}
	if !hasOutput && !allowEmpty {
		return fmt.Errorf("response has no usable output; use --allow-empty for an intentionally empty result")
	}
	return nil
}
