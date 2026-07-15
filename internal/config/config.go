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
	TeeOnFailure bool `json:"tee_on_failure"`
	// TeeMaxFileMB caps a single tee artifact file in MiB. A capture over the
	// cap keeps the head and tail of the stream around a truncation marker.
	TeeMaxFileMB int `json:"tee_max_file_mb"`
	// TeeMaxDirFiles bounds how many tee artifacts stay on disk; pruning
	// removes the oldest first.
	TeeMaxDirFiles int `json:"tee_max_dir_files"`
	// TeeMaxDirMB bounds the tee directory's total size in MiB.
	TeeMaxDirMB int `json:"tee_max_dir_mb"`
	// CostRatePerMtok prices input tokens in USD per million for spread cost
	// reporting. The default approximates mainstream frontier-model input
	// pricing; set it to match the model you actually run.
	CostRatePerMtok     float64     `json:"cost_rate_per_mtok"`
	MaxPreviewLines     int         `json:"max_preview_lines"`
	MaxMatchGroups      int         `json:"max_match_groups"`
	ReasoningBudgetMode string      `json:"reasoning_budget_mode"`
	Advanced            Advanced    `json:"advanced"`
	UpdateCheck         UpdateCheck `json:"update_check"`
	ProjectRules        rules.File  `json:"-"`
}

type Advanced struct {
	AggressivePrepareRewrites bool `json:"aggressive_prepare_rewrites"`
	NoisePrefiltering         bool `json:"noise_prefiltering"`
	AdaptiveBudgets           bool `json:"adaptive_budgets"`
	EarlyCaptureStop          bool `json:"early_capture_stop"`
	SemanticCompaction        bool `json:"semantic_compaction"`
	CompressionContract       bool `json:"compression_contract"`
	CompactArtifactRefs       bool `json:"compact_artifact_refs"`
	RetentionVerifier         bool `json:"retention_verifier"`
	// SessionDedup replaces re-renders of byte-identical recent output with a
	// short expandable reference.
	SessionDedup bool `json:"session_dedup"`
	// SessionDedupWindowMinutes bounds how far back an identical run may be
	// referenced.
	SessionDedupWindowMinutes int `json:"session_dedup_window_minutes"`
	// DeltaRender replaces a rerun's render with a compact change digest
	// against the previous run's stored output when the digest is strictly
	// cheaper. Rides on the session dedup store and window.
	DeltaRender bool `json:"delta_render"`
	// ProjectFilters enables loading declarative filter specs from the
	// working directory's .szr/filters. Off by default because project
	// files arrive with checkouts rather than from the user.
	ProjectFilters bool `json:"project_filters"`
}

// DefaultSessionDedupWindowMinutes is the recency window used when the
// configured window is missing or invalid.
const DefaultSessionDedupWindowMinutes = 30

// Tee retention defaults; zero or negative configured values fall back to
// these rather than meaning "unlimited".
const (
	DefaultTeeMaxFileMB   = 4
	DefaultTeeMaxDirFiles = 200
	DefaultTeeMaxDirMB    = 256
)

// DefaultCostRatePerMtok approximates a mainstream frontier-model input rate
// in USD per million tokens; override via cost_rate_per_mtok or --rate.
const DefaultCostRatePerMtok = 3.0

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
		TeeMaxFileMB:        DefaultTeeMaxFileMB,
		TeeMaxDirFiles:      DefaultTeeMaxDirFiles,
		TeeMaxDirMB:         DefaultTeeMaxDirMB,
		CostRatePerMtok:     DefaultCostRatePerMtok,
		MaxPreviewLines:     12,
		MaxMatchGroups:      8,
		ReasoningBudgetMode: ReasoningBudgetStandard,
		Advanced: Advanced{
			AggressivePrepareRewrites: true,
			NoisePrefiltering:         true,
			// Safe to enable by default: history compaction keeps the
			// per-command suggestion lookup cheap, and adaptations are
			// conservatively capped by the history budget adapter.
			AdaptiveBudgets:     true,
			EarlyCaptureStop:    true,
			SemanticCompaction:  true,
			CompressionContract: true,
			CompactArtifactRefs: true,
			// The retention verifier is szr's fidelity guarantee made
			// mechanical: a render must never be less informative than the
			// tokens it spends. On by default because it is the product's
			// identity, not an optimization.
			RetentionVerifier: true,
			// Session dedup is safe by default: a reference is only emitted
			// for byte-identical raw output whose stored artifact still
			// verifies, so expansion always recovers the exact bytes.
			SessionDedup:              true,
			SessionDedupWindowMinutes: DefaultSessionDedupWindowMinutes,
			// Delta rendering is safe by default for the same reason: a
			// digest is only emitted against a verified stored baseline, only
			// when strictly cheaper than the render it replaces, and never
			// drops a critical changed line.
			DeltaRender: true,
		},
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
