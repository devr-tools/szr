package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// A history line longer than the reader's limit used to fail every szr
// subcommand that reads history ("failed to read history: bufio.Scanner:
// token too long"), with no way out short of deleting the file. The oversized
// record is skipped instead.
func TestSpreadSurvivesOversizedHistoryLine(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)

	lines := []string{
		`{"command":"git status --short","profile":"git-status","raw_tokens":100,"filtered_tokens":40,"saved_tokens":60,"savings_pct":60}`,
		strings.Repeat("x", 200*1024),
		`{"command":"go test ./...","profile":"go-test","raw_tokens":500,"filtered_tokens":50,"saved_tokens":450,"savings_pct":90}`,
	}
	if err := os.WriteFile(paths.HistoryFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("seed history: %v", err)
	}

	store := history.New(paths.HistoryFile)
	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))

	for _, args := range [][]string{{"spread"}, {"spread", "--cost"}} {
		code, stdout, stderr := testutil.RunApp(t, app, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("%v failed code=%d stderr=%q", args, code, stderr)
		}
		if !strings.Contains(stdout, "Total commands:  2") {
			t.Fatalf("%v should report the two readable records, got:\n%s", args, stdout)
		}
	}
}
