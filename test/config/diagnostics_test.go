package config_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestNormalizeDiagnosticsRequiresExplicitHTTPSOnlyWhenEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Diagnostics.Enabled = true
	cfg.Diagnostics.Endpoint = "http://gateway.example/v1/events"
	if _, err := config.Normalize(cfg); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("normalize error = %v, want HTTPS validation", err)
	}
	cfg.Diagnostics.Endpoint = "https://gateway.example/v1/events"
	cfg.Diagnostics.MaxOutboxMB = 0
	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Diagnostics.MaxOutboxMB != config.DefaultDiagnosticsMaxOutboxMB {
		t.Fatalf("outbox default = %d", normalized.Diagnostics.MaxOutboxMB)
	}
}
