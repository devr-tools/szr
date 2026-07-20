package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/test/testutil"
)

func TestDiagnosticsStatusReportsLocalEventAndOutboxHealth(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	app := testutil.NewTestAppWithPaths(t, paths)
	testutil.MustWriteFile(t, filepath.Join(paths.DataDir, "events.jsonl"), "{\"type\":\"run_final\"}\nnot-json\n")
	testutil.MustWriteFile(t, filepath.Join(paths.DataDir, "diagnostics-outbox.jsonl"), "{\"type\":\"batch\"}\n")

	code, stdout, stderr := testutil.RunApp(t, app, "diagnostics", "status", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var report struct {
		Events struct {
			Exists       bool `json:"exists"`
			Records      int  `json:"records"`
			InvalidLines int  `json:"invalid_lines"`
		} `json:"events"`
		Outbox struct {
			Exists  bool `json:"exists"`
			Records int  `json:"records"`
		} `json:"outbox"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode status: %v; output=%q", err, stdout)
	}
	if !report.Events.Exists || report.Events.Records != 1 || report.Events.InvalidLines != 1 {
		t.Fatalf("unexpected event status: %#v", report.Events)
	}
	if !report.Outbox.Exists || report.Outbox.Records != 1 {
		t.Fatalf("unexpected outbox status: %#v", report.Outbox)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "diagnostics", "status")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "events: records=1") || !strings.Contains(stdout, "outbox: records=1") {
		t.Fatalf("unexpected text status code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestDiagnosticsPurgeRequiresExplicitConfirmationAndRemovesLocalStores(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	app := testutil.NewTestAppWithPaths(t, paths)
	eventsPath := filepath.Join(paths.DataDir, "events.jsonl")
	outboxPath := filepath.Join(paths.DataDir, "diagnostics-outbox.jsonl")
	testutil.MustWriteFile(t, eventsPath, "{}\n")
	testutil.MustWriteFile(t, outboxPath, "{}\n")

	code, stdout, stderr := testutil.RunApp(t, app, "diagnostics", "purge")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "requires --yes") {
		t.Fatalf("unexpected missing confirmation result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(eventsPath); err != nil {
		t.Fatalf("events should remain without confirmation: %v", err)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "diagnostics", "purge", "--yes")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "files=2") {
		t.Fatalf("unexpected purge result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, path := range []string{eventsPath, outboxPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got %v", path, err)
		}
	}
}

func TestDiagnosticsStatusReportsPersistedExporterHealthAndFlushRequiresExport(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	app := testutil.NewTestAppWithPaths(t, paths)
	statusPath := filepath.Join(paths.DataDir, "diagnostics-exporter-status.json")
	testutil.MustWriteFile(t, statusPath, `{"enabled":true,"endpoint_host":"gateway.example","dropped":3,"last_error":"transport_error"}`+"\n")

	code, stdout, stderr := testutil.RunApp(t, app, "diagnostics", "status", "--json")
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"endpoint_host":"gateway.example"`) || !strings.Contains(stdout, `"dropped":3`) {
		t.Fatalf("unexpected exporter status result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "diagnostics", "flush")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "export is disabled") {
		t.Fatalf("unexpected disabled flush code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestDiagnosticsRejectsInvalidArguments(t *testing.T) {
	app := testutil.NewTestApp(t)
	for _, args := range [][]string{
		{"diagnostics"},
		{"diagnostics", "unknown"},
		{"diagnostics", "status", "--bad"},
		{"diagnostics", "purge", "--bad"},
	} {
		code, _, stderr := testutil.RunApp(t, app, args...)
		if code != 2 || !strings.Contains(stderr, "diagnostics") {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, stderr)
		}
	}
}
