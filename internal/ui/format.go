package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"callm/internal/client"
)

// IsTerminal returns true if the writer is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil {
			return (stat.Mode() & os.ModeCharDevice) != 0
		}
	}
	return false
}

// StreamRenderer manages visual output of reasoning and content tokens.
type StreamRenderer struct {
	Out           io.Writer
	Err           io.Writer
	IsTTY         bool
	ShowReasoning bool
	OnlyReasoning bool
	InReasoning    bool
	HasReasoned    bool
	ContentStarted bool
	inThinkBlock   bool
	tagBuf         string
}

// NewStreamRenderer creates a renderer for streaming output.
func NewStreamRenderer(out, err io.Writer, showReasoning, onlyReasoning bool) *StreamRenderer {
	return &StreamRenderer{
		Out:           out,
		Err:           err,
		IsTTY:         IsTerminal(out),
		ShowReasoning: showReasoning,
		OnlyReasoning: onlyReasoning,
	}
}

func (r *StreamRenderer) emitReasoning(text string) {
	if text == "" || !r.ShowReasoning {
		return
	}
	if !r.InReasoning {
		r.InReasoning = true
		r.HasReasoned = true
		if r.IsTTY {
			fmt.Fprint(r.Err, "\033[2m\033[36m[Thinking...]\n")
		}
	}
	fmt.Fprint(r.Err, text)
}

func (r *StreamRenderer) emitContent(text string) {
	if text == "" || r.OnlyReasoning {
		return
	}
	if r.InReasoning {
		r.InReasoning = false
		if r.IsTTY {
			fmt.Fprint(r.Err, "\033[0m\n\n")
		} else {
			fmt.Fprint(r.Err, "\n\n")
		}
	}
	r.ContentStarted = true
	fmt.Fprint(r.Out, text)
}

func findPartialPrefixSuffix(s, target string) int {
	maxCheck := len(target) - 1
	if len(s) < maxCheck {
		maxCheck = len(s)
	}
	for i := maxCheck; i >= 1; i-- {
		prefix := target[:i]
		if strings.HasSuffix(s, prefix) {
			return len(s) - i
		}
	}
	return -1
}

// HandleDelta renders reasoning and content tokens cleanly.
func (r *StreamRenderer) HandleDelta(delta client.StreamDelta) {
	// 1. Check direct JSON delta reasoning fields (DeepSeek, OpenRouter, Anthropic, Qwen, Gemini)
	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}
	if reasoning == "" {
		reasoning = delta.Thought
	}

	if reasoning != "" {
		r.emitReasoning(reasoning)
	}

	// 2. Process delta.Content for inline <think>...</think> tags (Ollama, local vLLM, QwQ)
	if delta.Content != "" {
		r.tagBuf += delta.Content

		for len(r.tagBuf) > 0 {
			if r.inThinkBlock {
				idx := strings.Index(r.tagBuf, "</think>")
				if idx != -1 {
					r.emitReasoning(r.tagBuf[:idx])
					r.tagBuf = r.tagBuf[idx+len("</think>"):]
					r.inThinkBlock = false
					continue
				}

				if pIdx := findPartialPrefixSuffix(r.tagBuf, "</think>"); pIdx != -1 {
					r.emitReasoning(r.tagBuf[:pIdx])
					r.tagBuf = r.tagBuf[pIdx:]
					break
				}

				r.emitReasoning(r.tagBuf)
				r.tagBuf = ""
				break
			} else {
				idx := strings.Index(r.tagBuf, "<think>")
				if idx != -1 {
					r.emitContent(r.tagBuf[:idx])
					r.tagBuf = r.tagBuf[idx+len("<think>"):]
					r.inThinkBlock = true
					continue
				}

				if pIdx := findPartialPrefixSuffix(r.tagBuf, "<think>"); pIdx != -1 {
					r.emitContent(r.tagBuf[:pIdx])
					r.tagBuf = r.tagBuf[pIdx:]
					break
				}

				r.emitContent(r.tagBuf)
				r.tagBuf = ""
				break
			}
		}
	}
}

// Finish ensures all styles are reset and newlines flushed.
func (r *StreamRenderer) Finish() {
	if r.tagBuf != "" {
		if r.inThinkBlock {
			r.emitReasoning(r.tagBuf)
		} else {
			r.emitContent(r.tagBuf)
		}
		r.tagBuf = ""
	}
	if r.InReasoning {
		if r.IsTTY {
			fmt.Fprint(r.Err, "\033[0m\n")
		} else {
			fmt.Fprintln(r.Err)
		}
	}
	if r.ContentStarted {
		fmt.Fprintln(r.Out)
	}
}

// PrintStats prints performance and cost metrics to stderr.
func PrintStats(err io.Writer, duration time.Duration, usage *client.Usage, model string) {
	isTTY := IsTerminal(err)
	var promptTok, compTok, totalTok int
	var costStr string

	if usage != nil {
		promptTok = usage.PromptTokens
		compTok = usage.CompletionTokens
		totalTok = usage.TotalTokens
		cost := usage.GetCostFloat()
		if cost > 0 {
			costStr = fmt.Sprintf(" | cost: $%.6f", cost)
		}
	}

	var speedStr string
	if compTok > 0 && duration.Seconds() > 0 {
		tokPerSec := float64(compTok) / duration.Seconds()
		speedStr = fmt.Sprintf(" | %.1f tok/s", tokPerSec)
	}

	statsText := fmt.Sprintf("[stats: %v | %d tokens (%d in / %d out)%s%s | model: %s]",
		duration.Round(time.Millisecond), totalTok, promptTok, compTok, speedStr, costStr, model)

	if isTTY {
		fmt.Fprintf(err, "\033[90m%s\033[0m\n", statsText)
	} else {
		fmt.Fprintln(err, statsText)
	}
}
