package ui

import (
	"callm/internal/client"
	"errors"
	"io"
	"testing"
	"time"
)

var errDisk = errors.New("synthetic disk failure")

type failingWriter struct {
	remaining int
	short     bool
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.remaining >= len(p) {
		w.remaining -= len(p)
		return len(p), nil
	}
	if w.short {
		return 0, nil
	}
	return 0, errDisk
}

type flushWriter struct{}

func (flushWriter) Write(p []byte) (int, error) { return len(p), nil }
func (flushWriter) Flush() error                { return errDisk }
func TestRendererWriteFailures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		out, stderr io.Writer
		delta       client.StreamDelta
		finish      bool
		want        error
	}{
		{"content", &failingWriter{}, io.Discard, client.StreamDelta{Content: "answer"}, false, errDisk},
		{"reasoning", io.Discard, &failingWriter{}, client.StreamDelta{Reasoning: "steps"}, false, errDisk},
		{"short", &failingWriter{short: true}, io.Discard, client.StreamDelta{Content: "answer"}, false, io.ErrShortWrite},
		{"newline", &failingWriter{remaining: 6}, io.Discard, client.StreamDelta{Content: "answer"}, true, errDisk},
		{"flush", flushWriter{}, io.Discard, client.StreamDelta{Content: "answer"}, true, errDisk},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewStreamRenderer(tc.out, tc.stderr, true, false)
			err := r.HandleDelta(tc.delta)
			if tc.finish {
				if err != nil {
					t.Fatal(err)
				}
				err = r.Finish()
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
func TestAuxiliaryWriteFailures(t *testing.T) {
	for _, fn := range []func() error{
		func() error { return PrintStats(&failingWriter{}, time.Second, nil, "model") },
		func() error { return PrintModelInfo(&failingWriter{}, client.ModelInfo{ID: "model"}) },
		func() error { return PrintModelsTable(&failingWriter{}, []client.ModelInfo{{ID: "model"}}, "") },
	} {
		if err := fn(); !errors.Is(err, errDisk) {
			t.Fatalf("error=%v", err)
		}
	}
}
