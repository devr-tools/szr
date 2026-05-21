package python_test

import (
	"strings"
	"testing"

	pyfilter "szr/internal/filters/python"
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

func TestSummarizePytestPass(t *testing.T) {
	input := "collected 2 items\n\n..\n============================== 2 passed in 0.04s ==============================\n"
	if got := pyfilter.SummarizePytest(input, 4); got != "collected 2 items\n2 passed in 0.04s" {
		t.Fatalf("unexpected pytest pass summary: %q", got)
	}
}
