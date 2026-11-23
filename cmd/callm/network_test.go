package main

import (
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// socksMock is a minimal SOCKS5 server. It records each requested destination
// and connects only the hosts in routes, so tests never leave the machine.
type socksMock struct {
	listener   net.Listener
	routes     map[string]string // requested host:port -> local address
	user, pass string            // nonempty: require username/password auth
	mu         sync.Mutex
	seen       []string
}

func newSocksMock(t *testing.T, routes map[string]string, credentials ...string) *socksMock {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mock := &socksMock{listener: listener, routes: routes}
	if len(credentials) == 2 {
		mock.user, mock.pass = credentials[0], credentials[1]
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go mock.serve(conn)
		}
	}()
	return mock
}

func (m *socksMock) requests() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.seen...)
}

func (m *socksMock) serve(conn net.Conn) {
	defer conn.Close()
	read := func(n int) []byte {
		buf := make([]byte, n)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return nil
		}
		return buf
	}
	head := read(2)
	if head == nil || head[0] != 5 || read(int(head[1])) == nil {
		return
	}
	if m.user == "" {
		conn.Write([]byte{5, 0})
	} else {
		conn.Write([]byte{5, 2})
		auth := read(2)
		if auth == nil {
			return
		}
		user := read(int(auth[1]))
		size := read(1)
		if user == nil || size == nil {
			return
		}
		pass := read(int(size[0]))
		if string(user) != m.user || string(pass) != m.pass {
			conn.Write([]byte{1, 1})
			return
		}
		conn.Write([]byte{1, 0})
	}
	request := read(4)
	if request == nil || request[1] != 1 {
		return
	}
	var host string
	switch request[3] {
	case 3:
		size := read(1)
		if size == nil {
			return
		}
		host = string(read(int(size[0])))
	case 1:
		host = "ip:" + net.IP(read(4)).String()
	default:
		return
	}
	port := read(2)
	if port == nil {
		return
	}
	target := net.JoinHostPort(host, itoa(binary.BigEndian.Uint16(port)))
	m.mu.Lock()
	m.seen = append(m.seen, target)
	m.mu.Unlock()
	upstream, err := net.Dial("tcp", m.routes[target])
	if err != nil {
		conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	go io.Copy(upstream, conn)
	io.Copy(conn, upstream)
}

func itoa(n uint16) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

func chatMock(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, `{"data":[{"id":"model-x"}]}`)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"proxied answer"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(server.Close)
	return server
}

// The hostname exists only inside the mock proxy, so a successful answer proves
// the request went through it and that the proxy, not this host, resolved DNS.
func TestProxyCLI(t *testing.T) {
	server := chatMock(t)
	routes := map[string]string{"upstream.invalid:80": strings.TrimPrefix(server.URL, "http://")}
	base := []string{"--api", "http://upstream.invalid/v1", "--api-key", "dummy", "--no-stdin", "--no-stream"}
	for _, tc := range []struct {
		name, scheme, user, pass string
		env                      bool
		command                  []string
		want                     string
	}{
		{"socks5h flag", "socks5h", "", "", false, []string{"prompt"}, "proxied answer"},
		{"socks5 flag", "socks5", "", "", false, []string{"prompt"}, "proxied answer"},
		{"credentials", "socks5h", "alice", "s3cret", false, []string{"prompt"}, "proxied answer"},
		{"environment", "socks5h", "", "", true, []string{"prompt"}, "proxied answer"},
		{"models", "socks5h", "", "", false, []string{"models"}, "model-x"},
		{"info", "socks5h", "", "", false, []string{"info", "model-x"}, "model-x"},
		{"raw", "socks5h", "", "", false, []string{"raw", "/chat/completions", `{"messages":[]}`}, "proxied answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := newSocksMock(t, routes, tc.user, tc.pass)
			address := tc.scheme + "://"
			if tc.user != "" {
				address += tc.user + ":" + tc.pass + "@"
			}
			address += proxy.listener.Addr().String()
			args := append([]string{}, base...)
			if !tc.env {
				args = append(args, "--proxy", address)
			}
			if tc.command[0] != "prompt" {
				// Subcommands take only the shared flags.
				args = append([]string{tc.command[0], "--api", "http://upstream.invalid/v1", "--api-key", "dummy"}, args[len(base):]...)
				args = append(args, tc.command[1:]...)
			} else {
				args = append(args, tc.command...)
			}
			cmd := testCLI(t, args...)
			cmd.Env = append(cmd.Env, "CALLM_BASE_URL=", "OPENROUTER_REGION=")
			if tc.env {
				cmd.Env = append(cmd.Env, "CALLM_PROXY="+address)
			} else {
				cmd.Env = append(cmd.Env, "CALLM_PROXY=socks5h://127.0.0.1:1")
			}
			out, err := cmd.CombinedOutput()
			seen := proxy.requests()
			if err != nil || !strings.Contains(string(out), tc.want) || len(seen) != 1 || seen[0] != "upstream.invalid:80" {
				t.Fatalf("err=%v seen=%v output=%s", err, seen, out)
			}
		})
	}
}

func TestProxyFailsClosedAndHidesCredentials(t *testing.T) {
	server := chatMock(t)
	for _, tc := range []struct {
		name, proxy, api, want string
		ok                     bool
	}{
		// Port 1 is closed: the request must fail instead of connecting directly.
		{"dead proxy", "socks5h://alice:s3cret@127.0.0.1:1", "http://upstream.invalid/v1", "", false},
		{"bad scheme", "ftp://alice:s3cret@127.0.0.1:1", server.URL + "/v1", "proxy scheme must be", false},
		{"no scheme", "alice:s3cret@127.0.0.1:1", server.URL + "/v1", "proxy", false},
		// A loopback API host is reached directly, whatever the proxy state.
		{"loopback bypass", "socks5h://127.0.0.1:1", server.URL + "/v1", "proxied answer", true},
		{"direct", "direct", server.URL + "/v1", "proxied answer", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := testCLI(t, "--api", tc.api, "--api-key", "dummy", "--no-stdin", "--no-stream", "--proxy", tc.proxy, "prompt")
			cmd.Env = append(cmd.Env, "CALLM_BASE_URL=", "OPENROUTER_REGION=", "CALLM_PROXY=", "HTTP_PROXY=http://127.0.0.1:1", "NO_PROXY=")
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok || !strings.Contains(string(out), tc.want) || strings.Contains(string(out), "s3cret") {
				t.Fatalf("err=%v output=%s", err, out)
			}
		})
	}
}

// The mock proxy refuses the connection after recording its destination, so
// these cases show which OpenRouter host a region selects without contacting it.
func TestOpenRouterRegionCLI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		env    []string
		target string
		want   string
	}{
		{"eu flag", []string{"--or", "--region", "eu"}, nil, "eu.openrouter.ai:443", ""},
		{"us flag", []string{"--or", "--region", "US"}, nil, "us.openrouter.ai:443", ""},
		{"environment", []string{"--or"}, []string{"OPENROUTER_REGION=eu"}, "eu.openrouter.ai:443", ""},
		{"flag over environment", []string{"--or", "--region", "global"}, []string{"OPENROUTER_REGION=eu"}, "openrouter.ai:443", ""},
		{"models", []string{"models", "--or", "--region", "eu"}, nil, "eu.openrouter.ai:443", ""},
		{"info", []string{"info", "--or", "--region", "eu"}, nil, "eu.openrouter.ai:443", ""},
		{"raw", []string{"raw", "--or", "--region", "eu"}, nil, "eu.openrouter.ai:443", ""},
		{"other provider ignores environment", []string{"--orca"}, []string{"OPENROUTER_REGION=eu"}, "api.orcarouter.ai:443", ""},
		{"other provider", []string{"--orca", "--region", "eu"}, nil, "", "--region applies to OpenRouter only"},
		{"unknown", []string{"--or", "--region", "asia"}, nil, "", "choose eu, us or global"},
		{"flag with base URL", []string{"--or", "--region", "eu", "--api", "http://127.0.0.1:1/v1"}, nil, "", "conflicts with a custom base URL"},
		{"environment with base URL", []string{"--or"}, []string{"OPENROUTER_REGION=eu", "CALLM_BASE_URL=http://127.0.0.1:1/v1"}, "", "conflicts with a custom base URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := newSocksMock(t, nil)
			args := append([]string{}, tc.args...)
			args = append(args, "--proxy", "socks5h://"+proxy.listener.Addr().String())
			switch {
			case len(tc.args) > 0 && tc.args[0] == "info":
				args = append(args, "model-x")
			case len(tc.args) > 0 && tc.args[0] == "raw":
				args = append(args, "/chat/completions", `{"messages":[]}`)
			case len(tc.args) > 0 && tc.args[0] == "models":
			default:
				args = append(args, "--no-stdin", "--no-stream", "prompt")
			}
			cmd := testCLI(t, args...)
			// Empty values stop configuration files from supplying other providers' keys.
			cmd.Env = append(cmd.Env, "CALLM_API_KEY=", "CALLM_BASE_URL=", "CALLM_MODEL=", "CALLM_PROXY=", "OPENROUTER_REGION=",
				"POOLSIDE_API_KEY=", "ORCA_API_KEY=", "STRAITLY_API_KEY=", "DEEPSEEK_API_KEY=", "KIMI_API_KEY=", "OPENROUTER_API_KEY=openrouter-dummy")
			if tc.name == "other provider ignores environment" || tc.name == "other provider" {
				cmd.Env = append(cmd.Env, "ORCA_API_KEY=orca-dummy")
			}
			cmd.Env = append(cmd.Env, tc.env...)
			out, err := cmd.CombinedOutput()
			seen := proxy.requests()
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("err=%v output=%s", err, out)
			}
			if tc.target == "" && len(seen) != 0 {
				t.Fatalf("rejected options still connected: %v", seen)
			}
			if tc.target != "" && (len(seen) == 0 || seen[0] != tc.target) {
				t.Fatalf("destination %v, want %s; output=%s", seen, tc.target, out)
			}
		})
	}
}
