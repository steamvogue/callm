package client

import (
	"net/http"
	"strings"
	"testing"
)

func TestProxySelection(t *testing.T) {
	for _, tc := range []struct{ proxy, target, want string }{
		{"socks5h://127.0.0.1:1080", "https://eu.openrouter.ai/api/v1/models", "socks5h://127.0.0.1:1080"},
		{"socks5://user:pw@proxy.example:1080", "https://openrouter.ai/x", "socks5://user:pw@proxy.example:1080"},
		{"http://proxy.example:8080", "http://api.example/x", "http://proxy.example:8080"},
		{"https://proxy.example", "https://api.example/x", "https://proxy.example"},
		{"socks5h://127.0.0.1:1080", "http://localhost:11434/v1/models", ""},
		{"socks5h://127.0.0.1:1080", "http://127.0.0.1:8000/v1", ""},
		{"socks5h://127.0.0.1:1080", "http://[::1]:8000/v1", ""},
		{"socks5h://127.0.0.1:1080", "http://ollama.localhost/v1", ""},
		{"direct", "https://openrouter.ai/x", ""},
		{" NONE ", "https://openrouter.ai/x", ""},
	} {
		c := NewClient("https://api.example/v1", "key")
		if err := c.SetProxy(tc.proxy); err != nil {
			t.Fatalf("%s: %v", tc.proxy, err)
		}
		req, _ := http.NewRequest("GET", tc.target, nil)
		got, err := c.HTTPClient.Transport.(*http.Transport).Proxy(req)
		if err != nil || (got == nil) != (tc.want == "") || (got != nil && got.String() != tc.want) {
			t.Fatalf("%s -> %s: got %v, %v", tc.proxy, tc.target, got, err)
		}
	}
}

func TestProxyRejectsInvalidWithoutEchoingCredentials(t *testing.T) {
	for _, value := range []string{"user:secret@127.0.0.1:1080", "ftp://user:secret@host:21", "socks4://user:secret@host", "socks5h://", "http://user:secret@%zz"} {
		err := NewClient("https://api.example/v1", "key").SetProxy(value)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("%q: %v", value, err)
		}
	}
}

// An empty value leaves net/http's environment-based proxy selection in place.
func TestProxyEmptyKeepsDefault(t *testing.T) {
	c := NewClient("https://api.example/v1", "key")
	if err := c.SetProxy(""); err != nil || c.HTTPClient.Transport.(*http.Transport).Proxy == nil {
		t.Fatalf("default proxy selection lost: %v", err)
	}
}

func TestOpenRouterRegionalHostsUseOpenRouterProtocol(t *testing.T) {
	for _, base := range []string{"https://eu.openrouter.ai/api/v1", "https://us.openrouter.ai/api/v1"} {
		if c := NewClient(base, "key"); c.Provider != "or" {
			t.Fatalf("%s: provider %q", base, c.Provider)
		}
	}
}
