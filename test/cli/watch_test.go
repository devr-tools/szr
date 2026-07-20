package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/diagnostics"
	"github.com/devr-tools/szr/test/testutil"
)

func TestWatchEmitsSanitizedFinalExecutionEvent(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	// Use an app with its data directory under a known path so the test can
	// inspect the persisted stream as well as the public watch interface.
	app := testutil.NewTestAppWithPaths(t, paths)

	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "eventcmd", "#!/bin/sh\necho private-output\n")

	code, stdout, stderr := testutil.RunApp(t, app, "run", "eventcmd")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "private-output") {
		t.Fatalf("unexpected command result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "watch", "--jsonl", "--once")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected watch result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var event diagnostics.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &event); err != nil {
		t.Fatalf("decode watch event: %v; output=%q", err, stdout)
	}
	if event.Version != diagnostics.SchemaVersion || event.Type != diagnostics.EventRunFinal || event.RunID == "" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.RawTokensEst <= 0 || event.EmittedTokensEst <= 0 || event.ExitClass != "success" {
		t.Fatalf("missing final metrics: %#v", event)
	}

	persisted, err := os.ReadFile(filepath.Join(paths.DataDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, forbidden := range []string{"eventcmd", "private-output", paths.DataDir} {
		if strings.Contains(string(persisted), forbidden) {
			t.Fatalf("event stream leaked %q: %s", forbidden, persisted)
		}
	}
}

func TestWatchRejectsUnknownFlag(t *testing.T) {
	app := testutil.NewTestApp(t)
	code, stdout, stderr := testutil.RunApp(t, app, "watch", "--bad")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown watch flag --bad") {
		t.Fatalf("unexpected watch flag result code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
