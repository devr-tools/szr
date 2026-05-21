package profiles_test

import (
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestBuiltInProfiles(t *testing.T) {
	list := profiles.Builtins(3)
	if len(list) != 10 {
		t.Fatalf("expected 10 profiles, got %d", len(list))
	}

	gitStatus := testutil.FindProfile(t, list, "git-status")
	if !gitStatus.Match(engine.Invocation{Display: []string{"git", "status"}}) {
		t.Fatal("git-status should match")
	}
	preparedStatus := gitStatus.Prepare(engine.Invocation{Command: []string{"git", "status"}})
	if preparedStatus[len(preparedStatus)-1] != "--branch" {
		t.Fatalf("unexpected prepared status: %#v", preparedStatus)
	}
	preparedStatus = gitStatus.Prepare(engine.Invocation{Command: []string{"git", "status", "--short"}})
	if len(preparedStatus) != 3 {
		t.Fatalf("expected passthrough status prepare: %#v", preparedStatus)
	}
	if got := gitStatus.Render(engine.Invocation{}, engine.Execution{Stdout: "\x1b[31m## main\x1b[0m\nM  a\n"}); got == "" {
		t.Fatal("expected git-status render output")
	}
	if gitStatus.StreamPreference != engine.StreamStdoutOnly || gitStatus.StreamRender == nil || gitStatus.Budget.MaxLines < 3 {
		t.Fatalf("unexpected git-status stream metadata: %#v", gitStatus)
	}
	gitStatusStream := gitStatus.StreamRender(engine.Invocation{}, gitStatus.Budget)
	gitStatusStream.ConsumeStdout([]byte("## main\nM  a\n"))
	if got := gitStatusStream.Result(); got == "" || gitStatusStream.BytesParsed() == 0 {
		t.Fatalf("expected git-status stream output, got %q", got)
	}

	gitLog := testutil.FindProfile(t, list, "git-log")
	if !gitLog.Match(engine.Invocation{Display: []string{"git", "log"}}) {
		t.Fatal("git-log should match")
	}
	if len(gitLog.Prepare(engine.Invocation{Command: []string{"git", "log"}})) <= 2 {
		t.Fatal("expected git-log prepare to add flags")
	}
	if len(gitLog.Prepare(engine.Invocation{Command: []string{"git", "log", "--format=%H"}})) != 3 {
		t.Fatal("expected git-log to preserve explicit format")
	}
	if got := gitLog.Render(engine.Invocation{}, engine.Execution{Stdout: "abc one\ndef two\n"}); got == "" {
		t.Fatal("expected git-log render output")
	}
	if gitLog.StreamPreference != engine.StreamStdoutOnly || gitLog.StreamRender == nil {
		t.Fatalf("unexpected git-log stream metadata: %#v", gitLog)
	}

	gitDiff := testutil.FindProfile(t, list, "git-diff")
	if !gitDiff.Match(engine.Invocation{Display: []string{"git", "diff"}}) {
		t.Fatal("git-diff should match")
	}
	if len(gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}})) != 3 {
		t.Fatal("expected git-diff to add stat flag")
	}
	if len(gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff", "--stat"}})) != 3 {
		t.Fatal("expected git-diff to preserve explicit stat")
	}
	if got := gitDiff.Render(engine.Invocation{}, engine.Execution{Stdout: "diff --git a/a b/a\n a | 1 +\n"}); got == "" {
		t.Fatal("expected git-diff render output")
	}
	if gitDiff.StreamPreference != engine.StreamStdoutOnly || gitDiff.StreamRender == nil {
		t.Fatalf("unexpected git-diff stream metadata: %#v", gitDiff)
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
