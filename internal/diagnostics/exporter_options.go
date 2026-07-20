package diagnostics

import (
	"net/http"
	"time"
)

const (
	defaultMaxOutboxBytes = 8 << 20
	defaultBatchSize      = 50
	defaultFlushInterval  = 2 * time.Second
)

// ExporterOption customizes transport behavior for controlled integrations.
type ExporterOption func(*Exporter)

func WithHTTPClient(client *http.Client) ExporterOption {
	return func(exporter *Exporter) {
		if client != nil {
			exporter.client = client
		}
	}
}
