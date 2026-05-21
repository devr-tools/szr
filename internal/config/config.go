package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const appName = "szr"

type Config struct {
	TeeOnFailure    bool `json:"tee_on_failure"`
	MaxPreviewLines int  `json:"max_preview_lines"`
	MaxMatchGroups  int  `json:"max_match_groups"`
}

type Paths struct {
	ConfigDir   string
	ConfigFile  string
	DataDir     string
	HistoryFile string
	TeeDir      string
}

func Default() Config {
	return Config{
		TeeOnFailure:    true,
		MaxPreviewLines: 12,
		MaxMatchGroups:  8,
	}
}

func ResolvePaths() (Paths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return Paths{}, errors.New("failed to resolve user config directory")
		}
		configRoot = filepath.Join(home, ".config")
	}

	dataRoot, err := os.UserCacheDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return Paths{}, errors.New("failed to resolve user cache directory")
		}
		dataRoot = filepath.Join(home, ".local", "share")
	}

	configDir := filepath.Join(configRoot, appName)
	dataDir := filepath.Join(dataRoot, appName)

	return Paths{
		ConfigDir:   configDir,
		ConfigFile:  filepath.Join(configDir, "config.json"),
		DataDir:     dataDir,
		HistoryFile: filepath.Join(dataDir, "history.jsonl"),
		TeeDir:      filepath.Join(dataDir, "tee"),
	}, nil
}

func EnsurePaths(paths Paths) error {
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.TeeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func Load() (Config, Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return Config{}, Paths{}, err
	}
	if err := EnsurePaths(paths); err != nil {
		return Config{}, Paths{}, err
	}

	cfg := Default()
	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, paths, nil
		}
		return Config{}, Paths{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, Paths{}, err
	}
	return cfg, paths, nil
}
