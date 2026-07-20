package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/devr-tools/szr/internal/rules"
)

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
