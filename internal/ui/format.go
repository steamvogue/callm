package ui

import (
	"errors"
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
	Out            io.Writer
	Err            io.Writer
	IsTTY          bool
	ShowReasoning  bool
	ParseThinking  bool
	OnlyReasoning  bool
	InReasoning    bool
	HasReasoned    bool
	ContentStarted bool
	HasContent     bool
	inThinkBlock   bool
	tagBuf         string
	writeErr       error
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

// writeText detects both ordinary errors and invalid short writes.
func writeText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	return err
}

func (r *StreamRenderer) write(w io.Writer, text string) {
	if r.writeErr == nil {
		r.writeErr = writeText(w, text)
	}
}

func (r *StreamRenderer) emitReasoning(text string) {
	if text == "" || !r.ShowReasoning {
		return
	}
	r.HasReasoned = r.HasReasoned || strings.TrimSpace(text) != ""
	if !r.InReasoning {
		r.InReasoning = true
		if r.IsTTY {
			r.write(r.Err, "\033[2m\033[36m[Thinking...]\n")
		}
	}
	r.write(r.Err, text)
}

func (r *StreamRenderer) emitContent(text string) {
	if text == "" || r.OnlyReasoning {
		return
	}
	if r.InReasoning {
		r.InReasoning = false
		if r.IsTTY {
			r.write(r.Err, "\033[0m\n\n")
		} else {
			r.write(r.Err, "\n\n")
		}
	}
	r.ContentStarted = true
	r.HasContent = r.HasContent || strings.TrimSpace(text) != ""
	r.write(r.Out, text)
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
func (r *StreamRenderer) HandleDelta(delta client.StreamDelta) error {
	if r.writeErr != nil {
		return r.writeErr
	}
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

	// Interpret inline thinking tags only when explicitly requested.
	if !r.ParseThinking {
		r.emitContent(delta.Content)
		return r.writeErr
	}
	// Process opted-in inline <think>...</think> tags.
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
	return r.writeErr
}

// Finish ensures all styles are reset and newlines flushed.
func (r *StreamRenderer) Finish() error {
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
			r.write(r.Err, "\033[0m\n")
		} else {
			r.write(r.Err, "\n")
		}
	}
	if r.ContentStarted {
		r.write(r.Out, "\n")
	}
	for _, w := range []io.Writer{r.Out, r.Err} {
		if f, ok := w.(interface{ Flush() error }); ok {
			r.writeErr = errors.Join(r.writeErr, f.Flush())
		}
	}
	return r.writeErr
}

// PrintStats prints performance and cost metrics to stderr.
func PrintStats(err io.Writer, duration time.Duration, usage *client.Usage, model string) error {
	isTTY := IsTerminal(err)
	var promptTok, compTok, totalTok int
	var costStr string

	if usage != nil {
		promptTok = usage.PromptTokens
		compTok = usage.CompletionTokens
		totalTok = usage.TotalTokens
		cost, available := usage.CostValue()
		if available {
			costStr = fmt.Sprintf(" | cost: $%.6f", cost)
		}
	}

	var speedStr string
	if compTok > 0 && duration.Seconds() > 0 {
		tokPerSec := float64(compTok) / duration.Seconds()
		speedStr = fmt.Sprintf(" | %.1f tok/s", tokPerSec)
	}

	tokens := "usage unavailable"
	if usage != nil {
		tokens = fmt.Sprintf("%d tokens (%d in / %d out)", totalTok, promptTok, compTok)
		if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
			tokens += fmt.Sprintf(" [cache read: %d / cache write: %d; included in input]", usage.CacheReadInputTokens, usage.CacheCreationInputTokens)
		}
	}
	statsText := fmt.Sprintf("[stats: %v | %s%s%s | model: %s]", duration.Round(time.Millisecond), tokens, speedStr, costStr, model)

	if isTTY {
		return writeText(err, "\033[90m"+statsText+"\033[0m\n")
	} else {
		return writeText(err, statsText+"\n")
	}
}
