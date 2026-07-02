package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestGitOpsProfilesMatch(t *testing.T) {
	list := profiles.Builtins(3)
	for _, tc := range []struct {
		profile string
		command []string
	}{
		{profile: "git-fetch", command: []string{"git", "fetch", "origin"}},
		{profile: "git-clone", command: []string{"git", "clone", "https://github.com/devr-tools/szr.git"}},
		{profile: "git-merge", command: []string{"git", "merge", "feature"}},
		{profile: "git-rebase", command: []string{"git", "rebase", "main"}},
		{profile: "git-checkout", command: []string{"git", "checkout", "-b", "feature"}},
		{profile: "git-checkout", command: []string{"git", "switch", "main"}},
		{profile: "git-reset", command: []string{"git", "reset", "--hard", "HEAD~1"}},
		{profile: "git-stash", command: []string{"git", "stash", "pop"}},
		{profile: "git-branch", command: []string{"git", "branch", "-a"}},
		{profile: "git-cherry-pick", command: []string{"git", "cherry-pick", "abc1234"}},
	} {
		profile := testutil.FindProfile(t, list, tc.profile)
		if !profile.Match(engine.Classify(engine.Invocation{Display: tc.command})) {
			t.Fatalf("%s should match %v", tc.profile, tc.command)
		}
	}

	fetch := testutil.FindProfile(t, list, "git-fetch")
	if fetch.Match(engine.Classify(engine.Invocation{Display: []string{"git", "status"}})) {
		t.Fatal("git-fetch should not match git status")
	}
	status := testutil.FindProfile(t, list, "git-status")
	if status.Match(engine.Classify(engine.Invocation{Display: []string{"git", "fetch"}})) {
		t.Fatal("git-status should not match git fetch")
	}
}

func TestGitFetchProfileRender(t *testing.T) {
	list := profiles.Builtins(3)
	fetch := testutil.FindProfile(t, list, "git-fetch")
	if fetch.StreamPreference != engine.StreamStderrFirst || fetch.StreamRender == nil {
		t.Fatalf("unexpected git-fetch stream metadata: %#v", fetch)
	}
	exec := engine.Execution{Stderr: strings.Join([]string{
		"remote: Enumerating objects: 120, done.",
		"remote: Counting objects: 100% (120/120), done.",
		"Receiving objects: 100% (120/120), 250.00 KiB | 2.00 MiB/s, done.",
		"Resolving deltas: 100% (45/45), done.",
		"From github.com:devr-tools/szr",
		"   dae313d..d3d7424  main -> origin/main",
	}, "\n")}
	got := fetch.Render(engine.Invocation{}, exec)
	if !strings.Contains(got, "objects: 120 (250.00 KiB), deltas: 45") || !strings.Contains(got, "main -> origin/main") {
		t.Fatalf("unexpected git-fetch render output: %q", got)
	}
	if strings.Contains(got, "remote:") || strings.Contains(got, "Receiving objects") {
		t.Fatalf("expected git-fetch render to drop progress noise: %q", got)
	}

	stream := fetch.StreamRender(engine.Invocation{}, fetch.Budget)
	stream.ConsumeStderr([]byte("remote: Counting objects: 100% (10/10), done.\nFrom github.com:devr-tools/szr\n   aaa1111..bbb2222  main -> origin/main\n"))
	if got := stream.Result(); !strings.Contains(got, "main -> origin/main") || strings.Contains(got, "Counting objects") {
		t.Fatalf("unexpected git-fetch stream output: %q", got)
	}
}

func TestGitMergeProfileKeepsConflicts(t *testing.T) {
	list := profiles.Builtins(3)
	merge := testutil.FindProfile(t, list, "git-merge")
	exec := engine.Execution{Stdout: strings.Join([]string{
		"Auto-merging internal/app.go",
		"CONFLICT (content): Merge conflict in internal/app.go",
		"Auto-merging cmd/main.go",
		"CONFLICT (content): Merge conflict in cmd/main.go",
		"Automatic merge failed; fix conflicts and then commit the result.",
	}, "\n"), ExitCode: 1}
	got := merge.Render(engine.Invocation{}, exec)
	for _, want := range []string{
		"CONFLICT (content): Merge conflict in internal/app.go",
		"CONFLICT (content): Merge conflict in cmd/main.go",
		"Automatic merge failed; fix conflicts and then commit the result.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in merge render output %q", want, got)
		}
	}
}

func TestGoModProfile(t *testing.T) {
	list := profiles.Builtins(3)
	goMod := testutil.FindProfile(t, list, "go-mod")
	for _, command := range [][]string{
		{"go", "mod", "tidy"},
		{"go", "mod", "download"},
		{"go", "get", "golang.org/x/sys@latest"},
		{"go", "install", "github.com/devr-tools/szr/cmd/szr@latest"},
	} {
		if !goMod.Match(engine.Invocation{Command: command}) {
			t.Fatalf("go-mod should match %v", command)
		}
	}
	if goMod.Match(engine.Invocation{Command: []string{"go", "test", "./..."}}) || goMod.Match(engine.Invocation{Command: []string{"go", "build", "./..."}}) {
		t.Fatal("go-mod should not shadow go test or go build")
	}

	downloads := strings.Join([]string{
		"go: downloading golang.org/x/sys v0.30.0",
		"go: downloading golang.org/x/text v0.22.0",
		"go: downloading github.com/spf13/cobra v1.9.1",
		"go: downloading github.com/spf13/pflag v1.0.6",
		"go: downloading gopkg.in/yaml.v3 v3.0.1",
		"go: added github.com/spf13/cobra v1.9.1",
		"go: upgraded golang.org/x/sys v0.29.0 => v0.30.0",
	}, "\n")
	got := goMod.Render(engine.Invocation{}, engine.Execution{Stderr: downloads})
	if !strings.Contains(got, "downloaded 5 modules (top: golang.org/x/sys, golang.org/x/text, github.com/spf13/cobra, ...)") {
		t.Fatalf("expected collapsed download summary, got %q", got)
	}
	if !strings.Contains(got, "go: added github.com/spf13/cobra v1.9.1") || !strings.Contains(got, "go: upgraded golang.org/x/sys v0.29.0 => v0.30.0") {
		t.Fatalf("expected dependency change lines to be kept, got %q", got)
	}
	if strings.Contains(got, "go: downloading") {
		t.Fatalf("expected raw download lines to be dropped, got %q", got)
	}

	failure := "go: downloading github.com/missing/module v1.0.0\ngo: github.com/missing/module@v1.0.0: verifying module: checksum mismatch\n"
	if got := goMod.Render(engine.Invocation{}, engine.Execution{Stderr: failure, ExitCode: 1}); !strings.Contains(got, "checksum mismatch") {
		t.Fatalf("expected error lines to pass through, got %q", got)
	}

	if got := goMod.Render(engine.Invocation{}, engine.Execution{}); got != "ok" {
		t.Fatalf("expected quiet go mod run to render ok, got %q", got)
	}

	stream := goMod.StreamRender(engine.Invocation{}, goMod.Budget)
	stream.ConsumeStderr([]byte("go: downloading a.example/one v1.0.0\ngo: downloading a.example/two v1.0.0\n"))
	if got := stream.Result(); !strings.Contains(got, "downloaded 2 modules") {
		t.Fatalf("unexpected go-mod stream output: %q", got)
	}
}
