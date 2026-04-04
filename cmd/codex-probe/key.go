package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OAuthKey mirrors relay/channel/codex/oauth_key.go
type OAuthKey struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Email        string `json:"email,omitempty"`
	Type         string `json:"type,omitempty"`
	Expired      string `json:"expired,omitempty"`
}

func parseOAuthKey(raw string) (*OAuthKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty oauth key")
	}
	if !strings.HasPrefix(raw, "{") {
		return nil, fmt.Errorf("not a valid JSON object (content does not start with '{')")
	}
	var key OAuthKey
	if err := json.Unmarshal([]byte(raw), &key); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return &key, nil
}

// validateOAuthKey checks that a key has the required fields and a valid JWT format.
func validateOAuthKey(key *OAuthKey) error {
	if strings.TrimSpace(key.AccessToken) == "" {
		return fmt.Errorf("access_token is empty")
	}
	if strings.TrimSpace(key.AccountID) == "" {
		return fmt.Errorf("account_id is empty")
	}
	parts := strings.Split(key.AccessToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("access_token is not a valid JWT (expected 3 parts, got %d)", len(parts))
	}
	for i, p := range parts {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("access_token JWT part %d is empty", i+1)
		}
	}
	return nil
}

func loadKeyFromFile(path string) (*OAuthKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseOAuthKey(string(data))
}

func saveKeyToFile(path string, key *OAuthKey) error {
	data, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// buildKeyFileName generates a filename like codex_{email}_{date}.json
func buildKeyFileName(key *OAuthKey) string {
	email := key.Email
	if email == "" {
		email = key.AccountID
	}
	if email == "" {
		email = "unknown"
	}
	// sanitize
	safe := strings.NewReplacer("@", "_at_", ".", "_", "/", "_", "\\", "_", ":", "_").Replace(email)
	date := time.Now().Format("20060102")
	return fmt.Sprintf("codex_%s_%s.json", safe, date)
}

// prepareLoginOutputPath validates and prepares the output destination before OAuth starts.
// It returns:
//   - outDir: the directory that will hold the file (always pre-created)
//   - outFileHint: the explicit file path (only meaningful when isDirMode=false)
//   - isDirMode: true when the user provided a directory; filename will be auto-generated later
func prepareLoginOutputPath(pathArg string) (outDir, outFileHint string, isDirMode bool, err error) {
	if strings.TrimSpace(pathArg) == "" {
		return "", "", false, fmt.Errorf("path argument is empty")
	}

	cleaned := filepath.Clean(pathArg)

	// Determine intent: directory vs explicit file
	info, statErr := os.Stat(pathArg)
	existsAsDir := statErr == nil && info.IsDir()
	trailingSep := strings.HasSuffix(pathArg, "/") || strings.HasSuffix(pathArg, string(filepath.Separator))
	hasExt := filepath.Ext(cleaned) != ""

	if existsAsDir || trailingSep || !hasExt {
		// Directory mode — filename auto-generated after OAuth
		dir := cleaned
		if err2 := os.MkdirAll(dir, 0755); err2 != nil {
			return "", "", true, fmt.Errorf("failed to create directory %s: %w", dir, err2)
		}
		return dir, "", true, nil
	}

	// Explicit file mode — ensure parent directory exists
	parent := filepath.Dir(cleaned)
	if parent != "" && parent != "." {
		if err2 := os.MkdirAll(parent, 0755); err2 != nil {
			return "", "", false, fmt.Errorf("failed to create parent directory %s: %w", parent, err2)
		}
	}
	return filepath.Dir(cleaned), cleaned, false, nil
}

// resolveOutputPath determines the final file path for a key given the user-provided path argument.
// Rules:
//   - Already-existing directory → generate filename inside it
//   - Path ends with / or \ (intended as dir, may not exist) → mkdir -p then generate filename
//   - No file extension (e.g. "tokens/myaccount") → treated as dir intent → mkdir -p + generate filename
//   - Otherwise treated as explicit file path → ensure parent dirs exist
func resolveOutputPath(pathArg string, key *OAuthKey) (string, error) {
	// 1. Path already exists as a directory
	if info, err := os.Stat(pathArg); err == nil && info.IsDir() {
		return filepath.Join(pathArg, buildKeyFileName(key)), nil
	}

	cleaned := filepath.Clean(pathArg)

	// 2. Trailing separator → user intends a directory
	trailingSep := strings.HasSuffix(pathArg, "/") || strings.HasSuffix(pathArg, string(filepath.Separator))
	// 3. No extension → also treat as directory
	noExt := filepath.Ext(cleaned) == ""

	if trailingSep || noExt {
		if err := os.MkdirAll(cleaned, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", cleaned, err)
		}
		return filepath.Join(cleaned, buildKeyFileName(key)), nil
	}

	// 4. Explicit file path — ensure parent directory exists
	parent := filepath.Dir(cleaned)
	if parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory %s: %w", parent, err)
		}
	}
	return cleaned, nil
}

// loadKeysFromPath loads one or more tokens depending on whether pathArg is file/dir.
// Returns a slice of (filePath, key) pairs. All returned entries have been validated.
func loadKeysFromPath(pathArg string) ([]keyEntry, error) {
	info, err := os.Stat(pathArg)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", pathArg)
	}
	if !info.IsDir() {
		key, err := loadKeyFromFile(pathArg)
		if err != nil {
			return nil, fmt.Errorf("failed to read credential file (%s): %w", pathArg, err)
		}
		if err := validateOAuthKey(key); err != nil {
			return nil, fmt.Errorf("invalid credential (%s): %w", pathArg, err)
		}
		return []keyEntry{{path: pathArg, key: key}}, nil
	}

	entries, err := os.ReadDir(pathArg)
	if err != nil {
		return nil, err
	}
	var result []keyEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		fp := filepath.Join(pathArg, e.Name())
		key, err := loadKeyFromFile(fp)
		if err != nil {
			warnf("skipping %s: JSON parse error: %v", fp, err)
			continue
		}
		if err := validateOAuthKey(key); err != nil {
			warnf("skipping %s: invalid credential: %v", fp, err)
			continue
		}
		result = append(result, keyEntry{path: fp, key: key})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid Codex credential JSON files found in %s", pathArg)
	}
	return result, nil
}

type keyEntry struct {
	path string
	key  *OAuthKey
}
