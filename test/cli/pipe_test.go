package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localcmd "github.com/devr-tools/szr/internal/cli/localcmd"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/test/testutil"
)

func runPipeCommand(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := localcmd.Runtime{Stdout: &stdout, Stderr: &stderr}
	code := localcmd.RunPipe(rt, config.Default(), args, strings.NewReader(input), false)
	return code, stdout.String(), stderr.String()
}

func pipeLogFixture(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i%40 == 11 {
			fmt.Fprintf(&b, "2026-07-02T10:%02d:%02dZ ERROR upstream timeout shard=%d\n", i/60%60, i%60, i%3)
			continue
		}
		fmt.Fprintf(&b, "2026-07-02T10:%02d:%02dZ INFO request served path=/api/items status=200\n", i/60%60, i%60)
	}
	return b.String()
}

func TestPipeRoutesLogTextToSeveritySummary(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := runPipeCommand(t, pipeLogFixture(500))
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "lines:") || !strings.Contains(stdout, "ERROR") {
		t.Fatalf("expected severity histogram with errors, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ERROR upstream timeout") {
		t.Fatalf("expected deduplicated error message, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "status=200\n2026-") {
		t.Fatalf("expected reduced output, looks raw:\n%s", stdout)
	}
}

func TestPipeRoutesUniformJSONArrayToTable(t *testing.T) {
	t.Parallel()

	rows := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"name":"svc-%d","state":"running"}`, i, i))
	}
	input := "[" + strings.Join(rows, ",") + "]"

	code, stdout, _ := runPipeCommand(t, input)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "uniform objects, cols:") {
		t.Fatalf("expected tabular JSON preview, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "cols: id|name|state") {
		t.Fatalf("expected declared columns, got:\n%s", stdout)
	}
}

func TestPipeRoutesSingleJSONObjectToPreview(t *testing.T) {
	t.Parallel()

	code, stdout, _ := runPipeCommand(t, `{"service":"api","replicas":3,"healthy":true}`)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "service") || !strings.Contains(stdout, "replicas") {
		t.Fatalf("expected JSON structural preview, got:\n%s", stdout)
	}
}

func TestPipeRoutesBinaryContentToDigest(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	for i := 0; i < 2048; i++ {
		b.WriteByte(byte(i % 251))
	}
	b.WriteString("embedded-needle-string")
	for i := 0; i < 2048; i++ {
		b.WriteByte(byte(i % 233))
	}

	code, stdout, _ := runPipeCommand(t, b.String())
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.HasPrefix(stdout, "binary output: ") {
		t.Fatalf("expected binary digest headline, got:\n%s", stdout)
	}
}

func TestPipeRoutesPlainTextThroughFailureReducer(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"building module alpha",
		"building module beta",
		"error: cannot resolve symbol Frobnicate",
		"  at pkg/widget/frobnicate.go:42",
		"building module gamma",
	}, "\n") + "\n"

	code, stdout, _ := runPipeCommand(t, input)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "error: cannot resolve symbol Frobnicate") {
		t.Fatalf("expected failure signal to survive, got:\n%s", stdout)
	}
}

func TestPipeHintDiffForcesPatchSummary(t *testing.T) {
	t.Parallel()

	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1,4 @@",
		"+import \"fmt\"",
		" func main() {",
	}, "\n") + "\n"

	code, stdout, _ := runPipeCommand(t, diff, "--hint", "diff")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "diff --git a/main.go b/main.go") {
		t.Fatalf("expected patch summary with file header, got:\n%s", stdout)
	}
}

func TestPipeHintTestSummarizesGoTestJSON(t *testing.T) {
	t.Parallel()

	events := strings.Join([]string{
		`{"Action":"fail","Package":"example.com/pkg","Test":"TestBroken"}`,
		`{"Action":"fail","Package":"example.com/pkg"}`,
		`{"Action":"pass","Package":"example.com/other"}`,
	}, "\n") + "\n"

	code, stdout, _ := runPipeCommand(t, events, "--hint", "test")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "packages: pass=1 fail=1") {
		t.Fatalf("expected go test package summary, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "TestBroken") {
		t.Fatalf("expected failing test name, got:\n%s", stdout)
	}
}

func TestPipeHintLogForcesLogPathOnUnshapedText(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("plain line without any log shape\n", 30)
	code, stdout, _ := runPipeCommand(t, input, "--hint", "log")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "30 lines:") {
		t.Fatalf("expected forced log histogram over plain text, got:\n%s", stdout)
	}
}

func TestPipeHintJSONFallsBackToCompactLinesOnNonJSON(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("not json at all\n", 40)
	code, stdout, _ := runPipeCommand(t, input, "--hint", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "not json at all") {
		t.Fatalf("expected compact-line fallback, got:\n%s", stdout)
	}
	if strings.Count(stdout, "not json at all") >= 40 {
		t.Fatalf("expected folded output, got raw:\n%s", stdout)
	}
}

func TestPipeTruncatesOversizedInputWithNote(t *testing.T) {
	t.Parallel()

	line := "filler line for the pipe truncation test with some padding text\n"
	var b strings.Builder
	b.Grow(9 << 20)
	for b.Len() < 9<<20 {
		b.WriteString(line)
	}
	b.WriteString("the very last line of the oversized stream\n")

	code, stdout, _ := runPipeCommand(t, b.String())
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "note: input exceeded the 8MB pipe buffer") {
		t.Fatalf("expected truncation note, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tail (last") || !strings.Contains(stdout, "the very last line of the oversized stream") {
		t.Fatalf("expected retained tail excerpt, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--tee <path>` to preserve the full stream") {
		t.Fatalf("expected tee recovery suggestion, got:\n%s", stdout)
	}
}

func TestPipeTeeWritesRawStream(t *testing.T) {
	t.Parallel()

	input := pipeLogFixture(50)
	teePath := filepath.Join(t.TempDir(), "raw.log")

	code, stdout, _ := runPipeCommand(t, input, "--tee", teePath)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	raw, err := os.ReadFile(teePath)
	if err != nil {
		t.Fatalf("read tee file: %v", err)
	}
	if string(raw) != input {
		t.Fatalf("expected byte-exact tee copy (%d bytes vs %d input bytes)", len(raw), len(input))
	}
	if !strings.Contains(stdout, "full stream: "+teePath) {
		t.Fatalf("expected tee note, got:\n%s", stdout)
	}
}

func TestPipeEmptyStdin(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := runPipeCommand(t, "")
	if code != 0 {
		t.Fatalf("expected exit 0 for empty stdin, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "empty input (0 bytes on stdin)") {
		t.Fatalf("expected empty-input note, got:\n%s", stdout)
	}
}

func TestPipeWhitespaceOnlyInputNeverRendersOK(t *testing.T) {
	t.Parallel()

	code, stdout, _ := runPipeCommand(t, "   \n\n \t\n")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) == "ok" {
		t.Fatalf("pipe must not fabricate success framing, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no summarizable text") {
		t.Fatalf("expected honest placeholder, got:\n%s", stdout)
	}
}

func TestPipeTerminalStdinIsUsageError(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	rt := localcmd.Runtime{Stdout: &stdout, Stderr: &stderr}
	code := localcmd.RunPipe(rt, config.Default(), nil, strings.NewReader(""), true)
	if code != 2 {
		t.Fatalf("expected exit 2 for terminal stdin, got %d", code)
	}
	if !strings.Contains(stderr.String(), "pipe reads stdin from a pipe") {
		t.Fatalf("expected helpful terminal-stdin error, got: %s", stderr.String())
	}
}

func TestPipeUsageErrors(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"--hint", "bogus"},
		{"--hint"},
		{"--max-lines", "zero"},
		{"--max-lines", "0"},
		{"--tee"},
		{"unexpected-positional"},
	}
	for _, args := range cases {
		code, _, stderr := runPipeCommand(t, "input\n", args...)
		if code != 2 {
			t.Fatalf("expected exit 2 for args %v, got %d", args, code)
		}
		if !strings.Contains(stderr, "szr: pipe:") {
			t.Fatalf("expected pipe usage error for args %v, got: %s", args, stderr)
		}
	}
}

func TestPipeHelpDocumentsExitCodeHonesty(t *testing.T) {
	t.Parallel()

	code, stdout, _ := runPipeCommand(t, "", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
	if !strings.Contains(stdout, "cannot see the producing command's exit") {
		t.Fatalf("expected honesty note in help text, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--hint") || !strings.Contains(stdout, "--tee") {
		t.Fatalf("expected flag documentation, got:\n%s", stdout)
	}
}

func TestPipeMaxLinesOverridesBudget(t *testing.T) {
	t.Parallel()

	code, small, _ := runPipeCommand(t, pipeLogFixture(400), "--max-lines", "4")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	code, large, _ := runPipeCommand(t, pipeLogFixture(400), "--max-lines", "40")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if len(strings.Split(strings.TrimSpace(small), "\n")) > len(strings.Split(strings.TrimSpace(large), "\n")) {
		t.Fatalf("expected --max-lines 4 render to be no taller than --max-lines 40:\n--- small ---\n%s\n--- large ---\n%s", small, large)
	}
}

// The command must be registered in the app dispatch table and read the real
// process stdin.
func TestPipeCommandRegisteredInApp(t *testing.T) {
	app := testutil.NewTestApp(t)
	testutil.WithStdin(t, pipeLogFixture(120), func() {
		code, stdout, stderr := testutil.RunApp(t, app, "pipe")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
		}
		if !strings.Contains(stdout, "lines:") {
			t.Fatalf("expected log severity summary via app dispatch, got:\n%s", stdout)
		}
	})
}
