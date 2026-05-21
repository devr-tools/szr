package cpp_test

import (
	"strings"
	"testing"

	cppfilter "szr/internal/filters/cpp"
)

func TestSummarizeCTest(t *testing.T) {
	input := strings.Join([]string{
		"Test project /tmp/build",
		"1/2 Test #1: api_smoke ....................***Failed    0.02 sec",
		"src/api_test.cpp:19: Assertion failed",
		"The following tests FAILED:",
		"1 - api_smoke (Failed)",
	}, "\n")

	got := cppfilter.SummarizeCTest(input, 5)
	for _, want := range []string{
		"api_smoke",
		"src/api_test.cpp:19: Assertion failed",
		"The following tests FAILED:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in ctest summary:\n%s", want, got)
		}
	}
}

func TestSummarizeClangTooling(t *testing.T) {
	input := strings.Join([]string{
		"Running clang-tidy",
		"src/main.cpp:10:5: warning: use nullptr [modernize-use-nullptr]",
		"include/app.h:7:2: error: expected ';' after class",
		"bear: compiled 12 translation units",
	}, "\n")

	got := cppfilter.SummarizeClangTooling(input, 5)
	for _, want := range []string{
		"src/main.cpp:10:5: warning: use nullptr [modernize-use-nullptr]",
		"include/app.h:7:2: error: expected ';' after class",
		"bear: compiled 12 translation units",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in clang tooling summary:\n%s", want, got)
		}
	}
}
