package main

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const syncPromptText = "Detected renewed tokens in sync_dir. Sync now? [y/N]: "

func shouldPromptForSyncAfterRenew(probeCfg ProbeConfig, renewRows []RenewResult) bool {
	if err := validateSyncConfig(probeCfg); err != nil {
		return false
	}

	for _, row := range renewRows {
		if row.Err != nil || row.Skipped {
			continue
		}
		if renewedFileBelongsToSyncDir(row.File, probeCfg.SyncDir) {
			return true
		}
	}
	return false
}

func promptForSync(in io.Reader, out io.Writer) bool {
	fmt.Fprint(out, syncPromptText)

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func renewedFileBelongsToSyncDir(filePath, syncDir string) bool {
	_, realDir, err := resolveRealSyncDir(syncDir)
	if err != nil {
		return false
	}

	realFile, err := filepath.EvalSymlinks(filePath)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(realFile) {
		realFile, err = filepath.Abs(realFile)
		if err != nil {
			return false
		}
	}
	return pathWithinDir(realDir, realFile)
}
