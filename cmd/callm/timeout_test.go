package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeoutValues(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
		valid bool
	}{
		{"300", 300 * time.Second, true}, {"5m", 300 * time.Second, true},
		{"600s", 600 * time.Second, true}, {"0.05", 50 * time.Millisecond, true},
		{"0", 0, true}, {"0s", 0, true}, {"-1", 0, false},
		{"-1s", 0, false}, {"300junk", 0, false}, {"", 0, false},
		{"999999999999999999999", 0, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			timeout := registerTimeout(fs)
			err := fs.Parse([]string{"--timeout=" + tc.value})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
			if tc.valid && *timeout != tc.want {
				t.Fatalf("got %v, want %v", *timeout, tc.want)
			}
		})
	}
}

func TestTimeoutCLIHelper(t *testing.T) {
	if os.Getenv("CALLM_TIMEOUT_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"callm"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestTimeoutCLICommands(t *testing.T) {
	for _, command := range []struct {
		name string
		args []string
	}{
		{"chat", []string{"chat", "--no-stream", "prompt"}},
		{"stream", []string{"chat", "--stream", "prompt"}},
		{"anthropic", []string{"chat", "--no-stream", "--ant", "prompt"}},
		{"models", []string{"models"}},
		{"info", []string{"info", "test-model"}},
		{"raw", []string{"raw", "/endpoint", "{}"}},
	} {
		for _, value := range []string{"0.05", "invalid"} {
			t.Run(command.name+"/"+value, func(t *testing.T) {
				var requests atomic.Int32
				release := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					_, _ = io.Copy(io.Discard, r.Body)
					select {
					case <-r.Context().Done():
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				base := server.URL
				if command.name == "anthropic" {
					base += "/api.anthropic.com"
				}
				args := []string{command.args[0], "--api", base, "--api-key", "dummy", "--timeout", value}
				args = append(args, command.args[1:]...)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestTimeoutCLIHelper$", "--"}, args...)...)
				cmd.Env = append(os.Environ(), "CALLM_TIMEOUT_TEST_HELPER=1")
				cmd.Stdin = strings.NewReader("")
				output, err := cmd.CombinedOutput()
				if err == nil || ctx.Err() != nil {
					t.Fatalf("expected CLI error before guard deadline, got err=%v ctx=%v output=%s", err, ctx.Err(), output)
				}
				if value == "invalid" {
					if requests.Load() != 0 || !strings.Contains(string(output), "timeout must be") {
						t.Fatalf("invalid timeout should be rejected without HTTP: requests=%d output=%s", requests.Load(), output)
					}
				} else if requests.Load() != 1 || (!strings.Contains(strings.ToLower(string(output)), "timeout") && !strings.Contains(string(output), "context deadline exceeded")) {
					t.Fatalf("timeout did not reach request: requests=%d output=%s", requests.Load(), output)
				}
			})
		}
	}
}
