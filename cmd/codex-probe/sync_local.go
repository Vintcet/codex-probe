package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func loadSyncLocalRecords(dir string) (map[string]localSyncRecord, []error, error) {
	absDir, realDir, err := resolveRealSyncDir(dir)
	if err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read sync dir %s: %w", dir, err)
	}

	records := make(map[string]localSyncRecord, len(entries))
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		path := filepath.Join(absDir, entry.Name())
		realPath, err := resolveExistingPathWithinDir(realDir, path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}

		token, err := loadKeyFromFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if err := validateOAuthKey(token); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}

		lastRefresh, err := parseSyncLastRefresh(token.LastRefresh)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if !pathWithinDir(realDir, realPath) {
			errs = append(errs, fmt.Errorf("%s: resolved path %s escapes sync dir %s", path, realPath, realDir))
			continue
		}

		if existing, ok := records[token.AccountID]; ok {
			errs = append(errs, fmt.Errorf("%s: duplicate account_id %q already loaded from %s", path, token.AccountID, existing.Path))
			continue
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: resolve absolute path: %w", path, err))
			continue
		}
		records[token.AccountID] = localSyncRecord{
			AccountID:   token.AccountID,
			Path:        absPath,
			Token:       token,
			LastRefresh: lastRefresh,
		}
	}

	return records, errs, nil
}

func writeSyncRecord(dir string, existingPath string, token *OAuthKey) (string, error) {
	if token == nil {
		return "", fmt.Errorf("sync token is nil")
	}
	if err := validateOAuthKey(token); err != nil {
		return "", fmt.Errorf("validate sync token: %w", err)
	}
	if _, err := parseSyncLastRefresh(token.LastRefresh); err != nil {
		return "", fmt.Errorf("validate sync token: %w", err)
	}
	if strings.TrimSpace(existingPath) == "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create sync dir %s: %w", dir, err)
		}
	}

	path, err := resolveSyncRecordPath(dir, existingPath)
	if err != nil {
		return "", err
	}
	if path == "" {
		absDir, realDir, err := resolveRealSyncDir(dir)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(absDir, 0755); err != nil {
			return "", fmt.Errorf("create sync dir %s: %w", dir, err)
		}
		path = filepath.Join(absDir, buildSyncRecordFileName(token))
		if err := ensurePathWithinDir(realDir, path); err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", fmt.Errorf("create sync parent dir %s: %w", filepath.Dir(path), err)
		}
	}

	if err := saveKeyToFile(path, token); err != nil {
		return "", err
	}
	return path, nil
}

func buildSyncRecordFileName(token *OAuthKey) string {
	name := strings.TrimSpace(token.Email)
	if name == "" {
		name = strings.TrimSpace(token.AccountID)
	}
	if name == "" {
		name = "unknown"
	}
	name = strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(name)
	return name + ".json"
}

func parseSyncLastRefresh(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("last_refresh is empty")
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse last_refresh: %w", err)
	}
	return ts, nil
}

func resolveSyncRecordPath(dir string, existingPath string) (string, error) {
	existingPath = strings.TrimSpace(existingPath)
	if existingPath == "" {
		return "", nil
	}

	baseDir, realDir, err := resolveRealSyncDir(dir)
	if err != nil {
		return "", err
	}

	candidate := existingPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseDir, candidate)
	}
	candidate = filepath.Clean(candidate)

	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve sync record path %s: %w", existingPath, err)
	}
	info, err := os.Stat(absCandidate)
	if err == nil && info.IsDir() {
		return "", fmt.Errorf("existingPath %s points to a directory", existingPath)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat existingPath %s: %w", existingPath, err)
	}
	if err == nil {
		realCandidate, err := filepath.EvalSymlinks(absCandidate)
		if err != nil {
			return "", fmt.Errorf("resolve existingPath %s: %w", existingPath, err)
		}
		if !pathWithinDir(realDir, realCandidate) {
			return "", fmt.Errorf("existingPath %s resolves outside sync dir %s via symlink traversal to %s", existingPath, realDir, realCandidate)
		}
		return absCandidate, nil
	}

	if err := ensurePathWithinDir(realDir, absCandidate); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absCandidate), 0755); err != nil {
		return "", fmt.Errorf("create sync parent dir %s: %w", filepath.Dir(absCandidate), err)
	}

	return absCandidate, nil
}

func resolveRealSyncDir(dir string) (absDir string, realDir string, err error) {
	absDir, err = filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve sync dir %s: %w", dir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", "", fmt.Errorf("stat sync dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("sync dir %s is not a directory", dir)
	}
	realDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve real sync dir %s: %w", dir, err)
	}
	return absDir, realDir, nil
}

func resolveExistingPathWithinDir(realDir string, path string) (string, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve real path: %w", err)
	}
	if !pathWithinDir(realDir, realPath) {
		return "", fmt.Errorf("resolved path %s escapes sync dir %s via symlink traversal", realPath, realDir)
	}
	return realPath, nil
}

func ensurePathWithinDir(realDir string, path string) error {
	realParent, err := resolveRealParent(path)
	if err != nil {
		return err
	}
	if !pathWithinDir(realDir, realParent) {
		return fmt.Errorf("path %s escapes sync dir %s via parent %s", path, realDir, realParent)
	}
	return nil
}

func resolveRealParent(path string) (string, error) {
	parent := filepath.Dir(path)
	if parent == "." || parent == string(filepath.Separator) {
		return filepath.Clean(parent), nil
	}

	for {
		info, err := os.Lstat(parent)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return filepath.EvalSymlinks(parent)
			}
			return filepath.EvalSymlinks(parent)
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat parent %s: %w", parent, err)
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("no existing ancestor for %s", path)
		}
		parent = next
	}
}

func pathWithinDir(root string, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
