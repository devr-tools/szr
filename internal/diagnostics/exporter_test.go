package diagnostics_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/diagnostics"
)

func TestExporterPostsAllowlistedEventsAndClearsOwnerOnlyOutbox(t *testing.T) {
	var mu sync.Mutex
	var received []diagnostics.Event
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Version int                 `json:"version"`
			Events  []diagnostics.Event `json:"events"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if request.Method != http.MethodPost || payload.Version != diagnostics.SchemaVersion {
			t.Errorf("unexpected request: method=%s payload=%#v", request.Method, payload)
		}
		mu.Lock()
		received = append(received, payload.Events...)
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}

	outbox := filepath.Join(t.TempDir(), "diagnostics-outbox.jsonl")
	exporter, err := diagnostics.NewExporter(diagnostics.ExporterConfig{
		Enabled: true, Endpoint: "https://gateway.example/v1/events", OutboxPath: outbox, FlushInterval: time.Hour,
	}, diagnostics.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("new exporter: %v", err)
	}
	defer exporter.Close()
	event := diagnostics.Event{Version: diagnostics.SchemaVersion, Type: diagnostics.EventRunFinal, RunID: "opaque-run", Profile: "go-test-json"}
	exporter.Enqueue(event)
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	})
	if info, err := os.Stat(outbox); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("outbox mode = %v (%v), want 600", info, err)
	}
	if err := exporter.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0].RunID != event.RunID || received[0].Profile != event.Profile {
		t.Fatalf("received = %#v", received)
	}
	data, err := os.ReadFile(outbox)
	if err != nil || len(data) != 0 {
		t.Fatalf("outbox after ack = %q (%v), want empty", data, err)
	}
}

func TestStoreCloseDeliversQueuedEvent(t *testing.T) {
	var mu sync.Mutex
	var received []diagnostics.Event
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Events []diagnostics.Event `json:"events"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		mu.Lock()
		received = append(received, payload.Events...)
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}

	outbox := filepath.Join(t.TempDir(), "diagnostics-outbox.jsonl")
	exporter, err := diagnostics.NewExporter(diagnostics.ExporterConfig{
		Enabled: true, Endpoint: "https://gateway.example/v1/events", OutboxPath: outbox, FlushInterval: time.Hour,
	}, diagnostics.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("new exporter: %v", err)
	}
	store := diagnostics.NewWithExporter(filepath.Join(filepath.Dir(outbox), "events.jsonl"), exporter)
	event := diagnostics.Event{Version: diagnostics.SchemaVersion, Type: diagnostics.EventRunFinal, RunID: "final-run"}
	if err := store.Append(event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	store.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0].RunID != event.RunID {
		t.Fatalf("received = %#v, want queued final event", received)
	}
	data, err := os.ReadFile(outbox)
	if err != nil || len(data) != 0 {
		t.Fatalf("outbox after close = %q (%v), want empty", data, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestExporterRejectsNonHTTPSOrImplicitConfiguration(t *testing.T) {
	for _, endpoint := range []string{"", "http://gateway.example/events", "https:///events"} {
		exporter, err := diagnostics.NewExporter(diagnostics.ExporterConfig{Enabled: true, Endpoint: endpoint, OutboxPath: "outbox.jsonl"})
		if err == nil || exporter != nil {
			t.Fatalf("endpoint %q: exporter=%#v err=%v, want rejection", endpoint, exporter, err)
		}
	}
	if exporter, err := diagnostics.NewExporter(diagnostics.ExporterConfig{}); err != nil || exporter != nil {
		t.Fatalf("disabled exporter = %#v, %v; want nil, nil", exporter, err)
	}
}

func TestExporterKeepsEventDurablyWhenUploadFails(t *testing.T) {
	directory := t.TempDir()
	outbox := filepath.Join(directory, "diagnostics-outbox.jsonl")
	statusPath := filepath.Join(directory, "diagnostics-exporter-status.json")
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	exporter, err := diagnostics.NewExporter(diagnostics.ExporterConfig{
		Enabled: true, Endpoint: "https://gateway.example/v1/events", OutboxPath: outbox, StatusPath: statusPath, FlushInterval: time.Hour,
	}, diagnostics.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("new exporter: %v", err)
	}
	defer exporter.Close()
	exporter.Enqueue(diagnostics.Event{Version: diagnostics.SchemaVersion, Type: diagnostics.EventRunFinal, RunID: "durable"})
	waitFor(t, func() bool {
		data, err := os.ReadFile(outbox)
		status := diagnostics.ReadExporterStatus(statusPath)
		return err == nil && len(data) > 0 && status.LastFailureAt != nil
	})
	data, err := os.ReadFile(outbox)
	if err != nil || !strings.Contains(string(data), "durable") {
		t.Fatalf("durable outbox = %q (%v)", data, err)
	}
	status := diagnostics.ReadExporterStatus(statusPath)
	if !status.Enabled || status.EndpointHost != "gateway.example" || status.LastFailureAt == nil || status.RetryAt == nil || status.LastError != "transport_error" {
		t.Fatalf("unexpected exporter status: %#v", status)
	}
	if info, err := os.Stat(statusPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("status mode = %v (%v), want 600", info, err)
	}
}

func waitFor(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for asynchronous exporter")
}
