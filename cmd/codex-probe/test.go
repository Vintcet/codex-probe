package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

// baseModelList mirrors relay/channel/codex/constants.go
var baseModelList = []string{
	"gpt-5", "gpt-5-codex", "gpt-5-codex-mini",
	"gpt-5.1", "gpt-5.1-codex", "gpt-5.1-codex-max", "gpt-5.1-codex-mini",
	"gpt-5.2", "gpt-5.2-codex", "gpt-5.3-codex", "gpt-5.3-codex-spark",
	"gpt-5.4",
}

// ModelTestResult holds the result of testing one model.
type ModelTestResult struct {
	File       string
	AccountID  string
	Model      string
	HTTPStatus int
	LatencyMs  int64
	Available  bool
	Message    string
	Err        error
}

// APITestTokenSummary is one CSV row per credential: random sample of 3 models → available if any succeeds.
type APITestTokenSummary struct {
	File         string
	AccountID    string
	SampleModels string
	Available    bool
}

const apitestSampleSize = 3

// summarizeAPITestForCSV picks up to 3 random models from results and sets Available if at least one is usable.
func summarizeAPITestForCSV(results []ModelTestResult) APITestTokenSummary {
	s := APITestTokenSummary{}
	if len(results) == 0 {
		return s
	}
	s.File = results[0].File
	s.AccountID = results[0].AccountID

	n := apitestSampleSize
	if len(results) < n {
		n = len(results)
	}
	perm := rand.Perm(len(results))
	var picked []ModelTestResult
	for i := 0; i < n; i++ {
		picked = append(picked, results[perm[i]])
	}
	names := make([]string, 0, len(picked))
	for _, r := range picked {
		names = append(names, r.Model)
		if r.Err == nil && r.Available {
			s.Available = true
		}
	}
	s.SampleModels = strings.Join(names, ";")
	return s
}

// testAllModels tests every model in baseModelList for the given credential.
func testAllModels(ctx context.Context, client *http.Client, entry keyEntry) []ModelTestResult {
	var results []ModelTestResult
	for _, model := range baseModelList {
		r := testModel(ctx, client, entry, model)
		results = append(results, r)
	}
	return results
}

// testModel sends a minimal /backend-api/codex/responses request and reports availability.
func testModel(ctx context.Context, client *http.Client, entry keyEntry, model string) ModelTestResult {
	res := ModelTestResult{
		File:      entry.path,
		AccountID: entry.key.AccountID,
		Model:     model,
	}

	body, err := buildTestRequestBody(model)
	if err != nil {
		res.Err = err
		res.Message = err.Error()
		return res
	}

	reqURL := codexBaseURL + "/backend-api/codex/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		res.Err = err
		res.Message = err.Error()
		return res
	}

	// mirrors adaptor.go SetupRequestHeader
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(entry.key.AccessToken))
	req.Header.Set("chatgpt-account-id", strings.TrimSpace(entry.key.AccountID))
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "codex_cli_rs")
	req.Header.Set("Content-Type", "application/json")
	// Codex backend requires stream=true; use SSE Accept header accordingly.
	req.Header.Set("Accept", "text/event-stream")

	start := time.Now()
	resp, err := client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Err = err
		res.Message = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.HTTPStatus = resp.StatusCode

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Read just enough of the SSE stream to confirm the endpoint responded.
		firstChunk, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		res.Available = true
		if len(firstChunk) > 0 {
			res.Message = "ok (stream)"
		} else {
			res.Message = "ok"
		}
	} else {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		res.Message = extractErrorMessage(errBody, resp.StatusCode)
	}
	return res
}

func buildTestRequestBody(model string) ([]byte, error) {
	// stream=true is required by the Codex backend.
	payload := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "hi"},
		},
		"instructions": "",
		"store":        false,
		"stream":       true,
	}
	return json.Marshal(payload)
}

func extractErrorMessage(body []byte, status int) string {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		// OpenAI-style error envelope
		if errObj, ok := obj["error"].(map[string]any); ok {
			if msg, ok := errObj["message"].(string); ok && msg != "" {
				return msg
			}
			if code, ok := errObj["code"].(string); ok && code != "" {
				return code
			}
		}
		if msg, ok := obj["message"].(string); ok && msg != "" {
			return msg
		}
	}
	if len(body) > 0 && len(body) <= 200 {
		return strings.TrimSpace(string(body))
	}
	return fmt.Sprintf("HTTP %d", status)
}

// printModelTestResults pretty-prints test results for one credential.
func printModelTestResults(results []ModelTestResult) {
	fmt.Printf("  %-30s  %-8s  %-10s  %s\n", "模型", "HTTP", "延迟(ms)", "状态")
	fmt.Printf("  %s\n", strings.Repeat("-", 65))
	for _, r := range results {
		var statusStr string
		if r.Err != nil {
			statusStr = colorRed("ERROR: " + r.Err.Error())
		} else if r.Available {
			statusStr = colorGreen("✓ 可用")
		} else {
			statusStr = colorRed("✗ " + r.Message)
		}
		fmt.Printf("  %-30s  %-8d  %-10d  %s\n",
			r.Model, r.HTTPStatus, r.LatencyMs, statusStr)
	}
}
