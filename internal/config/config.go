package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/devr-tools/szr/internal/rules"
)

const appName = "szr"

type Config struct {
	TeeOnFailure        bool        `json:"tee_on_failure"`
	MaxPreviewLines     int         `json:"max_preview_lines"`
	MaxMatchGroups      int         `json:"max_match_groups"`
	ReasoningBudgetMode string      `json:"reasoning_budget_mode"`
	UpdateCheck         UpdateCheck `json:"update_check"`
	ProjectRules        rules.File  `json:"-"`
}

type UpdateCheck struct {
	Enabled       bool `json:"enabled"`
	IntervalHours int  `json:"interval_hours"`
	AutoUpdate    bool `json:"auto_update"`
}

type Paths struct {
	ConfigDir       string
	ConfigFile      string
	DataDir         string
	HistoryFile     string
	TeeDir          string
	ProjectDir      string
	ProjectRuleFile string
}

func Default() Config {
	return Config{
		TeeOnFailure:        true,
		MaxPreviewLines:     12,
		MaxMatchGroups:      8,
		ReasoningBudgetMode: ReasoningBudgetStandard,
		UpdateCheck: UpdateCheck{
			Enabled:       false,
			IntervalHours: 24,
		},
	}
}

func ResolvePaths() (Paths, error) {
	return ResolvePathsWith(os.UserConfigDir, os.UserCacheDir, os.UserHomeDir)
}

func EnsurePaths(paths Paths) error {
	return EnsurePathsWith(paths, os.MkdirAll)
}

func Load() (Config, Paths, error) {
	return LoadWith(ResolvePaths, EnsurePaths, os.Getwd, os.Stat, os.ReadFile)
}

func Save(paths Paths, cfg Config) error {
	return SaveWith(paths, cfg, EnsurePaths, os.WriteFile)
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
	getwd func() (string, error),
	stat func(string) (os.FileInfo, error),
	readFile func(string) ([]byte, error),
) (Config, Paths, error) {
	paths, err := resolveAndEnsurePaths(resolve, ensure)
	if err != nil {
		return Config{}, Paths{}, err
	}

	cfg, err := loadUserConfig(paths.ConfigFile, readFile)
	if err != nil {
		return Config{}, Paths{}, err
	}
	cfg, err = Normalize(cfg)
	if err != nil {
		return Config{}, Paths{}, err
	}

	cwd, err := getwd()
	if err != nil {
		return Config{}, Paths{}, err
	}
	projectRuleFile, _, err := rules.DiscoverWith(cwd, stat)
	if err != nil {
		return Config{}, Paths{}, err
	}
	if projectRuleFile == "" {
		return cfg, paths, nil
	}
	return attachProjectRules(cfg, paths, projectRuleFile, readFile)
}

func resolveAndEnsurePaths(
	resolve func() (Paths, error),
	ensure func(Paths) error,
) (Paths, error) {
	paths, err := resolve()
	if err != nil {
		return Paths{}, err
	}
	if err := ensure(paths); err != nil {
		return Paths{}, err
	}
	return paths, nil
}

func loadUserConfig(configFile string, readFile func(string) ([]byte, error)) (Config, error) {
	cfg := Default()
	data, err := readFile(configFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if err := applyConfigAliases(&cfg, data); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyConfigAliases(cfg *Config, data []byte) error {
	var aliases struct {
		ReasoningBudget     string `json:"reasoning_budget"`
		ReasoningBudgetMode string `json:"reasoning_budget_mode"`
	}
	if err := json.Unmarshal(data, &aliases); err != nil {
		return err
	}
	if aliases.ReasoningBudget != "" && aliases.ReasoningBudgetMode != "" && aliases.ReasoningBudget != aliases.ReasoningBudgetMode {
		return errors.New("config reasoning_budget and reasoning_budget_mode disagree")
	}
	if aliases.ReasoningBudget != "" {
		cfg.ReasoningBudgetMode = aliases.ReasoningBudget
	}
	return nil
}

func attachProjectRules(
	cfg Config,
	paths Paths,
	projectRuleFile string,
	readFile func(string) ([]byte, error),
) (Config, Paths, error) {
	projectData, err := readFile(projectRuleFile)
	if err != nil {
		return Config{}, Paths{}, err
	}
	projectRules, err := rules.ParseFile(projectRuleFile, projectData)
	if err != nil {
		return Config{}, Paths{}, err
	}
	cfg.ProjectRules = projectRules
	paths.ProjectDir = filepath.Dir(projectRuleFile)
	paths.ProjectRuleFile = projectRuleFile
	return cfg, paths, nil
}

func SaveWith(
	paths Paths,
	cfg Config,
	ensure func(Paths) error,
	writeFile func(string, []byte, os.FileMode) error,
) error {
	if err := ensure(paths); err != nil {
		return err
	}
	cfg, err := Normalize(cfg)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFile(paths.ConfigFile, data, 0o644)
}
