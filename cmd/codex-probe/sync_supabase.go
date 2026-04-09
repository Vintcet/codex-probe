package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type supabaseSyncClient struct {
	baseURL    string
	apiKey     string
	tokenKey   []byte
	httpClient *http.Client
}

type supabaseTokenRow struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	TokenData  string `json:"token_data"`
	UpdateTime string `json:"update_time"`
}

func (c *supabaseSyncClient) client() *http.Client {
	if c != nil && c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *supabaseSyncClient) fetchRemoteTokens(ctx context.Context) (map[string]remoteSyncRecord, []error, error) {
	reqURL := strings.TrimRight(c.baseURL, "/") + "/rest/v1/tokens?select=id,email,token_data,update_time"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build remote tokens request: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch remote tokens: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read remote tokens response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("fetch remote tokens: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []supabaseTokenRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, nil, fmt.Errorf("decode remote tokens: %w", err)
	}

	records := make(map[string]remoteSyncRecord, len(rows))
	var errs []error
	for i, row := range rows {
		rec, rowErr := c.remoteRecordFromRow(row)
		if rowErr != nil {
			errs = append(errs, fmt.Errorf("remote token row %d id=%q: %w", i, row.ID, rowErr))
			continue
		}
		records[rec.AccountID] = rec
	}

	return records, errs, nil
}

func (c *supabaseSyncClient) remoteRecordFromRow(row supabaseTokenRow) (remoteSyncRecord, error) {
	if strings.TrimSpace(row.ID) == "" {
		return remoteSyncRecord{}, fmt.Errorf("missing id")
	}
	if strings.TrimSpace(row.TokenData) == "" {
		return remoteSyncRecord{}, fmt.Errorf("missing token_data")
	}

	updateTime, err := parseSupabaseTimestamp(row.UpdateTime)
	if err != nil {
		return remoteSyncRecord{}, fmt.Errorf("parse update_time: %w", err)
	}

	token, err := decryptOAuthKey(c.tokenKey, row.TokenData)
	if err != nil {
		return remoteSyncRecord{}, err
	}
	if token.AccountID == "" {
		token.AccountID = row.ID
	}
	if token.Email == "" {
		token.Email = row.Email
	}

	return remoteSyncRecord{
		AccountID:  row.ID,
		Email:      row.Email,
		Token:      token,
		UpdateTime: updateTime,
	}, nil
}

func (c *supabaseSyncClient) upsertRemoteToken(ctx context.Context, rec remoteSyncRecord) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("supabase base url is empty")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return fmt.Errorf("supabase api key is empty")
	}
	if rec.Token == nil {
		return fmt.Errorf("upsert remote token: token is nil")
	}

	accountID := strings.TrimSpace(rec.AccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(rec.Token.AccountID)
	}
	if accountID == "" {
		return fmt.Errorf("upsert remote token: account id is empty")
	}
	email := strings.TrimSpace(rec.Email)
	if email == "" {
		email = strings.TrimSpace(rec.Token.Email)
	}

	tokenData, err := encryptOAuthKey(c.tokenKey, rec.Token)
	if err != nil {
		return fmt.Errorf("encrypt remote token: %w", err)
	}

	updateTime := rec.UpdateTime
	if updateTime.IsZero() {
		updateTime, err = parseSupabaseTimestamp(rec.Token.LastRefresh)
		if err != nil {
			return fmt.Errorf("resolve update_time: %w", err)
		}
	}

	payload := map[string]any{
		"id":          accountID,
		"email":       email,
		"token_data":  tokenData,
		"update_time": updateTime.UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal remote token payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/rest/v1/tokens", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build upsert request: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "resolution=merge-duplicates")

	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("upsert remote token: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read upsert response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upsert remote token: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func parseSupabaseTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("timestamp is empty")
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return ts, nil
}
