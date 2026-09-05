package client

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type timeoutTransport func(*http.Request) (*http.Response, error)

func (f timeoutTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var timeoutOperations = []struct {
	name      string
	anthropic bool
	run       func(context.Context, *Client) error
}{
	{"chat", false, func(ctx context.Context, c *Client) error { _, err := c.Chat(ctx, ChatRequest{}); return err }},
	{"stream", false, func(ctx context.Context, c *Client) error {
		_, err := c.StreamChat(ctx, ChatRequest{}, func(StreamChunk) error { return nil })
		return err
	}},
	{"models", false, func(ctx context.Context, c *Client) error { _, err := c.ListModels(ctx); return err }},
	{"raw", false, func(ctx context.Context, c *Client) error {
		_, err := c.RawRequest(ctx, "/raw", []byte("{}"))
		return err
	}},
	{"anthropic-chat", true, func(ctx context.Context, c *Client) error { _, err := c.Chat(ctx, ChatRequest{}); return err }},
	{"anthropic-stream", true, func(ctx context.Context, c *Client) error {
		_, err := c.StreamChat(ctx, ChatRequest{}, func(StreamChunk) error { return nil })
		return err
	}},
}

func TestDefaultRequestTimeout(t *testing.T) {
	for _, op := range timeoutOperations {
		t.Run(op.name, func(t *testing.T) {
			base := "https://example.invalid/v1"
			if op.anthropic {
				base = "https://api.anthropic.com/v1"
			}
			provider := ""
			if op.anthropic {
				provider = "ant"
			}
			c := NewClient(base, "dummy", provider)
			called := false
			c.HTTPClient.Transport = timeoutTransport(func(r *http.Request) (*http.Response, error) {
				called = true
				deadline, ok := r.Context().Deadline()
				remaining := time.Until(deadline)
				if !ok || remaining < 299*time.Second || remaining > 300*time.Second {
					t.Errorf("expected a 300s request deadline, got deadline=%v remaining=%v", ok, remaining)
				}
				body := `{"choices":[],"data":[],"content":[]}`
				if r.Header.Get("Accept") == "text/event-stream" {
					body = "data: [DONE]\n\n"
					if op.anthropic {
						body = "data: {\"type\":\"message_stop\"}\n\n"
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			if err := op.run(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("configured HTTP transport was bypassed")
			}
		})
	}
}

func TestRequestTimeoutCoversHeadersAndBody(t *testing.T) {
	for _, op := range timeoutOperations {
		for _, phase := range []string{"headers", "body"} {
			t.Run(op.name+"/"+phase, func(t *testing.T) {
				release := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					if phase == "body" {
						w.Header().Set("Content-Type", "text/event-stream")
						w.WriteHeader(http.StatusOK)
						w.(http.Flusher).Flush()
					}
					select {
					case <-r.Context().Done():
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				base := server.URL
				if op.anthropic {
					base += "/api.anthropic.com"
				}
				provider := ""
				if op.anthropic {
					provider = "ant"
				}
				c := NewClient(base, "dummy", provider)
				c.HTTPClient.Timeout = 50 * time.Millisecond
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				start := time.Now()
				err := op.run(ctx, c)
				// Older supported Go versions expose HTTP timeouts through net.Error
				// without matching context.DeadlineExceeded through errors.Is.
				var timeoutErr net.Error
				if !errors.Is(err, context.DeadlineExceeded) && !(errors.As(err, &timeoutErr) && timeoutErr.Timeout()) {
					t.Fatalf("expected timeout error, got %v", err)
				}
				if elapsed := time.Since(start); elapsed > time.Second {
					t.Fatalf("configured timeout was ignored; elapsed %v", elapsed)
				}
			})
		}
	}
}

func TestDisabledTimeoutPreservesCallerDeadline(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		ctx := context.Background()
		if bounded {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Second)
			defer cancel()
		}
		c := NewClient("https://example.invalid/v1", "dummy")
		c.HTTPClient.Timeout = 0
		c.HTTPClient.Transport = timeoutTransport(func(r *http.Request) (*http.Response, error) {
			deadline, ok := r.Context().Deadline()
			if ok != bounded {
				t.Errorf("deadline present=%v, want %v", ok, bounded)
			}
			if expected, ok := ctx.Deadline(); ok && !deadline.Equal(expected) {
				t.Error("caller deadline changed when disabling client timeout")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[]}`)), Request: r}, nil
		})
		if _, err := c.Chat(ctx, ChatRequest{}); err != nil {
			t.Fatal(err)
		}
	}
}
