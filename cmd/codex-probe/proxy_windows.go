//go:build windows

// proxy_windows.go — proxy detection for Windows
package main

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// detectSystemProxy checks env vars first, then the Windows Internet Settings registry.
func detectSystemProxy() (string, error) {
	// 1. Environment variables
	envCandidates := []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	}
	for _, k := range envCandidates {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v, nil
		}
	}

	// 2. Windows registry: HKCU\...\Internet Settings
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return "", nil
	}
	defer k.Close()

	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return "", nil
	}

	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || strings.TrimSpace(server) == "" {
		return "", nil
	}

	server = strings.TrimSpace(server)
	// ProxyServer may be "host:port" or "http=host:port;https=host:port"
	// Extract https or http entry when it contains protocol prefixes
	if strings.Contains(server, "=") {
		server = extractWindowsProxyEntry(server, "https")
		if server == "" {
			server = extractWindowsProxyEntry(server, "http")
		}
	}
	if server == "" {
		return "", nil
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	return server, nil
}

// extractWindowsProxyEntry parses "http=1.2.3.4:80;https=1.2.3.4:443" style values.
func extractWindowsProxyEntry(s, proto string) string {
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		prefix := proto + "="
		if strings.HasPrefix(strings.ToLower(part), prefix) {
			return strings.TrimSpace(part[len(prefix):])
		}
	}
	return ""
}
