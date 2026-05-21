package profiles_test

import (
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestBuiltInProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	if len(list) != 21 {
		t.Fatalf("expected 21 profiles, got %d", len(list))
	}

	goTest := testutil.FindProfile(t, list, "go-test-json")
	if !goTest.Match(engine.Invocation{Display: []string{"go", "test"}}) {
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

	goBuild := testutil.FindProfile(t, list, "go-build")
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
	goBuildStream := goBuild.StreamRender(engine.Invocation{}, goBuild.Budget)
	goBuildStream.ConsumeStderr([]byte("error: bad\n"))
	goBuildStream.ConsumeStdout([]byte("noise\n"))
	if got := goBuildStream.Result(); got != "error: bad" {
		t.Fatalf("unexpected go-build stream output: %q", got)
	}

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

	cargoTest := testutil.FindProfile(t, list, "cargo-test")
	if !cargoTest.Match(engine.Invocation{Display: []string{"cargo", "test"}}) {
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

	genericTest := testutil.FindProfile(t, list, "generic-test")
	if !genericTest.Match(engine.Invocation{Display: []string{"test", "pytest"}}) || genericTest.Match(engine.Invocation{Display: nil}) {
		t.Fatal("unexpected generic-test match behavior")
	}
	if got := genericTest.Render(engine.Invocation{}, engine.Execution{Stdout: "FAIL one"}); got == "" {
		t.Fatal("expected generic-test render output")
	}
	if genericTest.StreamPreference != engine.StreamStdoutFirst || genericTest.StreamRender == nil {
		t.Fatalf("unexpected generic-test stream metadata: %#v", genericTest)
	}

	genericSummary := testutil.FindProfile(t, list, "generic-summary")
	if !genericSummary.Match(engine.Invocation{Display: []string{"summary", "cmd"}}) || genericSummary.Match(engine.Invocation{Display: nil}) {
		t.Fatal("unexpected generic-summary match behavior")
	}
	if got := genericSummary.Render(engine.Invocation{}, engine.Execution{Stdout: "a\nb\nc\nd"}); got == "" {
		t.Fatal("expected generic-summary render output")
	}
	if genericSummary.StreamPreference != engine.StreamStdoutFirst || genericSummary.StreamRender == nil {
		t.Fatalf("unexpected generic-summary stream metadata: %#v", genericSummary)
	}
	genericSummaryStream := genericSummary.StreamRender(engine.Invocation{}, genericSummary.Budget)
	genericSummaryStream.ConsumeStdout([]byte("a\nb\nc\n"))
	if got := genericSummaryStream.Result(); got == "" {
		t.Fatal("expected generic-summary stream output")
	}
}
