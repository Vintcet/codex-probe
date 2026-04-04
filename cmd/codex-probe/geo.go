package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// geoAPIs are tried in order; first successful response wins.
var geoAPIs = []struct {
	url     string
	extract func(body string) string
}{
	{
		// returns plain text: "CN\n"
		url:     "https://ipinfo.io/country",
		extract: func(body string) string { return strings.TrimSpace(body) },
	},
	{
		// returns JSON: {"ip":"1.2.3.4","country":"CN"}
		url: "https://api.country.is/",
		extract: func(body string) string {
			for _, part := range strings.Split(body, `"`) {
				part = strings.TrimSpace(part)
				if len(part) == 2 && part == strings.ToUpper(part) && part != "ip" {
					return part
				}
			}
			return ""
		},
	},
	{
		// returns JSON: {"countryCode":"CN", ...}
		url: "http://ip-api.com/json/?fields=countryCode",
		extract: func(body string) string {
			const key = `"countryCode":"`
			idx := strings.Index(body, key)
			if idx < 0 {
				return ""
			}
			rest := body[idx+len(key):]
			end := strings.Index(rest, `"`)
			if end < 0 {
				return ""
			}
			return strings.TrimSpace(rest[:end])
		},
	},
}

// checkNotChina fetches the caller's country via public geo APIs.
// If CN is detected, prints a warning box and exits.
// If all APIs fail, prints a warning but continues.
func checkNotChina(client *http.Client) {
	infof("detecting network region...")

	countryCode, apiURL := fetchCountryCode(client)
	if countryCode == "" {
		warnf("could not determine network region, skipping check (continuing)")
		return
	}

	infof("network region: %s  (via %s)", countryCode, apiURL)

	if strings.EqualFold(countryCode, "CN") {
		fmt.Println()
		printCNWarningBox()
		fmt.Println()
		fatalf("exiting — please configure a proxy and retry.")
	}
}

// printCNWarningBox renders a properly aligned warning box.
// It uses runeDisplayWidth to account for double-width CJK characters.
func printCNWarningBox() {
	const innerW = 52 // display columns between the two │

	top := "┌" + strings.Repeat("─", innerW) + "┐"
	bot := "└" + strings.Repeat("─", innerW) + "┘"

	lines := []string{
		"",
		"[!]  检测到当前 IP 位于中国大陆 (CN)",
		"",
		"     Codex API 在中国大陆无法访问。",
		"     请配置代理后再使用本工具:",
		"     示例: --proxy http://127.0.0.1:7890",
		"",
	}

	fmt.Println(colorRed(top))
	for _, l := range lines {
		fmt.Println(colorRed(boxLine(l, innerW)))
	}
	fmt.Println(colorRed(bot))
}

// boxLine pads content to exactly innerW display columns, wrapped in │...│.
func boxLine(content string, innerW int) string {
	dw := runeDisplayWidth(content)
	pad := innerW - dw
	if pad < 0 {
		pad = 0
	}
	return "│" + content + strings.Repeat(" ", pad) + "│"
}

// runeDisplayWidth returns the number of terminal columns a string occupies.
// ASCII and Latin characters = 1 column; CJK, full-width = 2 columns.
func runeDisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWideRune(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// isWideRune reports whether r occupies two terminal columns.
func isWideRune(r rune) bool {
	// CJK Unified Ideographs and common full-width blocks
	if r >= 0x1100 && r <= 0x115F { return true }  // Hangul Jamo
	if r >= 0x2E80 && r <= 0x303E { return true }  // CJK Radicals / Kangxi
	if r >= 0x3040 && r <= 0x33FF { return true }  // Hiragana, Katakana, CJK symbols
	if r >= 0x3400 && r <= 0x4DBF { return true }  // CJK Extension A
	if r >= 0x4E00 && r <= 0x9FFF { return true }  // CJK Unified Ideographs
	if r >= 0xA000 && r <= 0xA4CF { return true }  // Yi
	if r >= 0xAC00 && r <= 0xD7AF { return true }  // Hangul Syllables
	if r >= 0xF900 && r <= 0xFAFF { return true }  // CJK Compatibility Ideographs
	if r >= 0xFE10 && r <= 0xFE1F { return true }  // Vertical forms
	if r >= 0xFE30 && r <= 0xFE6F { return true }  // CJK Compatibility Forms / Small Forms
	if r >= 0xFF00 && r <= 0xFF60 { return true }  // Fullwidth Latin / Katakana half
	if r >= 0xFFE0 && r <= 0xFFE6 { return true }  // Fullwidth Signs
	if r >= 0x20000 && r <= 0x2FA1F { return true } // CJK Extension B-F + Compatibility Supplement
	return unicode.Is(unicode.Han, r)
}

func fetchCountryCode(client *http.Client) (code string, apiURL string) {
	for _, api := range geoAPIs {
		c, err := fetchFromGeoAPI(client, api.url, api.extract)
		if err == nil && len(c) == 2 {
			return c, api.url
		}
	}
	return "", ""
}

func fetchFromGeoAPI(client *http.Client, apiURL string, extract func(string) string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "codex-probe/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", err
	}

	code := extract(string(body))
	if len(code) != 2 {
		return "", fmt.Errorf("unexpected country code %q", code)
	}
	return strings.ToUpper(code), nil
}
