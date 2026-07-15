package profiles_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	gradleprofiles "github.com/devr-tools/szr/internal/profiles/gradle"
	"github.com/devr-tools/szr/test/testutil"
)

func TestGradleBuildProfileMatch(t *testing.T) {
	profile := testutil.FindProfile(t, gradleprofiles.Profiles(6), "gradle-build")

	for _, display := range [][]string{
		{"gradle", "build"},
		{"gradle", "test"},
		{"gradle", "check"},
		{"gradle", "clean", "build"},
		{"gradle", ":app:test"},
		{"./gradlew", "build"},
		{"./gradlew", "--console=plain", "test"},
		{"gradlew", "check"},
		{"tools/gradlew", "build"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match gradle-build", display)
		}
	}
	for _, display := range [][]string{
		{"gradle", "tasks"},
		{"gradle"},
		{"./gradlew", "assemble"},
		{"make", "build"},
	} {
		if profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match gradle-build", display)
		}
	}
}

func TestGradleBuildProfilePrepare(t *testing.T) {
	profile := testutil.FindProfile(t, gradleprofiles.Profiles(6), "gradle-build")
	advanced := config.Default().Advanced

	got := profile.Prepare(engine.Invocation{Command: []string{"./gradlew", "build"}, Advanced: advanced})
	if !reflect.DeepEqual(got, []string{"./gradlew", "build", "--console=plain"}) {
		t.Fatalf("unexpected gradle prepare: %#v", got)
	}
	got = profile.Prepare(engine.Invocation{Command: []string{"gradle", "test", "--console=rich"}, Advanced: advanced})
	if !reflect.DeepEqual(got, []string{"gradle", "test", "--console=rich"}) {
		t.Fatalf("expected console flag to be preserved, got %#v", got)
	}
	got = profile.Prepare(engine.Invocation{Command: []string{"gradle", "build"}})
	if !reflect.DeepEqual(got, []string{"gradle", "build"}) {
		t.Fatalf("expected non-aggressive prepare passthrough, got %#v", got)
	}
}

func TestGradleBuildProfileRenderAndStream(t *testing.T) {
	profile := testutil.FindProfile(t, gradleprofiles.Profiles(6), "gradle-build")

	failure := "> Task :app:compileJava FAILED\n\n/src/App.java:12: error: cannot find symbol\nBUILD FAILED in 9s\n2 actionable tasks: 2 executed\n"
	rendered := profile.Render(engine.Invocation{}, engine.Execution{Stdout: failure, ExitCode: 1})
	for _, want := range []string{"> Task :app:compileJava FAILED", "/src/App.java:12: error: cannot find symbol", "BUILD FAILED in 9s"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in gradle render:\n%s", want, rendered)
		}
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected gradle-build stream metadata: %#v", profile)
	}
	stream := profile.StreamRender(engine.Invocation{}, profile.Budget)
	stream.ConsumeStdout([]byte("> Task :app:test\nBUILD SUCCESSFUL in 4s\n5 actionable tasks: 5 executed\n"))
	got := stream.Result()
	for _, want := range []string{"BUILD SUCCESSFUL in 4s", "5 actionable tasks: 5 executed", "tasks: 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gradle streamed output:\n%s", want, got)
		}
	}
}

func TestGradleBuildRoutesAheadOfBuildSystem(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	e := engine.New(cfg, paths, nil, profiles.Builtins(cfg.MaxPreviewLines))

	for display, want := range map[string]string{
		"gradle build":   "gradle-build",
		"./gradlew test": "gradle-build",
		"gradle tasks":   "build-system",
	} {
		inv := engine.Invocation{Command: strings.Fields(display), Display: strings.Fields(display)}
		if got := e.Explain(inv).Name; got != want {
			t.Fatalf("expected %q to route to %q, got %q", display, want, got)
		}
	}
}
