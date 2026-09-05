package ui

import (
	"fmt"
	"io"
	"os"
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
	InReasoning   bool
	HasReasoned   bool
	ContentStarted bool
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

// HandleDelta renders reasoning and content tokens cleanly.
func (r *StreamRenderer) HandleDelta(delta client.StreamDelta) {
	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}

	// Handle Reasoning tokens
	if reasoning != "" && r.ShowReasoning {
		if !r.InReasoning {
			r.InReasoning = true
			r.HasReasoned = true
			if r.IsTTY {
				fmt.Fprint(r.Err, "\033[2m\033[36m[Thinking...]\n")
			}
		}
		if r.IsTTY {
			fmt.Fprint(r.Err, reasoning)
		} else {
			fmt.Fprint(r.Err, reasoning)
		}
	}

	// If only reasoning was requested, don't output content
	if r.OnlyReasoning {
		return
	}

	// Handle Content tokens
	if delta.Content != "" {
		if r.InReasoning {
			r.InReasoning = false
			if r.IsTTY {
				fmt.Fprint(r.Err, "\033[0m\n\n")
			} else {
				fmt.Fprint(r.Err, "\n\n")
			}
		}
		r.ContentStarted = true
		fmt.Fprint(r.Out, delta.Content)
	}
}

// Finish ensures all styles are reset and newlines flushed.
func (r *StreamRenderer) Finish() {
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
