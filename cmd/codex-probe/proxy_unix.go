//go:build !windows

// proxy_unix.go — proxy detection for Linux and macOS
package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// detectSystemProxy returns the system-configured proxy URL, or "" if none.
// Priority:
//  1. Standard env vars (HTTPS_PROXY / HTTP_PROXY / ALL_PROXY)
//  2. macOS: scutil --proxy (system network preferences)
func detectSystemProxy() (string, error) {
	// 1. Environment variables (common to Linux, macOS, CI, Docker …)
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

	// 2. macOS system proxy via scutil
	if runtime.GOOS == "darwin" {
		if p, err := detectMacOSProxy(); err == nil && p != "" {
			return p, nil
		}
	}

	return "", nil
}

// detectMacOSProxy runs `scutil --proxy` and parses the output.
// Returns "" when no proxy is configured.
func detectMacOSProxy() (string, error) {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return "", fmt.Errorf("scutil: %w", err)
	}
	return parseSCUtilProxy(string(out)), nil
}

// parseSCUtilProxy parses output like:
//
//	<dictionary> {
//	  HTTPSEnable : 1
//	  HTTPSProxy : 127.0.0.1
//	  HTTPSPort : 7890
//	  SOCKSEnable : 1
//	  SOCKSProxy : 127.0.0.1
//	  SOCKSPort : 7890
//	}
func parseSCUtilProxy(output string) string {
	kv := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		kv[k] = v
	}

	// Prefer HTTPS proxy
	if kv["HTTPSEnable"] == "1" {
		host := kv["HTTPSProxy"]
		port := kv["HTTPSPort"]
		if host != "" {
			if port != "" {
				return "http://" + host + ":" + port
			}
			return "http://" + host
		}
	}
	// Then HTTP proxy
	if kv["HTTPEnable"] == "1" {
		host := kv["HTTPProxy"]
		port := kv["HTTPPort"]
		if host != "" {
			if port != "" {
				return "http://" + host + ":" + port
			}
			return "http://" + host
		}
	}
	// Then SOCKS proxy
	if kv["SOCKSEnable"] == "1" {
		host := kv["SOCKSProxy"]
		port := kv["SOCKSPort"]
		if host != "" {
			if _, err := strconv.Atoi(port); err == nil && port != "" {
				return "socks5://" + host + ":" + port
			}
			return "socks5://" + host
		}
	}
	return ""
}
