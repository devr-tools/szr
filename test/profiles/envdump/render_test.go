package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

// TestEnvPrintProfile pins the env-print contract: only a bare `env` is
// claimed (assignments and wrapped commands stay untouched), diagnostic
// variables stay readable, prefixes group, and secret-looking values redact.
func TestEnvPrintProfile(t *testing.T) {
	list := profiles.Builtins(6)
	envPrint := testutil.FindProfile(t, list, "env-print")

	if !envPrint.Match(engine.Invocation{Display: []string{"env"}}) {
		t.Fatal("expected env-print to match bare env")
	}
	for _, display := range [][]string{
		{"env", "KEY=value", "go", "test"},
		{"env", "-u", "GOROOT", "go", "build"},
		{"env", "-0"},
		{"printenv"},
	} {
		if envPrint.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected env-print not to match %v", display)
		}
	}

	rendered := envPrint.Render(engine.Invocation{}, engine.Execution{Stdout: strings.Join([]string{
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/tools/bin",
		"HOME=/Users/devbot",
		"SHELL=/bin/zsh",
		"API_TOKEN=abcd1234efgh",
		"GIT_EDITOR=vim",
		"GIT_PAGER=delta",
		"SESSION_BLOB=" + strings.Repeat("x", 64),
	}, "\n")})
	for _, want := range []string{
		"env: 7 vars",
		"PATH: 6 entries: /usr/local/bin /usr/bin /bin /usr/sbin (+2 more)",
		"HOME=/Users/devbot SHELL=/bin/zsh",
		"API_TOKEN=<redacted len=12>",
		"GIT_* (2): GIT_EDITOR=vim GIT_PAGER=delta",
		"SESSION_BLOB=<len 64>",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in env render:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "abcd1234efgh") {
		t.Fatalf("expected secret value to be redacted:\n%s", rendered)
	}
}

func TestEnvPrintStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)
	envPrint := testutil.FindProfile(t, list, "env-print")

	stream := envPrint.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"A_ONE=1", "A_TWO=2", "B_ONE=1", "B_TWO=2", "C_ONE=1", "C_TWO=2",
	}, "\n")))
	recovery, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable env reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected env recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
