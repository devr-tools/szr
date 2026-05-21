package build_test

import (
	"strings"
	"testing"

	buildfilter "szr/internal/filters/build"
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
