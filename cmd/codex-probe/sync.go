package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type SyncResult struct {
	RestoredCount int
	UploadedCount int
	SkippedCount  int
	FailedCount   int
	Errors        []error
}

func validateSyncConfig(cfg ProbeConfig) error {
	if strings.TrimSpace(cfg.SyncURL) == "" {
		return fmt.Errorf("sync config is incomplete: please add sync_url to config.json (see config.example.json)")
	}
	if strings.TrimSpace(cfg.SyncAPIKey) == "" {
		return fmt.Errorf("sync config is incomplete: please add sync_api_key to config.json (see config.example.json)")
	}
	if strings.TrimSpace(cfg.SyncAESGCMKey) == "" {
		return fmt.Errorf("sync config is incomplete: please add sync_aes_gcm_key to config.json (see config.example.json)")
	}
	if strings.TrimSpace(cfg.SyncDir) == "" {
		return fmt.Errorf("sync config is incomplete: please add sync_dir to config.json (see config.example.json)")
	}
	if _, err := decodeSyncKey(cfg.SyncAESGCMKey); err != nil {
		return fmt.Errorf("sync_aes_gcm_key in config.json is invalid: %w", err)
	}
	info, err := os.Stat(cfg.SyncDir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("sync_dir in config.json is invalid: %s is not a directory", cfg.SyncDir)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("sync_dir in config.json is invalid: %w", err)
	}
	return nil
}

func runSync(ctx context.Context, client *http.Client, probeCfg ProbeConfig) (SyncResult, error) {
	if err := validateSyncConfig(probeCfg); err != nil {
		return SyncResult{}, err
	}

	tokenKey, err := decodeSyncKey(probeCfg.SyncAESGCMKey)
	if err != nil {
		return SyncResult{}, fmt.Errorf("decode sync key: %w", err)
	}

	localRecords, localErrs, err := loadSyncLocalRecords(probeCfg.SyncDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			localRecords = map[string]localSyncRecord{}
		} else {
			return SyncResult{}, err
		}
	}

	remoteClient := &supabaseSyncClient{
		baseURL:    probeCfg.SyncURL,
		apiKey:     probeCfg.SyncAPIKey,
		tokenKey:   tokenKey,
		httpClient: client,
	}
	remoteRecords, remoteErrs, err := remoteClient.fetchRemoteTokens(ctx)
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{
		Errors: append(append([]error{}, localErrs...), remoteErrs...),
	}
	result.FailedCount = len(result.Errors)

	for _, decision := range mergeSyncRecords(localRecords, remoteRecords) {
		switch {
		case decision.WriteLocal:
			existingPath := ""
			if localRec, ok := localRecords[decision.AccountID]; ok {
				existingPath = localRec.Path
			}
			if _, err := writeSyncRecord(probeCfg.SyncDir, existingPath, decision.Token); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("restore %s: %w", decision.AccountID, err))
				result.FailedCount++
				continue
			}
			result.RestoredCount++
		case decision.UploadRemote:
			rec := remoteSyncRecord{
				AccountID: decision.AccountID,
				Token:     decision.Token,
			}
			if decision.Token != nil {
				rec.Email = decision.Token.Email
			}
			if err := remoteClient.upsertRemoteToken(ctx, rec); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("upload %s: %w", decision.AccountID, err))
				result.FailedCount++
				continue
			}
			result.UploadedCount++
		default:
			result.SkippedCount++
		}
	}

	return result, nil
}
