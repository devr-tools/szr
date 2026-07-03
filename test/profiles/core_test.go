package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBuiltInProfileCount(t *testing.T) {
	list := profiles.Builtins(3)
	if len(list) != 75 {
		t.Fatalf("expected 75 profiles, got %d", len(list))
	}
}

func TestCoreFilesystemProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	directoryListing := testutil.FindProfile(t, list, "directory-listing")
	if !directoryListing.Match(engine.Invocation{Command: []string{"ls"}}) || !directoryListing.Match(engine.Invocation{Command: []string{"tree"}}) {
		t.Fatal("directory-listing should match ls and tree")
	}
	if got := directoryListing.Prepare(engine.Invocation{Command: []string{"ls", "docs"}}); len(got) < 4 || got[len(got)-2] != "-1" || got[len(got)-1] != "-p" {
		t.Fatalf("expected ls prepare to normalize output, got %#v", got)
	}
	if got := directoryListing.Render(engine.Invocation{Command: []string{"ls"}}, engine.Execution{Stdout: "cmd/\ndocs/\nREADME.md\n"}); got == "" {
		t.Fatal("expected directory-listing render output")
	}

	catRead := testutil.FindProfile(t, list, "cat-read")
	if !catRead.Match(engine.Invocation{Command: []string{"cat", "README.md"}}) || catRead.Match(engine.Invocation{Command: []string{"cat", "-n", "README.md"}}) {
		t.Fatal("unexpected cat-read match behavior")
	}
	if catRead.Match(engine.Invocation{Command: []string{"cat", "data.json"}}) {
		t.Fatal("cat-read should not match JSON files")
	}
	if got := catRead.Render(engine.Invocation{Command: []string{"cat", "README.md"}}, engine.Execution{Stdout: "# Title\n\nBody\n"}); got == "" {
		t.Fatal("expected cat-read render output")
	}
	if got := catRead.Render(engine.Invocation{Command: []string{"cat", "main.go"}}, engine.Execution{Stdout: "package main\n\nfunc main() {\n\tprintln(\"x\")\n}\n"}); !strings.Contains(got, "1  package main") || !strings.Contains(got, "3  func main() { ... }") || strings.Contains(got, "println(\"x\")") {
		t.Fatalf("expected cat-read to keep signatures and drop body lines, got %q", got)
	}

	gitLsFiles := testutil.FindProfile(t, list, "git-ls-files")
	if !gitLsFiles.Match(engine.Classify(engine.Invocation{Display: []string{"git", "ls-files"}})) {
		t.Fatal("git-ls-files should match git ls-files")
	}
	if got := gitLsFiles.Render(engine.Invocation{}, engine.Execution{Stdout: "cmd/main.go\ninternal/app.go\n"}); got == "" {
		t.Fatal("expected git-ls-files render output")
	}
	if gitLsFiles.StreamPreference != engine.StreamStdoutOnly || gitLsFiles.StreamRender == nil {
		t.Fatalf("unexpected git-ls-files stream metadata: %#v", gitLsFiles)
	}
}

func TestSelectionPrefersSpecificConflictProfiles(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	e := engine.New(cfg, paths, nil, profiles.Builtins(cfg.MaxPreviewLines))

	if got := e.Explain(engine.Invocation{
		Command: []string{"cat", "data.json"},
		Display: []string{"cat", "data.json"},
	}).Name; got != "json-query" {
		t.Fatalf("expected cat data.json to select json-query, got %q", got)
	}

	if got := e.Explain(engine.Invocation{
		Command: []string{"kubectl", "get", "pods", "-o", "wide"},
		Display: []string{"kubectl", "get", "pods", "-o", "wide"},
	}).Name; got != "csv-tabular" {
		t.Fatalf("expected kubectl get -o wide to select csv-tabular, got %q", got)
	}

	if got := e.Explain(engine.Invocation{
		Command: []string{"curl", "https://api.example.test/v1/users"},
		Display: []string{"curl", "https://api.example.test/v1/users"},
	}).Name; got != "http-api" {
		t.Fatalf("expected API curl request to select http-api, got %q", got)
	}

	if got := e.Explain(engine.Invocation{
		Command: []string{"vercel", "ls"},
		Display: []string{"vercel", "ls"},
	}).Name; got != "vercel-deployments" {
		t.Fatalf("expected vercel ls to select vercel-deployments, got %q", got)
	}

	if got := e.Explain(engine.Invocation{
		Command: []string{"supabase", "functions", "logs", "stripe-webhook"},
		Display: []string{"supabase", "functions", "logs", "stripe-webhook"},
	}).Name; got != "supabase-function-logs" {
		t.Fatalf("expected supabase functions logs to select supabase-function-logs, got %q", got)
	}

	if got := e.Explain(engine.Invocation{
		Command: []string{"heroku", "logs", "--app", "api-prod"},
		Display: []string{"heroku", "logs", "--app", "api-prod"},
	}).Name; got != "heroku-router-logs" {
		t.Fatalf("expected heroku logs to select heroku-router-logs, got %q", got)
	}
}

func TestGoProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	t.Run("go-test-json", func(t *testing.T) {
		goTest := testutil.FindProfile(t, list, "go-test-json")
		assertGoTestProfile(t, goTest)
	})
	t.Run("generic-summary", func(t *testing.T) {
		genericSummary := testutil.FindProfile(t, list, "generic-summary")
		assertGenericSummaryProfile(t, genericSummary)
	})
	t.Run("go-build", func(t *testing.T) {
		goBuild := testutil.FindProfile(t, list, "go-build")
		assertGoBuildProfile(t, goBuild)
	})
	t.Run("go-lint", func(t *testing.T) {
		goLint := testutil.FindProfile(t, list, "go-lint")
		assertGoLintProfile(t, goLint)
	})
}

func assertGoTestProfile(t *testing.T, goTest engine.Profile) {
	t.Helper()

	if !goTest.Match(engine.Invocation{Display: []string{"go", "test"}}) || !goTest.Match(engine.Invocation{Command: []string{"go", "test"}}) {
		t.Fatal("go-test-json should match")
	}
	if len(goTest.Prepare(engine.Invocation{Command: []string{"go", "test", "./..."}})) != 4 {
		t.Fatal("expected go-test-json to add -json")
	}
	if len(goTest.Prepare(engine.Invocation{Command: []string{"go", "test", "-json"}})) != 3 {
		t.Fatal("expected go-test-json to preserve -json")
	}
	if goTest.StreamPreference != engine.StreamStdoutOnly || goTest.StreamRender == nil {
		t.Fatalf("unexpected go-test-json stream metadata: %#v", goTest)
	}

	goTestStream := goTest.StreamRender(engine.Invocation{}, goTest.Budget)
	goTestStream.ConsumeStdout([]byte(strings.Join([]string{
		`{"Action":"pass","Package":"github.com/acme/pass"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestOne"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestTwo"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestThree"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestFour"}`,
		`{"Action":"fail","Package":"github.com/acme/fail","Test":"TestFive"}`,
	}, "\n")))
	assertRecoveryCapableProfile(t, goTestStream, filters.RecoveryKindFullOutput, "omitted 1 additional test lines", true)
}

func assertGenericSummaryProfile(t *testing.T, genericSummary engine.Profile) {
	t.Helper()

	summaryStream := genericSummary.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	assertRecoveryCapableProfileAfterStdout(t, summaryStream, "line-1\nline-2\nline-3\n", filters.RecoveryKindFullOutput, "omitted 1 additional line", true)
}

func assertGoBuildProfile(t *testing.T, goBuild engine.Profile) {
	t.Helper()

	if !goBuild.Match(engine.Invocation{Display: []string{"go", "build"}}) || !goBuild.Match(engine.Invocation{Display: []string{"go", "vet"}}) {
		t.Fatal("go-build should match build and vet")
	}
	if goBuild.Match(engine.Invocation{Display: []string{"go", "test"}}) {
		t.Fatal("go-build should not match go test")
	}
	if got := goBuild.Render(engine.Invocation{}, engine.Execution{Stdout: "noise", Stderr: "error: bad"}); got == "" {
		t.Fatal("expected go-build render output")
	}
	if goBuild.StreamPreference != engine.StreamStderrFirst || goBuild.StreamRender == nil {
		t.Fatalf("unexpected go-build stream metadata: %#v", goBuild)
	}

	stream := goBuild.StreamRender(engine.Invocation{}, goBuild.Budget)
	stream.ConsumeStderr([]byte("error: bad\n"))
	stream.ConsumeStdout([]byte("noise\n"))
	if got := stream.Result(); got != "error: bad" {
		t.Fatalf("unexpected go-build stream output: %q", got)
	}
}

func assertGoLintProfile(t *testing.T, goLint engine.Profile) {
	t.Helper()

	if !goLint.Match(engine.Invocation{Display: []string{"golangci-lint", "run", "./..."}}) || !goLint.Match(engine.Invocation{Command: []string{"golangci-lint", "run", "--fix"}}) {
		t.Fatal("go-lint should match golangci-lint with any subcommand")
	}
	if !goLint.Match(engine.Invocation{Display: []string{"staticcheck", "./..."}}) {
		t.Fatal("go-lint should match staticcheck")
	}
	if goLint.Match(engine.Invocation{Display: []string{"go", "vet", "./..."}}) {
		t.Fatal("go-lint should leave go vet to go-build")
	}
	if goLint.StreamPreference != engine.StreamStdoutFirst || goLint.StreamRender == nil {
		t.Fatalf("unexpected go-lint stream metadata: %#v", goLint)
	}

	issues := "internal/engine/run.go:42:2: ineffectual assignment to err (ineffassign)\n" +
		"internal/cli/app.go:118:1: cognitive complexity 31 of func `run` is high (> 15) (gocognit)\n"
	rendered := goLint.Render(engine.Invocation{}, engine.Execution{
		Stdout: issues,
		Stderr: "level=warning msg=\"[config] deprecated option\"\n",
	})
	if !strings.Contains(rendered, "run.go:42:2: ineffectual assignment") {
		t.Fatalf("expected go-lint render to keep failed-lint anchors, got %q", rendered)
	}

	stream := goLint.StreamRender(engine.Invocation{}, goLint.Budget)
	stream.ConsumeStdout([]byte(issues))
	stream.ConsumeStderr([]byte("2 issues:\n* ineffassign: 1\n* gocognit: 1\n"))
	if got := stream.Result(); !strings.Contains(got, "run.go:42:2") || !strings.Contains(got, "app.go:118:1") {
		t.Fatalf("expected go-lint stream output to keep lint anchors, got %q", got)
	}
}

func assertRecoveryCapableProfileAfterStdout(t *testing.T, stream engine.StreamReducer, stdout, wantKind, wantSummary string, wantRequireRawCapture bool) {
	t.Helper()
	stream.ConsumeStdout([]byte(stdout))
	assertRecoveryCapableProfile(t, stream, wantKind, wantSummary, wantRequireRawCapture)
}

func assertRecoveryCapableProfile(t *testing.T, stream engine.StreamReducer, wantKind, wantSummary string, wantRequireRawCapture bool) {
	t.Helper()

	recovery, ok := stream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable reducer, got %T", stream)
	}
	kind, summary, requireRawCapture := recovery.RecoveryInfo()
	if kind != wantKind || summary != wantSummary || requireRawCapture != wantRequireRawCapture {
		t.Fatalf("unexpected recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestPytestProfile(t *testing.T) {
	list := profiles.Builtins(3)
	pytest := testutil.FindProfile(t, list, "pytest")
	if !pytest.Match(engine.Invocation{Display: []string{"pytest"}}) || !pytest.Match(engine.Invocation{Display: []string{"uv", "run", "pytest"}}) {
		t.Fatal("pytest profile should match direct and uv-wrapped invocations")
	}
	if pytest.Match(engine.Invocation{Display: []string{"python", "-m", "unittest"}}) {
		t.Fatal("pytest profile should not match unittest")
	}
	if got := pytest.Render(engine.Invocation{}, engine.Execution{Stdout: "collected 1 item\n\n============================== 1 passed in 0.03s ==============================\n"}); got == "" {
		t.Fatal("expected pytest render output")
	}
	if pytest.StreamPreference != engine.StreamStdoutFirst || pytest.StreamRender == nil {
		t.Fatalf("unexpected pytest stream metadata: %#v", pytest)
	}
}

func TestCargoProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	cargoTest := testutil.FindProfile(t, list, "cargo-test")
	if !cargoTest.Match(engine.Invocation{Display: []string{"cargo", "test"}}) || !cargoTest.Match(engine.Invocation{Command: []string{"cargo", "test"}}) {
		t.Fatal("cargo-test should match cargo test")
	}
	if cargoTest.Match(engine.Invocation{Display: []string{"cargo", "build"}}) {
		t.Fatal("cargo-test should not match cargo build")
	}
	if got := cargoTest.Render(engine.Invocation{}, engine.Execution{Stdout: "running 1 test\ntest result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out\n"}); got == "" {
		t.Fatal("expected cargo-test render output")
	}
	if cargoTest.StreamPreference != engine.StreamStdoutFirst || cargoTest.StreamRender == nil {
		t.Fatalf("unexpected cargo-test stream metadata: %#v", cargoTest)
	}

	cargoBuild := testutil.FindProfile(t, list, "cargo-build")
	if !cargoBuild.Match(engine.Invocation{Display: []string{"cargo", "build"}}) || !cargoBuild.Match(engine.Invocation{Display: []string{"cargo", "clippy"}}) {
		t.Fatal("cargo-build should match cargo build and clippy")
	}
	if got := cargoBuild.Render(engine.Invocation{}, engine.Execution{Stderr: "error[E0432]: unresolved import `x`\n--> src/lib.rs:1:1\n"}); got == "" {
		t.Fatal("expected cargo-build render output")
	}
	if cargoBuild.StreamPreference != engine.StreamStderrFirst || cargoBuild.StreamRender == nil {
		t.Fatalf("unexpected cargo-build stream metadata: %#v", cargoBuild)
	}
}

func TestBuildAndClangProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	buildSystem := testutil.FindProfile(t, list, "build-system")
	if !buildSystem.Match(engine.Invocation{Display: []string{"make", "test"}}) || !buildSystem.Match(engine.Invocation{Display: []string{"ninja"}}) {
		t.Fatal("build-system should match build orchestrators")
	}
	if got := buildSystem.Render(engine.Invocation{}, engine.Execution{Stdout: "FAILED: app\nninja: error: subcommand failed\n"}); got == "" {
		t.Fatal("expected build-system render output")
	}
	if buildSystem.StreamPreference != engine.StreamStdoutFirst || buildSystem.StreamRender == nil {
		t.Fatalf("unexpected build-system stream metadata: %#v", buildSystem)
	}

	ctest := testutil.FindProfile(t, list, "ctest")
	if !ctest.Match(engine.Invocation{Display: []string{"ctest"}}) {
		t.Fatal("ctest should match ctest")
	}
	if got := ctest.Prepare(engine.Invocation{Command: []string{"ctest"}}); len(got) != 2 || got[1] != "--output-on-failure" {
		t.Fatalf("expected ctest prepare to add --output-on-failure, got %#v", got)
	}
	if got := ctest.Render(engine.Invocation{}, engine.Execution{Stdout: "The following tests FAILED:\n2 - api_smoke (Failed)\n"}); got == "" {
		t.Fatal("expected ctest render output")
	}

	clangTooling := testutil.FindProfile(t, list, "clang-tooling")
	if !clangTooling.Match(engine.Invocation{Display: []string{"clang-tidy", "src/main.cpp"}}) || !clangTooling.Match(engine.Invocation{Display: []string{"bear", "--", "make"}}) {
		t.Fatal("clang-tooling should match clang tooling and bear")
	}
	if got := clangTooling.Render(engine.Invocation{}, engine.Execution{Stderr: "src/main.cpp:10:5: warning: boom [modernize-use-nullptr]\n"}); got == "" {
		t.Fatal("expected clang-tooling render output")
	}
}

func TestPatchAndContainerProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	patchDiff := testutil.FindProfile(t, list, "patch-diff")
	if !patchDiff.Match(engine.Invocation{Display: []string{"diff", "-u", "a", "b"}}) || !patchDiff.Match(engine.Invocation{Display: []string{"git", "apply", "fix.patch"}}) {
		t.Fatal("patch-diff should match diff and git apply")
	}
	if got := patchDiff.Render(engine.Invocation{}, engine.Execution{Stdout: "diff --git a/a.txt b/a.txt\n@@ -1 +1 @@\n"}); got == "" {
		t.Fatal("expected patch-diff render output")
	}

	dockerPS := testutil.FindProfile(t, list, "docker-ps")
	if !dockerPS.Match(engine.Invocation{Display: []string{"docker", "ps"}}) || !dockerPS.Match(engine.Invocation{Display: []string{"docker", "compose", "ps"}}) {
		t.Fatal("docker-ps should match docker ps and docker compose ps")
	}
	if got := dockerPS.Render(engine.Invocation{}, engine.Execution{Stdout: "api\tUp 2m\tapp\n"}); got == "" {
		t.Fatal("expected docker-ps render output")
	}
	if dockerPS.StreamPreference != engine.StreamStdoutOnly || dockerPS.StreamRender == nil {
		t.Fatalf("unexpected docker-ps stream metadata: %#v", dockerPS)
	}

	dockerLogs := testutil.FindProfile(t, list, "docker-logs")
	if !dockerLogs.Match(engine.Invocation{Display: []string{"docker", "logs", "api"}}) || !dockerLogs.Match(engine.Invocation{Display: []string{"docker", "compose", "logs", "api"}}) {
		t.Fatal("docker-logs should match docker logs and docker compose logs")
	}
	if got := dockerLogs.Render(engine.Invocation{}, engine.Execution{Stdout: "api | ERROR failed\n"}); got == "" {
		t.Fatal("expected docker-logs render output")
	}
	if dockerLogs.StreamPreference != engine.StreamStdoutFirst || dockerLogs.StreamRender == nil {
		t.Fatalf("unexpected docker-logs stream metadata: %#v", dockerLogs)
	}
}

func TestKubectlProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	kubectlGet := testutil.FindProfile(t, list, "kubectl-get")
	if !kubectlGet.Match(engine.Invocation{Display: []string{"kubectl", "get", "pods"}}) || !kubectlGet.Match(engine.Invocation{Display: []string{"kubectl", "-n", "default", "get", "pods"}}) {
		t.Fatal("kubectl-get should match direct and namespaced kubectl get")
	}
	if got := kubectlGet.Render(engine.Invocation{}, engine.Execution{Stdout: `{"kind":"PodList","items":[{"kind":"Pod","metadata":{"name":"api","namespace":"default"},"status":{"phase":"Running"}}]}`}); got == "" {
		t.Fatal("expected kubectl-get render output")
	}
	if kubectlGet.StreamPreference != engine.StreamStdoutOnly || kubectlGet.StreamRender == nil {
		t.Fatalf("unexpected kubectl-get stream metadata: %#v", kubectlGet)
	}

	kubectlDescribe := testutil.FindProfile(t, list, "kubectl-describe")
	if !kubectlDescribe.Match(engine.Invocation{Display: []string{"kubectl", "describe", "pod", "api"}}) {
		t.Fatal("kubectl-describe should match kubectl describe")
	}
	if got := kubectlDescribe.Render(engine.Invocation{}, engine.Execution{Stdout: "Name: api\nNamespace: default\nStatus: Running\n"}); got == "" {
		t.Fatal("expected kubectl-describe render output")
	}
	if kubectlDescribe.StreamPreference != engine.StreamStdoutOnly || kubectlDescribe.StreamRender == nil {
		t.Fatalf("unexpected kubectl-describe stream metadata: %#v", kubectlDescribe)
	}

	kubectlLogs := testutil.FindProfile(t, list, "kubectl-logs")
	if !kubectlLogs.Match(engine.Invocation{Display: []string{"kubectl", "logs", "api"}}) || !kubectlLogs.Match(engine.Invocation{Display: []string{"kubectl", "--namespace", "prod", "logs", "api"}}) {
		t.Fatal("kubectl-logs should match direct and namespaced kubectl logs")
	}
	if got := kubectlLogs.Render(engine.Invocation{}, engine.Execution{Stdout: "api/api ERROR failed\n"}); got == "" {
		t.Fatal("expected kubectl-logs render output")
	}
	if kubectlLogs.StreamPreference != engine.StreamStdoutFirst || kubectlLogs.StreamRender == nil {
		t.Fatalf("unexpected kubectl-logs stream metadata: %#v", kubectlLogs)
	}
}

func TestGitHubProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	ghPR := testutil.FindProfile(t, list, "gh-pr-view")
	if !ghPR.Match(engine.Invocation{Display: []string{"gh", "pr", "view"}}) || !ghPR.Match(engine.Invocation{Display: []string{"gh", "-R", "owner/repo", "pr", "view", "1"}}) {
		t.Fatal("gh-pr-view should match plain and repo-scoped invocations")
	}
	if got := ghPR.Render(engine.Invocation{}, engine.Execution{Stdout: `{"number":1,"title":"T","state":"OPEN","isDraft":false,"headRefName":"x","baseRefName":"main","files":[]}`}); got == "" {
		t.Fatal("expected gh-pr-view render output")
	}
	if ghPR.StreamPreference != engine.StreamStdoutOnly || ghPR.StreamRender == nil {
		t.Fatalf("unexpected gh-pr-view stream metadata: %#v", ghPR)
	}

	ghRun := testutil.FindProfile(t, list, "gh-run-view")
	if !ghRun.Match(engine.Invocation{Display: []string{"gh", "run", "view", "1"}}) || !ghRun.Match(engine.Invocation{Display: []string{"gh", "--repo", "owner/repo", "run", "view", "1"}}) {
		t.Fatal("gh-run-view should match plain and repo-scoped invocations")
	}
	if got := ghRun.Render(engine.Invocation{}, engine.Execution{Stdout: `{"workflowName":"CI","status":"completed","conclusion":"success","jobs":[]}`}); got == "" {
		t.Fatal("expected gh-run-view render output")
	}
	if ghRun.StreamPreference != engine.StreamStdoutFirst || ghRun.StreamRender == nil {
		t.Fatalf("unexpected gh-run-view stream metadata: %#v", ghRun)
	}

	ghRunLog := testutil.FindProfile(t, list, "gh-run-log")
	if !ghRunLog.Match(engine.Invocation{Display: []string{"gh", "run", "view", "1", "--log"}}) {
		t.Fatal("gh-run-log should match raw log mode")
	}
	if got := ghRunLog.Render(engine.Invocation{}, engine.Execution{Stdout: "test\tunit\terror: failed\n"}); got == "" {
		t.Fatal("expected gh-run-log render output")
	}
	if ghRunLog.StreamPreference != engine.StreamStdoutFirst || ghRunLog.StreamRender == nil {
		t.Fatalf("unexpected gh-run-log stream metadata: %#v", ghRunLog)
	}
}

func TestWorkspaceAndToolingProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	jsWorkspace := testutil.FindProfile(t, list, "js-workspace")
	if !jsWorkspace.Match(engine.Classify(engine.Invocation{Display: []string{"turbo", "run", "build"}})) || !jsWorkspace.Match(engine.Classify(engine.Invocation{Display: []string{"npm", "install"}})) {
		t.Fatal("js-workspace should match workspace tooling commands")
	}
	if got := jsWorkspace.Render(engine.Invocation{}, engine.Execution{Stderr: "npm ERR! missing script: build\n"}); got == "" {
		t.Fatal("expected js-workspace render output")
	}

	pythonTooling := testutil.FindProfile(t, list, "python-tooling")
	if !pythonTooling.Match(engine.Invocation{Display: []string{"poetry", "install"}}) || !pythonTooling.Match(engine.Invocation{Display: []string{"mypy", "src"}}) {
		t.Fatal("python-tooling should match python tooling commands")
	}
	if got := pythonTooling.Render(engine.Invocation{}, engine.Execution{Stderr: "src/app.py:3: error: Name \"x\" is not defined  [name-defined]\n"}); got == "" {
		t.Fatal("expected python-tooling render output")
	}

	ripgrep := testutil.FindProfile(t, list, "ripgrep")
	if !ripgrep.Match(engine.Classify(engine.Invocation{Display: []string{"rg", "needle", "."}})) {
		t.Fatal("ripgrep should match plain rg commands")
	}
	if ripgrep.Match(engine.Classify(engine.Invocation{Display: []string{"rg", "--json", "needle"}})) {
		t.Fatal("ripgrep should not match json rg mode")
	}
	if got := ripgrep.Render(engine.Invocation{}, engine.Execution{Stdout: "a.go:1:hit\nb.go:2:hit\n"}); got == "" {
		t.Fatal("expected ripgrep render output")
	}
	if ripgrep.StreamPreference != engine.StreamStdoutFirst || ripgrep.StreamRender == nil {
		t.Fatalf("unexpected ripgrep stream metadata: %#v", ripgrep)
	}
}

func TestGenericTestProfile(t *testing.T) {
	list := profiles.Builtins(3)
	genericTest := testutil.FindProfile(t, list, "generic-test")
	if !genericTest.Match(engine.Invocation{Display: []string{"test", "pytest"}}) || genericTest.Match(engine.Invocation{Display: nil}) {
		t.Fatal("unexpected generic-test match behavior")
	}
	if genericTest.Match(engine.Invocation{Display: []string{"test", "cargo", "test"}, Command: []string{"cargo", "test"}}) {
		t.Fatal("generic-test should defer to specialized wrapped cargo test handling")
	}
	if genericTest.Match(engine.Invocation{Display: []string{"test", "npm", "test"}, Command: []string{"npm", "test"}}) {
		t.Fatal("generic-test should defer to specialized wrapped js test handling")
	}
	if got := genericTest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL one"}); got == "" {
		t.Fatal("expected generic-test render output")
	}
	if genericTest.StreamPreference != engine.StreamStdoutFirst || genericTest.StreamRender == nil {
		t.Fatalf("unexpected generic-test stream metadata: %#v", genericTest)
	}
}
