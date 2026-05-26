package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/declarative"
	fsfilter "github.com/devr-tools/szr/internal/filters/fs"
)

func TestLineHelpers(t *testing.T) {
	t.Parallel()

	if got := filters.CompactLines("a\nb\nc", 2); got != "a\nb\n... +1 more lines" {
		t.Fatalf("unexpected compact lines: %q", got)
	}
	if got := filters.CompactLines("a\nb", 5); got != "a\nb" {
		t.Fatalf("unexpected compact passthrough: %q", got)
	}
	if got := filters.CompactLines("\x1b[31ma\x1b[0m\n", 2); got != "a" {
		t.Fatalf("unexpected compact ansi stripping: %q", got)
	}
	if got := filters.InterestingErrorLines("progress\nwarning: retry\nerror: boom\n", 5); got != "warning: retry\nerror: boom" {
		t.Fatalf("unexpected interesting error lines: %q", got)
	}
	if kind, summary, requireRawCapture := filters.DeclarativeFullOutputRecovery(declarativeResultForTest(2, 0), "lines"); kind != filters.RecoveryKindFullOutput || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected declarative recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	deduped := filters.DedupeLines("a\na\nb\nc\nc\n", 2)
	if deduped != "a (x2)\nb\n... +1 more folded lines" {
		t.Fatalf("unexpected dedupe: %q", deduped)
	}

	folded := filters.FoldConsecutiveLines([]string{"warn", "warn", "error", "error", "error", "hint"})
	if got := strings.Join(folded, ","); got != "warn (x2),error (x3),hint" {
		t.Fatalf("unexpected folded lines: %q", got)
	}

	anchors := filters.SelectUniqueAnchoredLines([]string{
		"at src/app.ts:10:2",
		"at src/app.ts:10:2",
		"at src/other.ts:4:1",
	}, 3)
	if got := strings.Join(anchors, ","); got != "at src/app.ts:10:2,at src/other.ts:4:1" {
		t.Fatalf("unexpected anchored lines: %q", got)
	}
}

func declarativeResultForTest(omittedBefore, omittedAfter int) declarative.Result {
	return declarative.Result{OmittedBefore: omittedBefore, OmittedAfter: omittedAfter}
}

func TestFailureHelpers(t *testing.T) {
	t.Parallel()

	t.Run("basic summaries", testBasicFailureSummaries)
	t.Run("streaming reducer", testStreamingFailureReducer)
	t.Run("ranked stack summary", testRankedStackSummary)
	t.Run("php anchor summary", testPHPAnchorSummary)
	t.Run("prefiltered summary", testPrefilteredSummary)
	t.Run("go test json recovery", testGoTestJSONRecovery)
}

func testBasicFailureSummaries(t *testing.T) {
	t.Helper()

	if got := filters.SummarizeGenericFailure("", 3); got != "ok" {
		t.Fatalf("unexpected empty generic failure: %q", got)
	}
	if got := filters.SummarizeGenericFailure("info\nwarning: x\npanic: y\n", 1); got != "panic: y" {
		t.Fatalf("unexpected generic failure summary: %q", got)
	}
	if got := filters.SummarizeGenericFailure("line1\nline2\nline3\n", 2); got != "line1\nline2\n... +1 more lines" {
		t.Fatalf("unexpected generic fallback: %q", got)
	}

	fallbackReducer := filters.NewGenericFailureReducer(2, 0)
	fallbackReducer.ConsumeStdout([]byte("line1\nline2\nline3\n"))
	kind, summary, requireRawCapture := fallbackReducer.RecoveryInfo()
	assertRecoveryInfo(t, kind, summary, requireRawCapture, filters.RecoveryKindFullOutput, "omitted 1 additional lines", true)
}

func testStreamingFailureReducer(t *testing.T) {
	t.Helper()

	reducer := filters.NewGenericFailureReducer(2, 0)
	reducer.ConsumeStderr([]byte("\x1b[31mwarning"))
	reducer.ConsumeStdout([]byte(": x\x1b[0m\npanic: y\n"))
	if got := reducer.Result(); got != "panic: y\nwarning: x" {
		t.Fatalf("unexpected streaming generic failure: %q", got)
	}
	if reducer.FallbackUsed() {
		t.Fatal("did not expect streaming reducer to report fallback")
	}
	if reducer.BytesParsed() != len("\x1b[31mwarning")+len(": x\x1b[0m\npanic: y\n") {
		t.Fatalf("unexpected bytes parsed: %d", reducer.BytesParsed())
	}
}

func testRankedStackSummary(t *testing.T) {
	t.Helper()

	stack := filters.SummarizeGenericFailure(strings.Join([]string{
		"warning: retrying connection",
		"warning: retrying connection",
		"panic: nil pointer dereference",
		"at /tmp/app/main.go:42",
		"at /tmp/app/main.go:42",
		"help: rerun with --verbose",
	}, "\n"), 4)
	assertContainsAll(t, stack,
		"panic: nil pointer dereference",
		"/tmp/app/main.go:42",
		"help: rerun with --verbose",
		"warning: retrying connection (+1 similar warnings)",
	)
}

func testPHPAnchorSummary(t *testing.T) {
	t.Helper()

	phpAnchor := filters.SummarizeGenericFailure(strings.Join([]string{
		"Fatal error: Uncaught RuntimeException",
		"at /srv/app/src/Kernel.php:27",
		"help: rerun with APP_ENV=test",
	}, "\n"), 3)
	assertContainsAll(t, phpAnchor,
		"Fatal error: Uncaught RuntimeException",
		"Kernel.php:27",
		"help: rerun with APP_ENV=test",
	)
}

func testPrefilteredSummary(t *testing.T) {
	t.Helper()

	input := strings.Join([]string{
		"Downloading registry index",
		"Resolving: total 12, reused 0, downloaded 6",
		"added 487 packages in 8s",
		"error: build failed",
		"/Users/alex/Documents/GitHub/szr/internal/filters/failure.go:201:3 undefined: noiseGate",
		"/Users/alex/Documents/GitHub/szr/internal/filters/failure.go:201:3 undefined: noiseGate",
		"help: rerun with --verbose",
	}, "\n")
	prefiltered := filters.SummarizeGenericFailure(input, 5)
	assertContainsAll(t, prefiltered,
		"error: build failed",
		".../internal/filters/failure.go:201:3",
		"help: rerun with --verbose",
		"... omitted 2 progress lines, 1 install lines",
		"(+1 similar frames)",
	)

	prefilterReducer := filters.NewGenericFailureReducer(5, 0)
	prefilterReducer.ConsumeStdout([]byte(input))
	kind, summary, requireRawCapture := prefilterReducer.RecoveryInfo()
	assertRecoveryInfo(t, kind, summary, requireRawCapture, filters.RecoveryKindFullOutput, "omitted 2 progress lines, 1 install lines", true)
}

func testGoTestJSONRecovery(t *testing.T) {
	t.Helper()

	goTestJSON := strings.Join([]string{
		`{"Action":"pass","Package":"github.com/acme/pass"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestOne"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestTwo"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestThree"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestFour"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestFive"}`,
	}, "\n")
	kind, summary, requireRawCapture := filters.GoTestJSONRecoveryInfo(goTestJSON)
	assertRecoveryInfo(t, kind, summary, requireRawCapture, filters.RecoveryKindFullOutput, "omitted 1 additional test lines", true)
}

func assertContainsAll(t *testing.T, got string, want ...string) {
	t.Helper()

	for _, fragment := range want {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected %q in output:\n%s", fragment, got)
		}
	}
}

func assertRecoveryInfo(t *testing.T, kind, summary string, requireRawCapture bool, wantKind, wantSummary string, wantRequireRawCapture bool) {
	t.Helper()

	if kind != wantKind || summary != wantSummary || requireRawCapture != wantRequireRawCapture {
		t.Fatalf("unexpected recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestUtilityHelpers(t *testing.T) {
	t.Parallel()

	if got := filters.CollapseBlock("plain text"); got != "plain text" {
		t.Fatalf("unexpected collapse fallback: %q", got)
	}
	if got := filters.Clip("abcdef", 3); got != "abc..." {
		t.Fatalf("unexpected clip long: %q", got)
	}
	if got := filters.Clip("abc", 5); got != "abc" {
		t.Fatalf("unexpected clip short: %q", got)
	}
	unique := filters.UniqueStrings([]string{" one ", "", "one", "two"})
	if strings.Join(unique, ",") != "one,two" {
		t.Fatalf("unexpected unique strings: %#v", unique)
	}
	if got := filters.ScannerDedupe([]byte("x\nx\n")); got != "x (x2)" {
		t.Fatalf("unexpected scanner dedupe: %q", got)
	}
	if got := filters.StripANSI("\x1b[31mred\x1b[0m plain"); got != "red plain" {
		t.Fatalf("unexpected ansi strip: %q", got)
	}

	compact := filters.NewCompactLineReducer(2, 0)
	compact.ConsumeStdout([]byte("a\n\x1b[31mb"))
	compact.ConsumeStderr([]byte("2\x1b[0m\nc\n"))
	if got := compact.Result(); got != "a\nb2\n... +1 more lines" {
		t.Fatalf("unexpected streaming compact lines: %q", got)
	}
	if kind, summary, requireRawCapture := compact.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected compact recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	entropyCompact := filters.NewCompactLineReducer(2, 0)
	entropyCompact.ConsumeStdout([]byte("repeat\nrepeat\nrepeat\nunique\n"))
	if got := entropyCompact.Result(); got != "repeat (x3)\nunique" {
		t.Fatalf("unexpected entropy-aware compact lines: %q", got)
	}

	contracted := filters.NewGenericFailureReducerWithContract(3, 0, 1, 1, 1)
	contracted.ConsumeStdout([]byte("panic: boom\nsrc/app.go:12:3\nhelp: rerun with -v\n"))
	if got := contracted.Result(); !strings.Contains(got, "panic: boom") || !strings.Contains(got, "src/app.go:12:3") || !strings.Contains(got, "help: rerun with -v") {
		t.Fatalf("expected contract-preserved failure parts, got %q", got)
	}
}

func TestFilesystemSummaries(t *testing.T) {
	t.Parallel()

	tree := strings.Join([]string{
		"project",
		"|-- cmd",
		"|   |-- app",
		"|   `-- lib",
		"`-- docs",
		"2 directories, 2 files",
	}, "\n")
	got := fsfilter.SummarizeTreeOutput(tree, 6)
	for _, want := range []string{"project", "cmd (2) app, lib", "docs", "2 directories, 2 files"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected tree summary %q in %q", want, got)
		}
	}

	compact := fsfilter.SummarizeTreeOutput(tree, 2)
	lines := strings.Split(compact, "\n")
	if len(lines) != 2 || lines[0] != "project" || lines[1] != "2 directories, 2 files" {
		t.Fatalf("expected compact tree summary to preserve root and footer, got %q", compact)
	}

	if kind, summary, requireRawCapture := fsfilter.TreeOutputRecoveryInfo(tree, 2); kind != filters.RecoveryKindFullOutput || summary != "omitted tree entries" || !requireRawCapture {
		t.Fatalf("unexpected tree recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	listingInput := "README.md\nMakefile\nsrc/\ndocs/\ninternal/\ntest/\npkg/\n"
	if kind, summary, requireRawCapture := fsfilter.DirectoryListingRecoveryInfo(listingInput, 4); kind != filters.RecoveryKindFullOutput || summary != "omitted 7 directory entries" || !requireRawCapture {
		t.Fatalf("unexpected directory listing recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	reducer := filters.NewBufferedTextReducerWithRecovery(true, false, func(input string) string {
		return fsfilter.SummarizeDirectoryListing(input, 4)
	}, func(input string) (string, string, bool) {
		return fsfilter.DirectoryListingRecoveryInfo(input, 4)
	})
	reducer.ConsumeStdout([]byte(listingInput))
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 7 directory entries" || !requireRawCapture {
		t.Fatalf("unexpected buffered recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
