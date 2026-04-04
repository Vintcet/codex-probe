// proxy.go — shared HTTP client builder, no build tag (all platforms)
package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// buildHTTPClient constructs an *http.Client.
// proxyURL = ""  → direct connection
// proxyURL = "http://..." | "socks5://..." → route through proxy
func buildHTTPClient(proxyURL string) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return &http.Client{}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
	}

	var transport *http.Transport
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if u.User != nil {
			pw, _ := u.User.Password()
			auth = &proxy.Auth{User: u.User.Username(), Password: pw}
		}
		dialer, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 dialer: %w", err)
		}
		transport = &http.Transport{Dial: dialer.Dial}
	default:
		transport = &http.Transport{Proxy: http.ProxyURL(u)}
	}
	return &http.Client{Transport: transport}, nil
}
