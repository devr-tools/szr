package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/diagnostics"
)

// diagnosticsOutboxFile is shared with the optional exporter. Keeping the
// file name here lets status and purge work even when exporting is disabled.
const diagnosticsOutboxFile = "diagnostics-outbox.jsonl"

const diagnosticsExporterStatusFile = "diagnostics-exporter-status.json"

// runDiagnostics manages only local diagnostic data. It deliberately makes no
// network calls, so it is safe to use when investigating export failures.
func (a *App) runDiagnostics(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: diagnostics requires a subcommand: status, flush, or purge")
		return 2
	}
	switch args[0] {
	case "status":
		return a.runDiagnosticsStatus(args[1:])
	case "purge":
		return a.runDiagnosticsPurge(args[1:])
	case "flush":
		return a.runDiagnosticsFlush(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown diagnostics subcommand %s\n", args[0])
		return 2
	}
}

type diagnosticsStorageStatus struct {
	Exists       bool       `json:"exists"`
	Bytes        int64      `json:"bytes"`
	Records      int        `json:"records"`
	InvalidLines int        `json:"invalid_lines,omitempty"`
	ModifiedAt   *time.Time `json:"modified_at,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type diagnosticsStatusReport struct {
	Events   diagnosticsStorageStatus   `json:"events"`
	Outbox   diagnosticsStorageStatus   `json:"outbox"`
	Exporter diagnostics.ExporterStatus `json:"exporter"`
}

//nolint:maintidx // Keeps status parsing and rendering in one command handler.
func (a *App) runDiagnosticsStatus(args []string) int {
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown diagnostics status flag %s\n", arg)
			return 2
		}
	}

	report := diagnosticsStatusReport{
		Events:   inspectDiagnosticsFile(filepath.Join(a.paths.DataDir, "events.jsonl")),
		Outbox:   inspectDiagnosticsFile(filepath.Join(a.paths.DataDir, diagnosticsOutboxFile)),
		Exporter: diagnostics.ReadExporterStatus(filepath.Join(a.paths.DataDir, diagnosticsExporterStatusFile)),
	}
	if asJSON {
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to write diagnostics status: %v\n", err)
			return 1
		}
		return 0
	}

	printDiagnosticsStorageStatus("events", report.Events)
	printDiagnosticsStorageStatus("outbox", report.Outbox)
	printDiagnosticsExporterStatus(report.Exporter)
	return 0
}

func printDiagnosticsExporterStatus(status diagnostics.ExporterStatus) {
	if !status.Enabled {
		fmt.Fprintln(os.Stdout, "exporter: disabled")
		return
	}
	fmt.Fprintf(os.Stdout, "exporter: enabled endpoint_host=%s dropped=%d", status.EndpointHost, status.Dropped)
	if status.LastSuccessAt != nil {
		fmt.Fprintf(os.Stdout, " last_success_at=%s", status.LastSuccessAt.Format(time.RFC3339))
	}
	if status.LastFailureAt != nil {
		fmt.Fprintf(os.Stdout, " last_failure_at=%s", status.LastFailureAt.Format(time.RFC3339))
	}
	if status.LastError != "" {
		fmt.Fprintf(os.Stdout, " last_error=%s", status.LastError)
	}
	if status.RetryAt != nil {
		fmt.Fprintf(os.Stdout, " retry_at=%s", status.RetryAt.Format(time.RFC3339))
	}
	fmt.Fprintln(os.Stdout)
}

// runDiagnosticsFlush is an explicit network action. It has a short bounded
// deadline and never runs as part of a wrapped command.
func (a *App) runDiagnosticsFlush(args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "szr: unknown diagnostics flush flag %s\n", args[0])
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	enabled, err := a.events.Flush(ctx)
	if !enabled {
		fmt.Fprintln(os.Stderr, "szr: diagnostics export is disabled")
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: diagnostics flush failed: %s\n", diagnosticsFlushError(err))
		return 1
	}
	fmt.Fprintln(os.Stdout, "diagnostics outbox flushed")
	return 0
}

func diagnosticsFlushError(err error) string {
	// Transport errors can include configured endpoint or proxy information.
	if err == context.DeadlineExceeded {
		return "timeout"
	}
	return "export failed; inspect diagnostics status"
}

//nolint:maintidx // The report deliberately includes every local I/O outcome.
func inspectDiagnosticsFile(path string) diagnosticsStorageStatus {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return diagnosticsStorageStatus{}
	}
	if err != nil {
		return diagnosticsStorageStatus{Error: err.Error()}
	}
	modifiedAt := info.ModTime().UTC()
	status := diagnosticsStorageStatus{Exists: true, Bytes: info.Size(), ModifiedAt: &modifiedAt}

	file, err := os.Open(path)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var value json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			status.InvalidLines++
			continue
		}
		status.Records++
	}
	if err := scanner.Err(); err != nil {
		status.Error = err.Error()
	}
	return status
}

func printDiagnosticsStorageStatus(name string, status diagnosticsStorageStatus) {
	if status.Error != "" {
		fmt.Printf("%s: error: %s\n", name, status.Error)
		return
	}
	if !status.Exists {
		fmt.Printf("%s: empty\n", name)
		return
	}
	fmt.Printf("%s: records=%d bytes=%d", name, status.Records, status.Bytes)
	if status.InvalidLines > 0 {
		fmt.Printf(" invalid_lines=%d", status.InvalidLines)
	}
	if status.ModifiedAt != nil {
		fmt.Printf(" modified_at=%s", status.ModifiedAt.Format(time.RFC3339))
	}
	fmt.Fprintln(os.Stdout)
}

func (a *App) runDiagnosticsPurge(args []string) int {
	if len(args) != 1 || args[0] != "--yes" {
		fmt.Fprintln(os.Stderr, "szr: diagnostics purge requires --yes")
		return 2
	}

	removed := 0
	for _, name := range []string{"events.jsonl", diagnosticsOutboxFile, diagnosticsExporterStatusFile} {
		path := filepath.Join(a.paths.DataDir, name)
		if err := os.Remove(path); err == nil {
			removed++
		} else if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "szr: failed to purge diagnostics %s: %v\n", name, err)
			return 1
		}
	}
	fmt.Fprintf(os.Stdout, "purged local diagnostics: files=%d\n", removed)
	return 0
}
