package diagnostics

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExporterStatus is durable health metadata for the optional exporter. It is
// intentionally separate from the event stream so `diagnostics status` never
// needs to make a network request.
type ExporterStatus struct {
	Enabled       bool       `json:"enabled"`
	EndpointHost  string     `json:"endpoint_host,omitempty"`
	Dropped       uint64     `json:"dropped"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	RetryAt       *time.Time `json:"retry_at,omitempty"`
}

func (e *Exporter) recordSuccess() {
	now := time.Now().UTC()
	e.mu.Lock()
	e.status.Dropped = e.dropped
	e.status.LastSuccessAt = &now
	e.status.LastError = ""
	e.status.RetryAt = nil
	e.mu.Unlock()
	e.persistStatus()
}

func (e *Exporter) recordFailure(err error) {
	now := time.Now().UTC()
	retryAt := now.Add(e.cfg.FlushInterval)
	e.mu.Lock()
	e.status.Dropped = e.dropped
	e.status.LastFailureAt = &now
	// Do not persist transport errors: they may contain a URL or proxy details.
	e.status.LastError = exporterErrorKind(err)
	e.status.RetryAt = &retryAt
	e.mu.Unlock()
	e.persistStatus()
}

func exporterErrorKind(err error) string {
	if err == nil {
		return ""
	}
	var urlError *url.Error
	if errors.As(err, &urlError) {
		return "transport_error"
	}
	if strings.HasPrefix(err.Error(), "diagnostics export returned HTTP ") {
		return err.Error()
	}
	return "local_error"
}

func (e *Exporter) persistStatus() {
	if e.cfg.StatusPath == "" {
		return
	}
	e.mu.Lock()
	status := e.status
	e.mu.Unlock()
	data, err := json.Marshal(status)
	if err == nil {
		_ = writeOwnerOnly(e.cfg.StatusPath, append(data, '\n'))
	}
}

// ReadExporterStatus loads status written by an exporter in a prior process.
// Invalid/missing state is treated as absent so diagnostics remain usable.
func ReadExporterStatus(path string) ExporterStatus { return readExporterStatus(path) }

func readExporterStatus(path string) ExporterStatus {
	if path == "" {
		return ExporterStatus{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ExporterStatus{}
	}
	var status ExporterStatus
	if json.Unmarshal(data, &status) != nil {
		return ExporterStatus{}
	}
	return status
}

func writeOwnerOnly(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".diagnostics-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
