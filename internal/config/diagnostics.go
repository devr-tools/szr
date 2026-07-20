package config

type Diagnostics struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	// MaxOutboxMB bounds the on-disk queue for events waiting to be exported.
	MaxOutboxMB int `json:"max_outbox_mb"`
}

const DefaultDiagnosticsMaxOutboxMB = 8

// GatewayHints contains only connection metadata. The bearer token itself is
// read from the named environment variable when a user explicitly refreshes
// hints; it is never written to config or diagnostics files.
type GatewayHints struct {
	Enabled          bool   `json:"enabled"`
	Endpoint         string `json:"endpoint"`
	AuthTokenEnv     string `json:"auth_token_env"`
	SigningPublicKey string `json:"signing_public_key"`
}
