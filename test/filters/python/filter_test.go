package python_test

import (
	"strings"
	"testing"

	pyfilter "github.com/devr-tools/szr/internal/filters/python"
)

func TestSummarizePytestFailures(t *testing.T) {
	input := strings.Join([]string{
		"============================= test session starts ==============================",
		"collected 3 items",
		"",
		"tests/test_math.py .F.                                                    [100%]",
		"",
		"=================================== FAILURES ===================================",
		"___________________________________ test_add ___________________________________",
		"",
		"    def test_add():",
		">       assert add(1, 2) == 2",
		"E       assert 3 == 2",
		"",
		"tests/test_math.py:12: AssertionError",
		"=========================== short test summary info ============================",
		"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
		"========================= 1 failed, 2 passed in 0.12s =========================",
	}, "\n")

	got := pyfilter.SummarizePytest(input, 6)
	for _, want := range []string{
		"collected 3 items",
		"1 failed, 2 passed in 0.12s",
		"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
		"assert add(1, 2) == 2",
		"assert 3 == 2",
		"tests/test_math.py:12: AssertionError",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in pytest summary:\n%s", want, got)
		}
	}
}

func TestSummarizePytestFixtureError(t *testing.T) {
	input := strings.Join([]string{
		"collected 1 item",
		"",
		"==================================== ERRORS ====================================",
		"_______________________ ERROR at setup of test_client ________________________",
		"",
		"E       fixture 'client' not found",
		">       available fixtures: cache, capfd, caplog",
		"",
		"=========================== short test summary info ============================",
		"ERROR tests/test_api.py::test_client - fixture 'client' not found",
		"=============================== 1 error in 0.03s ===============================",
	}, "\n")

	got := pyfilter.SummarizePytest(input, 8)
	for _, want := range []string{
		"collected 1 item",
		"1 error in 0.03s",
		"ERROR tests/test_api.py::test_client - fixture 'client' not found",
		"fixture 'client' not found",
		"available fixtures: cache, capfd, caplog",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in pytest fixture summary:\n%s", want, got)
		}
	}
}

func TestSummarizePytestKeepsEveryShortSummaryFailure(t *testing.T) {
	input := strings.Join([]string{
		"collected 52 items",
		"=================================== FAILURES ===================================",
		"__________________________ test_rate_limit_headers ____________________________",
		">       assert resp.status_code == 429",
		"E       assert 200 == 429",
		"tests/test_api.py:87: AssertionError",
		"________________________ test_normalize_rejects_empty _________________________",
		">       assert normalize_user(\"   \") == \"anonymous\"",
		"E       AssertionError: assert '' == 'anonymous'",
		"tests/test_users.py:31: AssertionError",
		"=========================== short test summary info ============================",
		"FAILED tests/test_api.py::test_rate_limit_headers - assert 200 == 429",
		"FAILED tests/test_users.py::test_normalize_rejects_empty - AssertionError: as...",
		"FAILED tests/test_users.py::test_normalize_variants[space-2] - AssertionError",
		"========================= 3 failed, 49 passed in 4.83s =========================",
	}, "\n")

	got := pyfilter.SummarizePytest(input, 12)
	for _, want := range []string{
		"FAILED tests/test_api.py::test_rate_limit_headers - assert 200 == 429",
		"FAILED tests/test_users.py::test_normalize_rejects_empty - AssertionError: as...",
		"FAILED tests/test_users.py::test_normalize_variants[space-2] - AssertionError",
		"3 failed, 49 passed in 4.83s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in pytest summary:\n%s", want, got)
		}
	}
}

func TestSummarizePytestRecoversIdsFromSectionBanners(t *testing.T) {
	input := strings.Join([]string{
		"collected 12 items",
		"=================================== FAILURES ===================================",
		"________________________ test_normalize_rejects_empty _________________________",
		">       assert normalize_user(\"   \") == \"anonymous\"",
		"E       AssertionError: assert '' == 'anonymous'",
		"tests/test_users.py:31: AssertionError",
		"========================= 1 failed, 11 passed in 0.61s ========================",
	}, "\n")

	got := pyfilter.SummarizePytest(input, 8)
	if !strings.Contains(got, "test_normalize_rejects_empty") {
		t.Fatalf("expected section-banner test id retention:\n%s", got)
	}
}

func TestSummarizePytestPass(t *testing.T) {
	input := "collected 2 items\n\n..\n============================== 2 passed in 0.04s ==============================\n"
	if got := pyfilter.SummarizePytest(input, 4); got != "collected 2 items\n2 passed in 0.04s" {
		t.Fatalf("unexpected pytest pass summary: %q", got)
	}
}

func TestSummarizePythonTooling(t *testing.T) {
	input := strings.Join([]string{
		"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
		"src/app.py:18:5: F401 `os` imported but unused",
		"ERROR: Could not find a version that satisfies the requirement missing-pkg",
		"Found 2 errors in 1 file (checked 4 source files)",
	}, "\n")

	got := pyfilter.SummarizePythonTooling(input, 5)
	for _, want := range []string{
		"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
		"src/app.py:18:5: F401 `os` imported but unused",
		"Could not find a version that satisfies the requirement missing-pkg",
		"Found 2 errors in 1 file",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in python tooling summary:\n%s", want, got)
		}
	}
}

func TestSummarizePytestFoldsRepeatedFrames(t *testing.T) {
	input := strings.Join([]string{
		"collected 1 item",
		"FAILED tests/test_api.py::test_call - RuntimeError: boom",
		"RuntimeError: boom",
		"tests/test_api.py:14: RuntimeError",
		"tests/test_api.py:14: RuntimeError",
		">       available fixtures: cache, capfd",
	}, "\n")

	got := pyfilter.SummarizePytest(input, 6)
	if strings.Count(got, "tests/test_api.py:14: RuntimeError") != 1 {
		t.Fatalf("expected folded pytest stack anchors, got %q", got)
	}
	if !strings.Contains(got, "available fixtures: cache, capfd") {
		t.Fatalf("expected fixture hint retention, got %q", got)
	}
}

func TestPythonRecoveryInfo(t *testing.T) {
	pytestInput := strings.Join([]string{
		"collected 3 items",
		"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
		"assert add(1, 2) == 2",
		"assert 3 == 2",
		"tests/test_math.py:12: AssertionError",
		"available fixtures: cache, capfd, caplog",
	}, "\n")
	if kind, summary, requireRawCapture := pyfilter.PytestRecoveryInfo(pytestInput, 4); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected pytest recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	toolingInput := strings.Join([]string{
		"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
		"src/app.py:18:5: F401 `os` imported but unused",
		"ERROR: Could not find a version that satisfies the requirement missing-pkg",
		"Found 2 errors in 1 file (checked 4 source files)",
	}, "\n")
	if kind, summary, requireRawCapture := pyfilter.PythonToolingRecoveryInfo(toolingInput, 3); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected python tooling recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
