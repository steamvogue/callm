package client

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// SetProxy sends this client's requests through one proxy: http, https, socks5
// or socks5h. Both SOCKS forms pass the hostname to the proxy, so DNS resolves
// at the exit. "direct" (or "none") ignores the proxy environment variables. An
// empty value keeps the default, HTTPS_PROXY/HTTP_PROXY/NO_PROXY.
//
// Loopback API hosts always connect directly: a remote exit cannot reach this
// machine's local model server. Every other request fails if the proxy is down;
// nothing falls back to a direct connection.
func (c *Client) SetProxy(raw string) error {
	proxy, err := proxyFunc(raw)
	if err != nil || proxy == nil {
		return err
	}
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok {
		return errors.New("proxy requires the default HTTP transport")
	}
	transport.Proxy = proxy
	return nil
}

// proxyFunc returns nil for an empty value. Errors never echo the value, which
// may hold proxy credentials.
func proxyFunc(raw string) (func(*http.Request) (*url.URL, error), error) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		return nil, nil
	case "direct", "none":
		return func(*http.Request) (*url.URL, error) { return nil, nil }, nil
	}
	target, err := url.Parse(raw)
	if err != nil || target.Hostname() == "" {
		return nil, errors.New("proxy must be a URL such as socks5h://127.0.0.1:1080 or http://host:8080, or \"direct\"")
	}
	switch target.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errors.New("proxy scheme must be http, https, socks5 or socks5h")
	}
	return func(req *http.Request) (*url.URL, error) {
		if isLoopbackHost(req.URL.Hostname()) {
			return nil, nil
		}
		return target, nil
	}, nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
