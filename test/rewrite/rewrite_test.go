package rewrite_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/rewrite"
)

func TestAnalyze(t *testing.T) {
	cases := []struct {
		name           string
		command        string
		autoRewrite    bool
		rewriteCommand string
		producerOnly   bool
		wrapMode       string
		checkHint      bool
		wantHint       bool
		hintContains   string
	}{
		{name: "git direct", command: "git diff HEAD~1..HEAD --stat", autoRewrite: true, rewriteCommand: "szr git diff HEAD~1..HEAD --stat"},
		{name: "git pipeline", command: "git diff HEAD~1..HEAD --stat | tail -30", autoRewrite: true, rewriteCommand: "szr proxy git diff HEAD~1..HEAD --stat | tail -30", producerOnly: true},
		{name: "grep hint only", command: "/usr/bin/grep -rn 'user.*id' .", checkHint: true, wantHint: true},
		{name: "git status hint", command: "/usr/bin/git status --short", autoRewrite: true, rewriteCommand: "szr /usr/bin/git status --short", checkHint: true, wantHint: true},
		{name: "grep structured direct", command: "/usr/bin/grep -rn needle .", autoRewrite: true, rewriteCommand: "szr grep needle .", wrapMode: "direct", checkHint: true, wantHint: true, hintContains: "szr grep <pattern> <path>"},
		{name: "grep pipeline remains hint only", command: "/usr/bin/grep -rn needle . | head -20", checkHint: true, wantHint: true},
		{name: "find structured direct", command: "/usr/bin/find /repo -name users.py -type f -maxdepth 2", autoRewrite: true, rewriteCommand: "szr find /repo --name users.py --type f --max-depth 2", wrapMode: "direct"},
		{name: "find unsupported flags stay hint only", command: "/usr/bin/find /repo -mtime -1", checkHint: true, wantHint: true},
		{name: "git ls-files direct", command: "git ls-files '*.go'", autoRewrite: true, rewriteCommand: "szr git ls-files '*.go'"},
		{name: "fd pipeline producer only", command: "fd needle src | head -20", autoRewrite: true, rewriteCommand: "szr proxy fd needle src | head -20", producerOnly: true},
		{name: "fd exec remains unsupported", command: "fd needle src -x rm {}", checkHint: true, wantHint: true},
		{name: "ls structured direct", command: "ls ./internal", autoRewrite: true, rewriteCommand: "szr ls ./internal"},
		{name: "tree rewrites to ls", command: "tree ./internal", autoRewrite: true, rewriteCommand: "szr ls ./internal"},
		{name: "ls flags remain unsupported", command: "ls -la", checkHint: true, wantHint: true},
		{name: "npx tsc direct", command: "npx tsc --noEmit", autoRewrite: true, rewriteCommand: "szr tsc --noEmit"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := rewrite.Analyze(tc.command, "szr")
			assertAnalyzeDecision(t, decision, tc)
		})
	}
}

func assertAnalyzeDecision(t *testing.T, decision rewrite.Decision, tc struct {
	name           string
	command        string
	autoRewrite    bool
	rewriteCommand string
	producerOnly   bool
	wrapMode       string
	checkHint      bool
	wantHint       bool
	hintContains   string
}) {
	t.Helper()
	if decision.AutoRewrite != tc.autoRewrite {
		t.Fatalf("unexpected auto rewrite for %s: %#v", tc.name, decision)
	}
	if decision.Rewrite != tc.rewriteCommand {
		t.Fatalf("unexpected rewrite for %s: %#v", tc.name, decision)
	}
	if decision.ProducerOnly != tc.producerOnly {
		t.Fatalf("unexpected producer-only flag for %s: %#v", tc.name, decision)
	}
	if tc.wrapMode != "" && decision.WrapMode != tc.wrapMode {
		t.Fatalf("unexpected wrap mode for %s: %#v", tc.name, decision)
	}
	if tc.checkHint && tc.wantHint != (decision.Hint != "") {
		t.Fatalf("unexpected hint presence for %s: %#v", tc.name, decision)
	}
	if tc.hintContains != "" && !strings.Contains(decision.Hint, tc.hintContains) {
		t.Fatalf("expected hint %q in %q", tc.hintContains, decision.Hint)
	}
}

func TestFamily(t *testing.T) {
	cases := map[string]string{
		"git diff HEAD~1..HEAD --stat":        "git diff",
		"/usr/bin/git status --short":         "git status",
		"git ls-files '*.go'":                 "git ls-files",
		"/usr/bin/find /repo -name users.py":  "find",
		"/usr/bin/grep -rn needle /repo":      "grep",
		"fd needle src":                       "fd",
		"ls ./internal":                       "ls",
		"tree ./internal":                     "tree",
		"npx tsc --noEmit":                    "tsc",
		"python -m pytest tests/unit":         "python",
		"git diff HEAD~1..HEAD --stat | tail": "git diff",
	}
	for command, want := range cases {
		if got := rewrite.Family(command); got != want {
			t.Fatalf("family(%q) = %q, want %q", command, got, want)
		}
	}
}
