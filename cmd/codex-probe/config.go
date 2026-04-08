package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfigFileName = "config.json"
const defaultRenewRetryMax = 3

type ProbeConfig struct {
	RenewBeforeExpiryDays int `json:"renew_before_expiry_days"`
}

func defaultProbeConfig() ProbeConfig {
	return ProbeConfig{
		RenewBeforeExpiryDays: 3,
	}
}

func loadProbeConfig(pathArg, executablePath string) (ProbeConfig, string, error) {
	cfg := defaultProbeConfig()
	if pathArg == "" {
		cfgPath, err := defaultConfigPath(executablePath)
		if err != nil {
			return cfg, "", err
		}
		if _, err := os.Stat(cfgPath); err != nil {
			if os.IsNotExist(err) {
				if errWrite := writeProbeConfig(cfgPath, cfg); errWrite != nil {
					return cfg, "", fmt.Errorf("write default config: %w", errWrite)
				}
				return cfg, cfgPath, nil
			}
			return cfg, "", fmt.Errorf("stat config: %w", err)
		}
		loaded, err := readProbeConfig(cfgPath)
		return loaded, cfgPath, err
	}

	loaded, err := readProbeConfig(pathArg)
	return loaded, pathArg, err
}

func defaultConfigPath(executablePath string) (string, error) {
	if executablePath == "" {
		return "", fmt.Errorf("executable path is empty")
	}
	return filepath.Join(filepath.Dir(executablePath), defaultConfigFileName), nil
}

func readProbeConfig(path string) (ProbeConfig, error) {
	cfg := defaultProbeConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.RenewBeforeExpiryDays < 0 {
		cfg.RenewBeforeExpiryDays = 0
	}
	return cfg, nil
}

func writeProbeConfig(path string, cfg ProbeConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
