package profiles_test

import (
	"strings"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
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
