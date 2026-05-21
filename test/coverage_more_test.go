package test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/filters"
	"szr/internal/history"
	"szr/internal/installers"
	"szr/internal/profiles"
	"szr/internal/rules"
)

func TestInstallersCoverageEdges(t *testing.T) {
	root := t.TempDir()

	if _, err := installers.DetectPathsWith(filepath.Join(root, "missing"), os.Stat); err == nil {
		t.Fatal("expected detect paths missing-root error")
	}

	if _, err := installers.RenderAll(installers.Options{RepoRoot: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("expected render all error for missing repo root")
	}

	plan := installers.Plan{
		Files: []installers.File{{
			Path:     filepath.Join(root, "AGENTS.md"),
			Content:  "",
			Mode:     0o644,
			Strategy: installers.StrategyMerge,
			Marker:   "szr-empty",
		}},
	}
	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply empty merge plan: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read empty merge file: %v", err)
	}
	if !strings.Contains(string(data), "<!-- szr-empty:begin -->") {
		t.Fatalf("expected empty merge markers, got %q", string(data))
	}
}

func TestJSProfileCoverageEdges(t *testing.T) {
	list := profiles.Builtins(4)
	find := func(name string) engine.Profile {
		t.Helper()
		for _, profile := range list {
			if profile.Name == name {
				return profile
			}
		}
		t.Fatalf("missing profile %s", name)
		return engine.Profile{}
	}

	pm := find("js-package-test")
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
	if got := pm.Prepare(engine.Invocation{Command: []string{"bun", "test"}}); !reflect.DeepEqual(got, []string{"bun", "test"}) {
		t.Fatalf("expected unknown package manager passthrough, got %#v", got)
	}
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"jest"}}`)
	if got := pm.Prepare(engine.Invocation{Cwd: root}); got != nil {
		t.Fatalf("expected nil prepare for empty package manager command, got %#v", got)
	}

	vitest := find("vitest-json")
	if got := vitest.Prepare(engine.Invocation{}); !reflect.DeepEqual(got, []string{"--reporter=json"}) {
		t.Fatalf("expected structured vitest args for empty command, got %#v", got)
	}
	if rendered := vitest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL src/a.test.ts\nExpected: 1"}); !strings.Contains(rendered, "FAIL src/a.test.ts") {
		t.Fatalf("unexpected vitest render output: %q", rendered)
	}

	jest := find("jest-json")
	if got := jest.Prepare(engine.Invocation{Command: []string{"jest", "--outputFile=report.json"}}); !reflect.DeepEqual(got, []string{"jest", "--outputFile=report.json"}) {
		t.Fatalf("expected jest output file to be preserved, got %#v", got)
	}
	if rendered := jest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL src/b.test.ts\nExpected: 2"}); !strings.Contains(rendered, "FAIL src/b.test.ts") {
		t.Fatalf("unexpected jest render output: %q", rendered)
	}
}

func TestJSTextCoverageEdges(t *testing.T) {
	got := filters.SummarizeJSTest(strings.Join([]string{
		"FAIL src/a.test.ts",
		"Expected: 1",
		"Received: 2",
		"Test Suites: 1 failed",
		"Tests: 1 failed",
		"Snapshots: 0 total",
		"Time: 0.01s",
	}, "\n"), 2)
	for _, want := range []string{"FAIL src/a.test.ts", "Test Suites: 1 failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in tight js summary:\n%s", want, got)
		}
	}

	embedded := filters.SummarizeJSTest("banner\n{\"numPassedTests\":1,\"numFailedTests\":0,\"numTotalTests\":1,\"numPassedTestSuites\":1,\"numFailedTestSuites\":0,\"testResults\":[]}\ntrailer", 3)
	if !strings.Contains(embedded, "all tests passed") {
		t.Fatalf("expected embedded json parse, got %q", embedded)
	}

	textFallback := filters.SummarizeJSTest("not json\nstill not json\nFAIL x", 1)
	if !strings.Contains(textFallback, "FAIL x") {
		t.Fatalf("expected plain text fallback, got %q", textFallback)
	}

	empty := filters.SummarizeJSTest("", 2)
	if empty != "" {
		t.Fatalf("expected empty js summary, got %q", empty)
	}

	statusOnly := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/status-only.test.ts","status":"failed","message":"","assertionResults":[]}]}`, 1)
	if !strings.Contains(statusOnly, "src/status-only.test.ts") {
		t.Fatalf("expected status-only suite to survive summary, got %q", statusOnly)
	}

	unnamed := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/unnamed.test.ts","status":"passed","message":"","assertionResults":[{"ancestorTitles":[],"fullName":"","title":"","status":"failed","failureMessages":["Error: unnamed failure"]}]}]}`, 4)
	if !strings.Contains(unnamed, "Error: unnamed failure") {
		t.Fatalf("expected unnamed failure detail to survive summary, got %q", unnamed)
	}

	mixed := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":1,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":2,"success":false,"testResults":[{"name":"src/mixed.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["mixed"],"fullName":"mixed passes","title":"passes","status":"passed","failureMessages":[]},{"ancestorTitles":["mixed"],"fullName":"mixed fails","title":"fails","status":"failed","failureMessages":["Error: mixed failure"]}]}]}`, 5)
	if !strings.Contains(mixed, "mixed fails") || strings.Contains(mixed, "mixed passes") {
		t.Fatalf("expected only failing assertion details in mixed suite summary, got %q", mixed)
	}

	noData := filters.SummarizeJSTest("{}", 2)
	if noData != "{}" {
		t.Fatalf("expected no-data report to fall back to compact output, got %q", noData)
	}
}

func TestCLIBenchCoverageEdges(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), engine.New(config.Default(), paths, history.New(paths.HistoryFile), nil))
	code, stdout, stderr := runApp(t, app, "bench", "clean-pass")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to benchmark clean-pass") {
		t.Fatalf("unexpected bench failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	okApp := newTestApp(t)
	code, stdout, stderr = runApp(t, okApp, "bench", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected bench all json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode bench all json: %v", err)
	}
	if len(payload) != 5 {
		t.Fatalf("expected all benchmark fixtures, got %#v", payload)
	}
}

func TestRuleDiscoverYMLEdge(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".szr.yml"), []byte("profiles: []\n"), 0o644); err != nil {
		t.Fatalf("write yml rule file: %v", err)
	}
	path, format, err := rules.DiscoverWith(root, os.Stat)
	if err != nil || format != rules.FormatYAML || !strings.HasSuffix(path, ".szr.yml") {
		t.Fatalf("unexpected yml discovery path=%q format=%q err=%v", path, format, err)
	}

	_, _, err = rules.DiscoverWith(root, func(string) (os.FileInfo, error) {
		return nil, errors.New("discover fail")
	})
	if err == nil || !strings.Contains(err.Error(), "discover fail") {
		t.Fatalf("expected discovery stat error, got %v", err)
	}
}
