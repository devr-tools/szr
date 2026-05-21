package rust_test

import (
	"strings"
	"testing"

	rustfilter "szr/internal/filters/rust"
)

func TestSummarizeCargoTest(t *testing.T) {
	input := strings.Join([]string{
		"running 3 tests",
		"test tests::math::adds ... ok",
		"test tests::math::subtracts ... FAILED",
		"",
		"failures:",
		"",
		"---- tests::math::subtracts stdout ----",
		"thread 'tests::math::subtracts' panicked at src/lib.rs:42:5:",
		"assertion `left == right` failed",
		"",
		"failures:",
		"tests::math::subtracts",
		"",
		"test result: FAILED. 2 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out",
		"error: test failed, to rerun pass `--lib`",
	}, "\n")

	got := rustfilter.SummarizeCargoTest(input, 7)
	for _, want := range []string{
		"running 3 tests",
		"test result: FAILED. 2 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out",
		"test tests::math::subtracts ... FAILED",
		"thread 'tests::math::subtracts' panicked at src/lib.rs:42:5:",
		"assertion `left == right` failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cargo test summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCargoBuild(t *testing.T) {
	input := strings.Join([]string{
		"Compiling app v0.1.0 (/tmp/app)",
		"error[E0432]: unresolved import `missing::Thing`",
		"--> src/lib.rs:4:5",
		"help: consider importing this module instead",
		"warning: unused import: `std::fmt`",
		"error: could not compile `app` due to 1 previous error; 1 warning emitted",
	}, "\n")

	got := rustfilter.SummarizeCargoBuild(input, 6)
	for _, want := range []string{
		"error[E0432]: unresolved import `missing::Thing`",
		"--> src/lib.rs:4:5",
		"help: consider importing this module instead",
		"warning: unused import: `std::fmt`",
		"error: could not compile `app` due to 1 previous error; 1 warning emitted",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cargo build summary:\n%s", want, got)
		}
	}
}
