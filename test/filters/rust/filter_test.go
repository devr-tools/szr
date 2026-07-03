package rust_test

import (
	"strings"
	"testing"

	rustfilter "github.com/devr-tools/szr/internal/filters/rust"
	"github.com/devr-tools/szr/internal/history"
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

func TestSummarizeCargoBuildFoldsRepeatedHints(t *testing.T) {
	input := strings.Join([]string{
		"error[E0432]: unresolved import `missing::Thing`",
		"--> src/lib.rs:4:5",
		"help: consider importing this module instead",
		"help: consider importing this module instead",
		"note: `#[warn(unused_imports)]` on by default",
	}, "\n")

	got := rustfilter.SummarizeCargoBuild(input, 6)
	if !strings.Contains(got, "help: consider importing this module instead (x2)") {
		t.Fatalf("expected folded cargo hints, got %q", got)
	}
	if strings.Count(got, "--> src/lib.rs:4:5") != 1 {
		t.Fatalf("expected unique cargo stack anchor, got %q", got)
	}
}

func TestSummarizeCargoBuildKeepsEveryErrorCode(t *testing.T) {
	input := strings.Join([]string{
		"   Compiling autocfg v1.4.0",
		"   Compiling libc v0.2.169",
		"   Compiling billing-core v0.3.2 (/workspace/billing/core)",
		"error[E0308]: mismatched types",
		"   --> src/parser.rs:142:22",
		"    |",
		"142 |     let total: u64 = entries.iter().map(|e| e.amount).sum::<i64>();",
		"help: you can convert an `i64` to a `u64` and panic if the converted value doesn't fit",
		"error[E0599]: no method named `finalise` found for struct `InvoiceBuilder` in the current scope",
		"   --> src/invoice.rs:214:30",
		"help: there is a method `finalize` with a similar name",
		"error[E0382]: borrow of moved value: `line_items`",
		"   --> src/invoice.rs:221:16",
		"Some errors have detailed explanations: E0308, E0382, E0599.",
		"error: could not compile `billing-core` (lib) due to 3 previous errors",
	}, "\n")

	got := rustfilter.SummarizeCargoBuild(input, 12)
	for _, want := range []string{
		"error[E0308]: mismatched types",
		"error[E0599]: no method named `finalise` found for struct `InvoiceBuilder`",
		"error[E0382]: borrow of moved value: `line_items`",
		"--> src/parser.rs:142:22",
		"--> src/invoice.rs:214:30",
		"--> src/invoice.rs:221:16",
		"error: could not compile `billing-core` (lib) due to 3 previous errors",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cargo build summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Compiling autocfg") {
		t.Fatalf("expected compile progress noise to be dropped:\n%s", got)
	}
}

func TestSummarizeCargoBuildHeadersSurviveTightBudget(t *testing.T) {
	input := strings.Join([]string{
		"error[E0599]: no method named `finalise` found for struct `InvoiceBuilder` in the current scope",
		"   --> src/invoice.rs:214:30",
		"help: there is a method `finalize` with a similar name",
		"error[E0382]: borrow of moved value: `line_items`",
		"   --> src/invoice.rs:221:16",
		"error[E0308]: mismatched types",
		"   --> src/parser.rs:142:22",
		"error: could not compile `billing-core` (lib) due to 3 previous errors",
	}, "\n")

	got := rustfilter.SummarizeCargoBuild(input, 4)
	for _, want := range []string{"error[E0599]", "error[E0382]", "error[E0308]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive the tight budget:\n%s", want, got)
		}
	}
}

func TestSummarizeCargoClippyKeepsLintNames(t *testing.T) {
	input := strings.Join([]string{
		"    Checking billing-core v0.3.2 (/workspace/billing/core)",
		"warning: redundant clone",
		"  --> src/lib.rs:88:27",
		"   |",
		"88 |     emit(event.payload.clone());",
		"   |                           ^^^^^^^^ help: remove this",
		"   |",
		"   = help: for further information visit https://rust-lang.github.io/rust-clippy/master/index.html#redundant_clone",
		"   = note: `#[warn(clippy::redundant_clone)]` on by default",
		"warning: this `if` statement can be collapsed",
		"   --> src/reconcile.rs:41:5",
		"    = help: for further information visit https://rust-lang.github.io/rust-clippy/master/index.html#collapsible_if",
		"warning: `billing-core` (lib) generated 2 warnings (run `cargo clippy --fix --lib -p billing-core` to apply 1 suggestion)",
		"    Finished `dev` profile [unoptimized + debuginfo] target(s) in 1.18s",
	}, "\n")

	got := rustfilter.SummarizeCargoBuild(input, 12)
	for _, want := range []string{
		"warning: redundant clone [clippy::redundant_clone]",
		"warning: this `if` statement can be collapsed [clippy::collapsible_if]",
		"--> src/lib.rs:88:27",
		"88 |     emit(event.payload.clone());",
		"warning: `billing-core` (lib) generated 2 warnings",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in clippy summary:\n%s", want, got)
		}
	}
}

// TestSummarizeCargoClippyUnderContractKeepsEveryLintSlug pins the clippy
// needles under the armed compression contract: every warning header (with
// its lint slug annotation) must reach the display verbatim, including
// second-and-later warning blocks and the flagged identifier, while the
// whole render fits the contract allowance.
func TestSummarizeCargoClippyUnderContractKeepsEveryLintSlug(t *testing.T) {
	input := strings.Join([]string{
		"    Checking ledger-core v0.7.1 (/workspace/ledger/core)",
		"warning: redundant clone",
		"  --> src/lib.rs:88:27",
		"   |",
		"88 |     emit(event.payload.clone());",
		"   |                           ^^^^^^^^ help: remove this",
		"   |",
		"   = help: for further information visit https://rust-lang.github.io/rust-clippy/master/index.html#redundant_clone",
		"   = note: `#[warn(clippy::redundant_clone)]` on by default",
		"warning: this `if` statement can be collapsed",
		"   --> src/reconcile.rs:41:5",
		"41  | /     if enabled {",
		"42  | |         if entry.balanced() {",
		"43  | |             apply(entry);",
		"44  | |         }",
		"45  | |     }",
		"    | |_____^",
		"    = help: for further information visit https://rust-lang.github.io/rust-clippy/master/index.html#collapsible_if",
		"warning: unused variable: `cursor`",
		"   --> src/reconcile.rs:77:9",
		"    |",
		"77  |     let cursor = table.scan();",
		"    |         ^^^^^^ help: if this is intentional, prefix it with an underscore: `_cursor`",
		"warning: `ledger-core` (lib) generated 3 warnings (run `cargo clippy --fix --lib -p ledger-core` to apply 1 suggestion)",
		"    Finished `dev` profile [unoptimized + debuginfo] target(s) in 1.42s",
	}, "\n")

	got := rustfilter.SummarizeCargoBuildUnderContract(input, 10, true)
	for _, needle := range []string{"redundant_clone", "collapsible_if", "cursor"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected lint needle %q in self-capped clippy summary:\n%s", needle, got)
		}
	}
	allowed := (history.EstimateTokens(input) + 4) / 5
	if allowed < 48 {
		allowed = 48
	}
	if got := history.EstimateTokens(got); got > allowed {
		t.Fatalf("expected self-capped clippy summary within the contract allowance (%d), got %d tokens", allowed, got)
	}
}

func TestRustRecoveryInfo(t *testing.T) {
	testInput := strings.Join([]string{
		"test tests::math::subtracts ... FAILED",
		"thread 'tests::math::subtracts' panicked at src/lib.rs:42:5:",
		"assertion `left == right` failed",
		"test result: FAILED. 2 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out",
		"error: test failed, to rerun pass `--lib`",
	}, "\n")
	if kind, summary, requireRawCapture := rustfilter.CargoTestRecoveryInfo(testInput, 3); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected cargo test recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	buildInput := strings.Join([]string{
		"error[E0432]: unresolved import `missing::Thing`",
		"--> src/lib.rs:4:5",
		"help: consider importing this module instead",
		"warning: unused import: `std::fmt`",
		"error: could not compile `app` due to 1 previous error; 1 warning emitted",
	}, "\n")
	if kind, summary, requireRawCapture := rustfilter.CargoBuildRecoveryInfo(buildInput, 3); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected cargo build recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
