package profiles_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestJSProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "js-package-test")

	report := strings.Join([]string{
		"> app@test",
		"> vitest run --reporter=json",
		`{"numPassedTestSuites":1,"numFailedTestSuites":1,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3"]}]}]}`,
	}, "\n")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stdout: report})
	for _, want := range []string{"suites: pass=1 fail=1", "src/math.test.ts", "math subtracts", "Expected: 2", "Received: 3"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered output:\n%s", want, rendered)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil || profile.Budget.MaxLines < 6 {
		t.Fatalf("unexpected js-package-test stream metadata: %#v", profile)
	}
	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("> app@test\n"))
	streamed.ConsumeStdout([]byte("> vitest run --reporter=json\n"))
	streamed.ConsumeStdout([]byte(`{"numPassedTestSuites":1,"numFailedTestSuites":1,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3"]}]}]}`))
	streamRendered := streamed.Result()
	for _, want := range []string{"suites: pass=1 fail=1", "src/math.test.ts", "Expected: 2"} {
		if !strings.Contains(streamRendered, want) {
			t.Fatalf("expected %q in streamed output:\n%s", want, streamRendered)
		}
	}
}

func TestJSPackageTestProfileCoverage(t *testing.T) {
	list := profiles.Builtins(4)
	pm := testutil.FindProfile(t, list, "js-package-test")
	if pm.Match(engine.Invocation{Display: []string{"npm"}}) {
		t.Fatal("did not expect short npm args to match")
	}
	if !pm.Match(engine.Invocation{Display: []string{"npm", "run", "test"}}) {
		t.Fatal("expected npm run test to match")
	}
	if !pm.Match(engine.Invocation{Display: []string{"yarn", "test"}}) {
		t.Fatal("expected yarn test to match")
	}
	if pm.Match(engine.Invocation{Display: []string{"pnpm", "lint"}}) {
		t.Fatal("did not expect non-test package manager command to match")
	}
	if got := pm.Prepare(engine.Invocation{Command: []string{"bun", "test"}}); strings.Join(got, ",") != "bun,test" {
		t.Fatalf("expected unknown package manager passthrough, got %#v", got)
	}

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"jest"}}`)
	if got := pm.Prepare(engine.Invocation{Cwd: root}); got != nil {
		t.Fatalf("expected nil prepare for empty package manager command, got %#v", got)
	}
}

func TestJSStructuredRendererCoverage(t *testing.T) {
	list := profiles.Builtins(4)
	vitest := testutil.FindProfile(t, list, "vitest-json")
	if rendered := vitest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL src/a.test.ts\nExpected: 1"}); !strings.Contains(rendered, "FAIL src/a.test.ts") {
		t.Fatalf("unexpected vitest render output: %q", rendered)
	}
	if vitest.StreamPreference != engine.StreamStdoutFirst || vitest.StreamRender == nil {
		t.Fatalf("unexpected vitest stream metadata: %#v", vitest)
	}

	jest := testutil.FindProfile(t, list, "jest-json")
	if rendered := jest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL src/b.test.ts\nExpected: 2"}); !strings.Contains(rendered, "FAIL src/b.test.ts") {
		t.Fatalf("unexpected jest render output: %q", rendered)
	}
	if jest.StreamPreference != engine.StreamStdoutFirst || jest.StreamRender == nil {
		t.Fatalf("unexpected jest stream metadata: %#v", jest)
	}
}

func TestJSWorkspaceRendererCoverage(t *testing.T) {
	list := profiles.Builtins(4)
	workspace := testutil.FindProfile(t, list, "js-workspace")
	rendered := workspace.Render(engine.Invocation{}, engine.Execution{
		Stderr: strings.Join([]string{
			"vite v5.4.0 building for production...",
			"src/app.ts:14:7: error: Cannot find module 'x'",
			"npm ERR! code ELIFECYCLE",
		}, "\n"),
	})
	for _, want := range []string{"src/app.ts:14:7: error: Cannot find module 'x'", "npm ERR! code ELIFECYCLE"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in js-workspace render output:\n%s", want, rendered)
		}
	}
	if workspace.StreamPreference != engine.StreamStdoutFirst || workspace.StreamRender == nil {
		t.Fatalf("unexpected js-workspace stream metadata: %#v", workspace)
	}
}
