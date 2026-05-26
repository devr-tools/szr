package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPytestProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "pytest")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"collected 2 items",
			"",
			">       assert add(1, 2) == 2",
			"E       assert 3 == 2",
			"tests/test_math.py:12: AssertionError",
			"=========================== short test summary info ============================",
			"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
			"========================= 1 failed, 1 passed in 0.10s =========================",
		}, "\n"),
	})
	for _, want := range []string{
		"collected 2 items",
		"1 failed, 1 passed in 0.10s",
		"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
		"assert add(1, 2) == 2",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered pytest output:\n%s", want, rendered)
		}
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil || profile.Budget.MaxLines < 6 {
		t.Fatalf("unexpected pytest stream metadata: %#v", profile)
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("collected 1 item\n"))
	streamed.ConsumeStdout([]byte("E       fixture 'client' not found\n"))
	streamed.ConsumeStdout([]byte("ERROR tests/test_api.py::test_client - fixture 'client' not found\n"))
	streamed.ConsumeStderr([]byte("=============================== 1 error in 0.03s ===============================\n"))
	streamRendered := streamed.Result()
	for _, want := range []string{
		"collected 1 item",
		"ERROR tests/test_api.py::test_client - fixture 'client' not found",
		"1 error in 0.03s",
	} {
		if !strings.Contains(streamRendered, want) {
			t.Fatalf("expected %q in streamed pytest output:\n%s", want, streamRendered)
		}
	}
}

func TestPythonToolingProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "python-tooling")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
			"src/app.py:18:5: F401 `os` imported but unused",
			"Found 2 errors in 1 file (checked 4 source files)",
		}, "\n"),
	})
	for _, want := range []string{
		"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
		"src/app.py:18:5: F401 `os` imported but unused",
		"Found 2 errors in 1 file",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in python-tooling render output:\n%s", want, rendered)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected python-tooling stream metadata: %#v", profile)
	}
}

func TestPythonProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	pytest := testutil.FindProfile(t, list, "pytest")
	pytestStream := pytest.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 4})
	pytestStream.ConsumeStdout([]byte(strings.Join([]string{
		"collected 3 items",
		"FAILED tests/test_math.py::test_add - AssertionError: assert 3 == 2",
		"assert add(1, 2) == 2",
		"assert 3 == 2",
		"tests/test_math.py:12: AssertionError",
		"available fixtures: cache, capfd, caplog",
	}, "\n")))
	pytestRecovery, ok := pytestStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable pytest reducer, got %T", pytestStream)
	}
	if kind, summary, requireRawCapture := pytestRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected pytest recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	tooling := testutil.FindProfile(t, list, "python-tooling")
	toolingStream := tooling.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	toolingStream.ConsumeStdout([]byte(strings.Join([]string{
		"src/app.py:12: error: Name \"missing\" is not defined  [name-defined]",
		"src/app.py:18:5: F401 `os` imported but unused",
		"ERROR: Could not find a version that satisfies the requirement missing-pkg",
		"Found 2 errors in 1 file (checked 4 source files)",
	}, "\n")))
	toolingRecovery, ok := toolingStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable python tooling reducer, got %T", toolingStream)
	}
	if kind, summary, requireRawCapture := toolingRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected python tooling recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
