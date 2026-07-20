package diagnostics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

// ExporterConfig is deliberately small: diagnostics export must be explicitly
// enabled and its destination must always be HTTPS.
type ExporterConfig struct {
	Enabled        bool
	Endpoint       string
	OutboxPath     string
	MaxOutboxBytes int64
	BatchSize      int
	FlushInterval  time.Duration
	// StatusPath stores sanitized exporter health for diagnostics status. It
	// contains no events, endpoint URL, credentials, or response bodies.
	StatusPath string
}

// Exporter asynchronously persists allowlisted events to an owner-only JSONL
// outbox and uploads batches. It never performs network I/O on Append's
// caller goroutine. Events that cannot fit are dropped oldest-first.
type Exporter struct {
	cfg    ExporterConfig
	client *http.Client
	wake   chan struct{}
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once

	mu       sync.Mutex
	outboxMu sync.Mutex
	flushMu  sync.Mutex
	dropped  uint64
	status   ExporterStatus
}

//nolint:maintidx // Constructor validation is kept next to exporter defaults.
func NewExporter(cfg ExporterConfig, options ...ExporterOption) (*Exporter, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		return nil, errors.New("diagnostics exporter requires an explicit HTTPS endpoint")
	}
	if cfg.OutboxPath == "" {
		return nil, errors.New("diagnostics exporter requires an outbox path")
	}
	if cfg.MaxOutboxBytes <= 0 {
		cfg.MaxOutboxBytes = defaultMaxOutboxBytes
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	exporter := &Exporter{
		cfg:    cfg,
		client: &http.Client{Timeout: 3 * time.Second},
		wake:   make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	exporter.status = readExporterStatus(cfg.StatusPath)
	exporter.status.Enabled = true
	exporter.status.EndpointHost = endpoint.Host
	exporter.dropped = exporter.status.Dropped
	exporter.persistStatus()
	for _, option := range options {
		option(exporter)
	}
	go exporter.run()
	return exporter, nil
}

// Enqueue durably stores the allowlisted event before signalling the
// background uploader. It performs no network I/O; an interrupted process
// leaves the event in the owner-only outbox for the next invocation.
func (e *Exporter) Enqueue(event Event) {
	if e == nil {
		return
	}
	if err := e.append(event); err != nil {
		e.mu.Lock()
		e.dropped++
		e.status.Dropped = e.dropped
		e.mu.Unlock()
		e.persistStatus()
		return
	}
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Exporter) Close() {
	if e == nil {
		return
	}
	e.once.Do(func() { close(e.stop) })
	<-e.done
}

func (e *Exporter) Dropped() uint64 {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dropped
}

// Status returns a snapshot of persistent exporter health without any network
// I/O. The returned values are safe to show in `szr diagnostics status`.
func (e *Exporter) Status() ExporterStatus {
	if e == nil {
		return ExporterStatus{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Exporter) run() {
	defer close(e.done)
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()
	// A prior one-shot invocation may have left durable events behind.
	e.flush()
	for {
		select {
		case <-e.wake:
			e.flush()
		case <-ticker.C:
			e.flush()
		case <-e.stop:
			e.flush()
			return
		}
	}
}

// Flush is synchronous only for administrative/testing callers. Normal
// command execution uses the exporter's background ticker.
func (e *Exporter) Flush(ctx context.Context) error {
	if e == nil {
		return nil
	}
	return e.flushContext(ctx)
}

func (e *Exporter) flush() { _ = e.flushContext(context.Background()) }

//nolint:maintidx // A flush must preserve the complete request/ack sequence.
func (e *Exporter) flushContext(ctx context.Context) error {
	e.flushMu.Lock()
	defer e.flushMu.Unlock()
	e.outboxMu.Lock()
	events, err := readOutbox(e.cfg.OutboxPath, e.cfg.BatchSize)
	e.outboxMu.Unlock()
	if err != nil {
		e.recordFailure(err)
		return err
	}
	if len(events) == 0 {
		return err
	}
	body, err := json.Marshal(struct {
		Version int     `json:"version"`
		Events  []Event `json:"events"`
	}{Version: SchemaVersion, Events: events})
	if err != nil {
		e.recordFailure(err)
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		e.recordFailure(err)
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		e.recordFailure(err)
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("diagnostics export returned HTTP %d", response.StatusCode)
		e.recordFailure(err)
		return err
	}
	e.outboxMu.Lock()
	defer e.outboxMu.Unlock()
	err = removeOutboxPrefix(e.cfg.OutboxPath, len(events))
	if err != nil {
		e.recordFailure(err)
		return err
	}
	e.recordSuccess()
	return nil
}

//nolint:maintidx // Bounded outbox pruning is intentionally linear and explicit.
func (e *Exporter) append(event Event) error {
	e.outboxMu.Lock()
	defer e.outboxMu.Unlock()
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	data, err := os.ReadFile(e.cfg.OutboxPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for len(data)+len(line) > int(e.cfg.MaxOutboxBytes) && len(data) > 0 {
		if next := bytes.IndexByte(data, '\n'); next >= 0 {
			data = data[next+1:]
			e.mu.Lock()
			e.dropped++
			e.status.Dropped = e.dropped
			e.mu.Unlock()
			e.persistStatus()
		} else {
			data = nil
		}
	}
	if len(line) > int(e.cfg.MaxOutboxBytes) {
		e.mu.Lock()
		e.dropped++
		e.status.Dropped = e.dropped
		e.mu.Unlock()
		e.persistStatus()
		return nil
	}
	return writeOutbox(e.cfg.OutboxPath, append(data, line...))
}

func readOutbox(path string, limit int) ([]Event, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && len(events) < limit {
		var event Event
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			events = append(events, event)
		}
	}
	return events, scanner.Err()
}

func removeOutboxPrefix(path string, count int) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for count > 0 && len(data) > 0 {
		next := bytes.IndexByte(data, '\n')
		if next < 0 {
			data = nil
			break
		}
		data = data[next+1:]
		count--
	}
	return writeOutbox(path, data)
}

func writeOutbox(path string, data []byte) error {
	return writeOwnerOnly(path, data)
}
