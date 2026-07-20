package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

const (
	ReasoningBudgetStandard   = "standard"
	ReasoningBudgetAgent      = "agent"
	ReasoningBudgetAggressive = "aggressive"
)

func NormalizeReasoningBudgetMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", ReasoningBudgetStandard:
		return ReasoningBudgetStandard, nil
	case ReasoningBudgetAgent, "loop", "agent-loop":
		return ReasoningBudgetAgent, nil
	case ReasoningBudgetAggressive, "tight", "spread":
		return ReasoningBudgetAggressive, nil
	default:
		return "", fmt.Errorf("invalid reasoning budget mode %q (want %q, %q, or %q)", value, ReasoningBudgetStandard, ReasoningBudgetAgent, ReasoningBudgetAggressive)
	}
}

func ResolveReasoningBudgetMode(value string) string {
	mode, err := NormalizeReasoningBudgetMode(value)
	if err != nil {
		return ReasoningBudgetStandard
	}
	return mode
}

func Normalize(cfg Config) (Config, error) {
	mode, err := NormalizeReasoningBudgetMode(cfg.ReasoningBudgetMode)
	if err != nil {
		return Config{}, err
	}
	cfg.ReasoningBudgetMode = mode
	if cfg.UpdateCheck.IntervalHours <= 0 {
		cfg.UpdateCheck.IntervalHours = Default().UpdateCheck.IntervalHours
	}
	if cfg.Advanced.SessionDedupWindowMinutes <= 0 {
		cfg.Advanced.SessionDedupWindowMinutes = DefaultSessionDedupWindowMinutes
	}
	if cfg.TeeMaxFileMB <= 0 {
		cfg.TeeMaxFileMB = DefaultTeeMaxFileMB
	}
	if cfg.TeeMaxDirFiles <= 0 {
		cfg.TeeMaxDirFiles = DefaultTeeMaxDirFiles
	}
	if cfg.TeeMaxDirMB <= 0 {
		cfg.TeeMaxDirMB = DefaultTeeMaxDirMB
	}
	if cfg.CostRatePerMtok <= 0 {
		cfg.CostRatePerMtok = DefaultCostRatePerMtok
	}
	if cfg.Diagnostics.MaxOutboxMB <= 0 {
		cfg.Diagnostics.MaxOutboxMB = DefaultDiagnosticsMaxOutboxMB
	}
	if cfg.Diagnostics.Enabled {
		endpoint, err := url.Parse(cfg.Diagnostics.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			return Config{}, fmt.Errorf("diagnostics endpoint must be an explicit HTTPS URL when diagnostics are enabled")
		}
	}
	if cfg.GatewayHints.Enabled {
		endpoint, err := url.Parse(cfg.GatewayHints.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			return Config{}, fmt.Errorf("gateway hint endpoint must be an explicit HTTPS URL when gateway hints are enabled")
		}
		if !validEnvironmentVariableName(cfg.GatewayHints.AuthTokenEnv) {
			return Config{}, fmt.Errorf("gateway hint auth_token_env must name an environment variable when gateway hints are enabled")
		}
		key, err := base64.StdEncoding.DecodeString(cfg.GatewayHints.SigningPublicKey)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("gateway hint signing_public_key must be a base64 Ed25519 public key when gateway hints are enabled")
		}
	}
	return cfg, nil
}

func validEnvironmentVariableName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || (i > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
