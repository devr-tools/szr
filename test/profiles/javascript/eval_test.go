package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestNodeEvalProfileMetadata(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "node-eval")

	if profile.StreamPreference != engine.StreamStderrFirst {
		t.Fatalf("expected stderr-first stream preference, got %q", profile.StreamPreference)
	}
	if profile.Budget.MinFailures < 1 || profile.Budget.MinAnchors < 1 {
		t.Fatalf("expected failure/anchor floors, got %#v", profile.Budget)
	}

	if !profile.Match(engine.Classify(engine.Invocation{Command: []string{"node", "-e", "throw 1"}})) {
		t.Fatal("node-eval should match node -e")
	}
	if !profile.Match(engine.Classify(engine.Invocation{Display: []string{"node", "--print", "1+1"}})) {
		t.Fatal("node-eval should match node --print")
	}
	if !profile.Match(engine.Classify(engine.Invocation{Command: []string{"zsh", "-lc", "source /dev/null && node -e 'throw 1'"}})) {
		t.Fatal("node-eval should match node -e behind a shell wrapper")
	}
	if profile.Match(engine.Classify(engine.Invocation{Command: []string{"node", "server.mjs"}})) {
		t.Fatal("node-eval should not match node script runs")
	}
}

func TestNodeEvalProfileRendersFailureAnchors(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "node-eval")

	stderr := strings.Join([]string{
		"[eval]:1",
		`throw new Error("boom")`,
		"^",
		"",
		"Error: boom",
		"    at [eval]:1:7",
		"    at runScriptInThisContext (node:internal/vm:209:10)",
		"    at evalTypeScript (node:internal/process/execution:104:62)",
		"Node.js v22.14.0",
	}, "\n")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stderr: stderr, ExitCode: 1})
	if !strings.Contains(rendered, "boom") {
		t.Fatalf("expected thrown error message to survive, got %q", rendered)
	}
	if !strings.Contains(rendered, "[eval]:1:7") {
		t.Fatalf("expected eval stack anchor to survive, got %q", rendered)
	}

	reducer := profile.StreamRender(engine.Invocation{}, profile.Budget)
	reducer.ConsumeStderr([]byte(stderr))
	streamed := reducer.Result()
	if !strings.Contains(streamed, "boom") || !strings.Contains(streamed, "[eval]:1:7") {
		t.Fatalf("expected streaming reducer to keep error and eval anchor, got %q", streamed)
	}
}

func TestNodeEvalProfileRendersSuccessCompactly(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "node-eval")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stdout: "{\"ok\":true}\ndone\n"})
	if !strings.Contains(rendered, `{"ok":true}`) || !strings.Contains(rendered, "done") {
		t.Fatalf("expected compact success lines, got %q", rendered)
	}
}
