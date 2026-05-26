package build_test

import (
	"strings"
	"testing"

	buildfilter "github.com/devr-tools/szr/internal/filters/build"
)

func TestSummarizeBuildSystem(t *testing.T) {
	input := strings.Join([]string{
		"[3/10] Building CXX object src/app.cpp.o",
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"ninja: build stopped: subcommand failed.",
	}, "\n")

	got := buildfilter.SummarizeBuildSystem(input, 4)
	for _, want := range []string{
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"ninja: build stopped: subcommand failed.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in build summary:\n%s", want, got)
		}
	}
}

func TestBuildSystemRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"src/app.cpp:14:2: note: candidate function not viable",
		"src/lib.cpp:20:7: error: no member named 'x' in 'Thing'",
		"ninja: build stopped: subcommand failed.",
	}, "\n")

	kind, summary, requireRawCapture := buildfilter.BuildSystemRecoveryInfo(input, 3)
	if kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected build recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
