package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RenewResult struct {
	File      string
	AccountID string
	Skipped   bool
	Reason    string
	Err       error
}

func writeRenewSummary(w io.Writer, rows []RenewResult) {
	fmt.Fprintln(w, colorCyan("  [renew summary]"))
	fmt.Fprintf(w, "  %-40s  %s\n", "file", "status")
	fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 55))

	ok, fail := 0, 0
	for _, r := range rows {
		label := r.File
		if r.AccountID != "" {
			label = r.AccountID
		}
		if r.Err != nil {
			fail++
			fmt.Fprintf(w, "  %-40s  %s\n", label, colorRed("ERROR: "+r.Err.Error()))
			continue
		}
		if r.Skipped {
			fmt.Fprintf(w, "  %-40s  %s\n", label, colorCyan("skipped: "+r.Reason))
			continue
		}
		ok++
		fmt.Fprintf(w, "  %-40s  %s\n", label, colorGreen("✓ renewed"))
	}

	fmt.Fprintf(w, "  total: %d  ok: %s  fail: %s\n",
		len(rows), colorGreen(fmt.Sprintf("%d", ok)), colorRed(fmt.Sprintf("%d", fail)))
}

func renewKeyEntry(ctx context.Context, client *http.Client, entry keyEntry, tokenURL string) (*OAuthKey, error) {
	if entry.key == nil {
		return nil, fmt.Errorf("credential is nil")
	}
	if strings.TrimSpace(entry.key.RefreshToken) == "" {
		return nil, fmt.Errorf("refresh_token is empty")
	}

	refreshed, err := refreshTokenWithURL(ctx, client, entry.key.RefreshToken, tokenURL)
	if err != nil {
		return nil, err
	}

	entry.key.AccessToken = refreshed.AccessToken
	entry.key.RefreshToken = refreshed.RefreshToken
	entry.key.IDToken = refreshed.IDToken
	entry.key.LastRefresh = time.Now().Format(time.RFC3339)
	entry.key.Expired = refreshed.ExpiresAt.Format(time.RFC3339)
	if strings.TrimSpace(entry.key.Type) == "" {
		entry.key.Type = "codex"
	}
	if accountID, ok := tokenAccountID(refreshed.IDToken, entry.key.AccessToken); ok {
		entry.key.AccountID = accountID
	}
	if refreshed.Email != "" {
		entry.key.Email = refreshed.Email
	}

	if err := saveKeyToFile(entry.path, entry.key); err != nil {
		return nil, err
	}
	return entry.key, nil
}

func renewKeyEntryWithRetry(ctx context.Context, client *http.Client, entry keyEntry, tokenURL string, maxRetries int) (*OAuthKey, error) {
	if maxRetries <= 0 {
		maxRetries = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		updated, err := renewKeyEntry(ctx, client, entry, tokenURL)
		if err == nil {
			return updated, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func shouldRenewKey(key *OAuthKey, cfg ProbeConfig, force bool, now time.Time) (bool, string) {
	if force {
		return true, "force"
	}
	if key == nil {
		return true, "missing credential"
	}
	if strings.TrimSpace(key.IDToken) == "" {
		return true, "missing id_token"
	}
	expiredRaw := strings.TrimSpace(key.Expired)
	if expiredRaw == "" {
		return true, "missing expired"
	}
	expiry, err := time.Parse(time.RFC3339, expiredRaw)
	if err != nil {
		return true, "invalid expired"
	}
	threshold := now.Add(time.Duration(cfg.RenewBeforeExpiryDays) * 24 * time.Hour)
	if !expiry.After(threshold) {
		return true, "expiring soon"
	}
	return false, ""
}
