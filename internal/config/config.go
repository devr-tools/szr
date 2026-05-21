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
	return ResolvePathsWith(os.UserConfigDir, os.UserCacheDir, os.UserHomeDir)
}

func EnsurePaths(paths Paths) error {
	return EnsurePathsWith(paths, os.MkdirAll)
}

func Load() (Config, Paths, error) {
	return LoadWith(ResolvePaths, EnsurePaths, os.ReadFile)
}

func ResolvePathsWith(
	userConfigDir func() (string, error),
	userCacheDir func() (string, error),
	userHomeDir func() (string, error),
) (Paths, error) {
	configRoot, err := userConfigDir()
	if err != nil {
		home, homeErr := userHomeDir()
		if homeErr != nil {
			return Paths{}, errors.New("failed to resolve user config directory")
		}
		configRoot = filepath.Join(home, ".config")
	}

	dataRoot, err := userCacheDir()
	if err != nil {
		home, homeErr := userHomeDir()
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

func EnsurePathsWith(paths Paths, mkdirAll func(string, os.FileMode) error) error {
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.TeeDir} {
		if err := mkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func LoadWith(
	resolve func() (Paths, error),
	ensure func(Paths) error,
	readFile func(string) ([]byte, error),
) (Config, Paths, error) {
	paths, err := resolve()
	if err != nil {
		return Config{}, Paths{}, err
	}
	if err := ensure(paths); err != nil {
		return Config{}, Paths{}, err
	}

	cfg := Default()
	data, err := readFile(paths.ConfigFile)
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
