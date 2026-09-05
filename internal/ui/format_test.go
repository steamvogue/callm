package ui

import (
	"bytes"
	"strings"
	"testing"

	"callm/internal/client"
)

func TestStreamRendererInlineThinking(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	r := NewStreamRenderer(&out, &errOut, true, false)
	r.IsTTY = false // disable ANSI escapes in test

	chunks := []client.StreamDelta{
		{Content: "Hello! <th"},
		{Content: "ink>\nLet me calculate 2+2.\n"},
		{Content: "2+2=4.\n</think>\nThe answer is 4."},
	}

	for _, c := range chunks {
		r.HandleDelta(c)
	}
	r.Finish()

	errText := errOut.String()
	outText := out.String()

	if !strings.Contains(errText, "Let me calculate 2+2.") || !strings.Contains(errText, "2+2=4.") {
		t.Fatalf("expected reasoning in errOut, got: %q", errText)
	}
	if strings.Contains(outText, "calculate") {
		t.Fatalf("reasoning leaked into stdout: %q", outText)
	}
	if !strings.Contains(outText, "Hello!") || !strings.Contains(outText, "The answer is 4.") {
		t.Fatalf("expected content in outText, got: %q", outText)
	}
}

func TestStreamRendererNoReasoning(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	r := NewStreamRenderer(&out, &errOut, false, false)
	r.IsTTY = false

	chunks := []client.StreamDelta{
		{Content: "<think>Secret thoughts</think>Public answer"},
	}

	for _, c := range chunks {
		r.HandleDelta(c)
	}
	r.Finish()

	if errOut.Len() > 0 {
		t.Fatalf("expected no stderr output, got: %q", errOut.String())
	}
	if strings.Contains(out.String(), "Secret") {
		t.Fatalf("suppressed reasoning leaked into stdout: %q", out.String())
	}
	if !strings.Contains(out.String(), "Public answer") {
		t.Fatalf("expected 'Public answer' in stdout, got: %q", out.String())
	}
}

func TestStreamRendererOnlyReasoning(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	r := NewStreamRenderer(&out, &errOut, true, true)
	r.IsTTY = false

	chunks := []client.StreamDelta{
		{Content: "<think>Only this</think>Not this"},
	}

	for _, c := range chunks {
		r.HandleDelta(c)
	}
	r.Finish()

	if !strings.Contains(errOut.String(), "Only this") {
		t.Fatalf("expected reasoning in stderr, got: %q", errOut.String())
	}
	if strings.Contains(out.String(), "Not this") {
		t.Fatalf("content should be suppressed when onlyReasoning=true, got: %q", out.String())
	}
}

func TestStreamRendererDeltaFields(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	r := NewStreamRenderer(&out, &errOut, true, false)
	r.IsTTY = false

	chunks := []client.StreamDelta{
		{ReasoningContent: "Step 1\n"},
		{Reasoning: "Step 2\n"},
		{Thought: "Step 3\n"},
		{Content: "Final result."},
	}

	for _, c := range chunks {
		r.HandleDelta(c)
	}
	r.Finish()

	errText := errOut.String()
	outText := out.String()

	if !strings.Contains(errText, "Step 1") || !strings.Contains(errText, "Step 2") || !strings.Contains(errText, "Step 3") {
		t.Fatalf("expected all reasoning fields in errOut, got: %q", errText)
	}
	if !strings.Contains(outText, "Final result.") {
		t.Fatalf("expected 'Final result.' in outText, got: %q", outText)
	}
}
