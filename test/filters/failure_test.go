package filters_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/declarative"
)

func TestFailureNoSignalHeadFoldsTimestampedLogs(t *testing.T) {
	t.Parallel()

	messages := []string{
		"INFO starting worker",
		"INFO fetching batch",
		"INFO processed batch",
		"INFO flushing cache",
		"INFO heartbeat ok",
	}
	lines := make([]string, 0, 500)
	for group, message := range messages {
		for i := 0; i < 100; i++ {
			lines = append(lines, fmt.Sprintf("2026-05-20T21:%02d:%02dZ %s", group, i%60, message))
		}
	}
	got := filters.SummarizeGenericFailure(strings.Join(lines, "\n"), 3)
	assertContainsAll(t, got,
		"2026-05-20T21:00:00Z INFO starting worker (x100)",
		"2026-05-20T21:01:00Z INFO fetching batch (x100)",
		"2026-05-20T21:02:00Z INFO processed batch (x100)",
		"... +2 more lines",
	)
	if strings.Count(got, "\n") > 3 {
		t.Fatalf("expected folded head within budget, got:\n%s", got)
	}
}

func TestSimilarLineKeyTimestampFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		first string
		next  string
	}{
		{"iso8601", "2026-05-20T21:00:00Z downloading cache", "2026-05-20T21:00:01Z downloading cache"},
		{"syslog double space", "Jan  2 15:04:05 host app started", "Jan  2 15:04:06 host app started"},
		{"syslog padded day", "Jan 02 15:04:05 host app started", "Jan 12 16:20:30 host app started"},
		{"bracketed iso", "[2024-01-02T10:00:00Z] retry queued", "[2024-01-02T10:00:05Z] retry queued"},
		{"bracketed clock", "[15:04:05] retry queued", "[15:04:09] retry queued"},
		{"bare clock millis", "15:04:05.123 retry queued", "15:04:06.456 retry queued"},
		{"epoch seconds", "1717171717 retry queued", "1717171718 retry queued"},
		{"epoch millis", "1717171717123 retry queued", "1717171717456 retry queued"},
		{"level then timestamp", "INFO 2024-01-02T10:00:00Z connection reset", "INFO 2024-01-02T10:00:09Z connection reset"},
		{"bracketed level then clock", "[WARN] 15:04:05 connection reset", "[WARN] 15:04:06 connection reset"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if declarative.SimilarLineKey(tc.first) != declarative.SimilarLineKey(tc.next) {
				t.Fatalf("expected %q and %q to share a similarity key (%q vs %q)",
					tc.first, tc.next, declarative.SimilarLineKey(tc.first), declarative.SimilarLineKey(tc.next))
			}
			folded := declarative.FoldConsecutiveSimilar([]string{tc.first, tc.next})
			if len(folded) != 1 || !strings.HasSuffix(folded[0], "(x2)") {
				t.Fatalf("expected fold to single (x2) line, got %#v", folded)
			}
		})
	}
}

func TestSimilarLineKeyKeepsLogLevelDistinct(t *testing.T) {
	t.Parallel()

	info := "INFO 2024-01-02T10:00:00Z connection reset"
	errLine := "ERROR 2024-01-02T10:00:01Z connection reset"
	if declarative.SimilarLineKey(info) == declarative.SimilarLineKey(errLine) {
		t.Fatalf("ERROR line must not share key with INFO line: %q", declarative.SimilarLineKey(info))
	}
	folded := declarative.FoldConsecutiveSimilar([]string{info, errLine})
	if len(folded) != 2 {
		t.Fatalf("expected ERROR line to stay separate from INFO run, got %#v", folded)
	}
	// Only the timestamp is stripped; the level token stays in the key.
	if got := declarative.SimilarLineKey(info); got != "INFO connection reset" {
		t.Fatalf("unexpected similarity key: %q", got)
	}
	// A line that is only a timestamp keeps its content.
	if got := declarative.SimilarLineKey("2024-01-02T10:00:00Z"); got == "" {
		t.Fatal("timestamp-only line must not normalize to empty key")
	}
}

func TestDiagnosticAnchorExtendedExtensions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		line string
		want string
	}{
		{"from /srv/app/lib/worker.rb:15:in 'perform'", "/srv/app/lib/worker.rb:15:in"},
		{"at com.example.MainKt.main(Main.kt:12)", "Main.kt:12"},
		{"at MyApp/Sources/App/main.swift:42", "MyApp/Sources/App/main.swift:42"},
		{"warning: unused variable at lib/tasks/report.kts:3", "lib/tasks/report.kts:3"},
		{"** (RuntimeError) lib/my_app/worker.ex:9", "lib/my_app/worker.ex:9"},
		{"deploy.sh:12: command not found", "deploy.sh:12:"},
	}
	for _, tc := range cases {
		if got := filters.DiagnosticAnchor(tc.line); got != tc.want {
			t.Fatalf("DiagnosticAnchor(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}

	rubySummary := filters.SummarizeGenericFailure(strings.Join([]string{
		"RuntimeError: boom",
		"from /srv/app/very/deep/nested/lib/worker.rb:15:in 'perform'",
	}, "\n"), 4)
	assertContainsAll(t, rubySummary, "RuntimeError: boom", ".../nested/lib/worker.rb:15:in")
}

func TestFailureLongPathShortening(t *testing.T) {
	t.Parallel()

	got := filters.SummarizeGenericFailure(strings.Join([]string{
		"Error: worker crashed",
		"at run (/Users/alex/dev/shop/node_modules/.pnpm/esbuild@0.19.2/bin/esbuild:44:12)",
		"error: cannot open /private/var/folders/ab/xyz1234567890/T/build-artifacts-9876/output.bin",
	}, "\n"), 6)
	assertContainsAll(t, got,
		"Error: worker crashed",
		"at run (.../esbuild@0.19.2/bin/esbuild:44:12)",
		"error: cannot open .../T/build-artifacts-9876/output.bin",
	)
	if strings.Contains(got, "/Users/alex/dev/shop/node_modules") {
		t.Fatalf("expected long node_modules path to be shortened:\n%s", got)
	}
}

func TestFailurePassLinesWithFailureSignalRetained(t *testing.T) {
	t.Parallel()

	got := filters.SummarizeGenericFailure("3 tests passed, 2 failed\n", 4)
	if got == "ok" || !strings.Contains(got, "3 tests passed, 2 failed") {
		t.Fatalf("mixed pass/fail summary line must be retained, got: %q", got)
	}

	withNoise := filters.SummarizeGenericFailure(strings.Join([]string{
		"test button renders passed",
		"test button mounts passed",
		"error: boom",
	}, "\n"), 4)
	assertContainsAll(t, withNoise, "error: boom", "... omitted 2 pass lines")
}
