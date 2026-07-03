package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestDotnetProfiles(t *testing.T) {
	list := profiles.Builtins(3)

	dotnetTest := testutil.FindProfile(t, list, "dotnet-test")
	if !dotnetTest.Match(engine.Invocation{Display: []string{"dotnet", "test"}}) || !dotnetTest.Match(engine.Invocation{Command: []string{"dotnet", "test", "--filter", "InvoiceTests"}}) {
		t.Fatal("dotnet-test should match dotnet test")
	}
	if dotnetTest.Match(engine.Invocation{Display: []string{"dotnet", "build"}}) {
		t.Fatal("dotnet-test should not match dotnet build")
	}
	if got := dotnetTest.Prepare(engine.Invocation{Command: []string{"dotnet", "test"}}); len(got) != 3 || got[2] != "--nologo" {
		t.Fatalf("expected dotnet test prepare to add --nologo, got %#v", got)
	}
	if got := dotnetTest.Prepare(engine.Invocation{Command: []string{"dotnet", "test", "--nologo"}}); len(got) != 3 {
		t.Fatalf("expected dotnet test prepare to preserve --nologo, got %#v", got)
	}
	if got := dotnetTest.Prepare(engine.Invocation{Command: []string{"dotnet", "test", "--", "app-arg"}}); len(got) != 5 || got[2] != "--nologo" {
		t.Fatalf("expected --nologo before app-arg separator, got %#v", got)
	}
	if dotnetTest.StreamPreference != engine.StreamStdoutFirst || dotnetTest.StreamRender == nil {
		t.Fatalf("unexpected dotnet-test stream metadata: %#v", dotnetTest)
	}
	rendered := dotnetTest.Render(engine.Invocation{}, engine.Execution{Stdout: "  Failed Billing.Tests.InvoiceTests.Applies_Late_Fee [6 ms]\n  Error Message:\n   Assert.Equal() Failure: Values differ\nFailed!  - Failed:     1, Passed:     7, Skipped:     0, Total:     8, Duration: 74 ms - Billing.Tests.dll (net9.0)\n"})
	if !strings.Contains(rendered, "Failed Billing.Tests.InvoiceTests.Applies_Late_Fee") || !strings.Contains(rendered, "Assert.Equal() Failure") {
		t.Fatalf("expected dotnet-test render to keep failed test signal, got %q", rendered)
	}

	dotnetBuild := testutil.FindProfile(t, list, "dotnet-build")
	if !dotnetBuild.Match(engine.Invocation{Display: []string{"dotnet", "build"}}) ||
		!dotnetBuild.Match(engine.Invocation{Display: []string{"dotnet", "publish"}}) ||
		!dotnetBuild.Match(engine.Invocation{Display: []string{"msbuild", "Billing.sln"}}) {
		t.Fatal("dotnet-build should match dotnet build, publish, and msbuild")
	}
	if dotnetBuild.Match(engine.Invocation{Display: []string{"dotnet", "run"}}) {
		t.Fatal("dotnet-build should not match dotnet run")
	}
	if got := dotnetBuild.Prepare(engine.Invocation{Command: []string{"msbuild", "Billing.sln"}}); len(got) != 2 {
		t.Fatalf("expected msbuild prepare to stay untouched, got %#v", got)
	}
	if dotnetBuild.StreamPreference != engine.StreamStdoutFirst || dotnetBuild.StreamRender == nil {
		t.Fatalf("unexpected dotnet-build stream metadata: %#v", dotnetBuild)
	}
	rendered = dotnetBuild.Render(engine.Invocation{}, engine.Execution{Stdout: "/src/App.cs(5,1): error CS1002: ; expected [/src/App.csproj]\nBuild FAILED.\n"})
	if !strings.Contains(rendered, "error CS1002") || !strings.Contains(rendered, "Build FAILED.") {
		t.Fatalf("expected dotnet-build render to keep coded diagnostics, got %q", rendered)
	}

	stream := dotnetBuild.StreamRender(engine.Invocation{}, dotnetBuild.Budget)
	stream.ConsumeStdout([]byte("/src/App.cs(5,1): error CS1002: ; expected [/src/App.csproj]\n"))
	stream.ConsumeStderr([]byte("Build FAILED.\n"))
	if got := stream.Result(); !strings.Contains(got, "error CS1002") || !strings.Contains(got, "Build FAILED.") {
		t.Fatalf("unexpected dotnet-build stream output: %q", got)
	}
}
