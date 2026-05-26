package rewrite_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/rewrite"
)

func TestAnalyze(t *testing.T) {
	t.Run("git direct", func(t *testing.T) {
		decision := rewrite.Analyze("git diff HEAD~1..HEAD --stat", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr git diff HEAD~1..HEAD --stat" {
			t.Fatalf("unexpected git direct rewrite: %#v", decision)
		}
	})

	t.Run("git pipeline", func(t *testing.T) {
		decision := rewrite.Analyze("git diff HEAD~1..HEAD --stat | tail -30", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr proxy git diff HEAD~1..HEAD --stat | tail -30" || !decision.ProducerOnly {
			t.Fatalf("unexpected git pipeline rewrite: %#v", decision)
		}
	})

	t.Run("grep hint only", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/grep -rn 'user.*id' .", "szr")
		if decision.AutoRewrite || decision.Rewrite != "" || decision.Hint == "" {
			t.Fatalf("unexpected grep wrapper decision: %#v", decision)
		}
	})

	t.Run("git status hint", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/git status --short", "szr")
		if !decision.AutoRewrite || decision.Hint == "" || decision.Rewrite != "szr /usr/bin/git status --short" {
			t.Fatalf("unexpected git status decision: %#v", decision)
		}
	})

	t.Run("grep structured direct", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/grep -rn needle .", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr grep needle ." || decision.WrapMode != "direct" || !strings.Contains(decision.Hint, "szr grep <pattern> <path>") {
			t.Fatalf("unexpected grep structured rewrite: %#v", decision)
		}
	})

	t.Run("grep pipeline remains hint only", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/grep -rn needle . | head -20", "szr")
		if decision.AutoRewrite || decision.ProducerOnly || decision.Hint == "" {
			t.Fatalf("unexpected grep pipeline rewrite: %#v", decision)
		}
	})

	t.Run("find structured direct", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/find /repo -name users.py -type f -maxdepth 2", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr find /repo --name users.py --type f --max-depth 2" || decision.WrapMode != "direct" {
			t.Fatalf("unexpected find structured rewrite: %#v", decision)
		}
	})

	t.Run("find unsupported flags stay hint only", func(t *testing.T) {
		decision := rewrite.Analyze("/usr/bin/find /repo -mtime -1", "szr")
		if decision.AutoRewrite || decision.Rewrite != "" || decision.Hint == "" {
			t.Fatalf("unexpected find wrapper decision: %#v", decision)
		}
	})

	t.Run("git ls-files direct", func(t *testing.T) {
		decision := rewrite.Analyze("git ls-files '*.go'", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr git ls-files '*.go'" {
			t.Fatalf("unexpected git ls-files rewrite: %#v", decision)
		}
	})

	t.Run("fd pipeline producer only", func(t *testing.T) {
		decision := rewrite.Analyze("fd needle src | head -20", "szr")
		if !decision.AutoRewrite || !decision.ProducerOnly || decision.Rewrite != "szr proxy fd needle src | head -20" {
			t.Fatalf("unexpected fd pipeline rewrite: %#v", decision)
		}
	})

	t.Run("fd exec remains unsupported", func(t *testing.T) {
		decision := rewrite.Analyze("fd needle src -x rm {}", "szr")
		if decision.AutoRewrite {
			t.Fatalf("unexpected fd exec rewrite: %#v", decision)
		}
	})

	t.Run("ls structured direct", func(t *testing.T) {
		decision := rewrite.Analyze("ls ./internal", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr ls ./internal" {
			t.Fatalf("unexpected ls rewrite: %#v", decision)
		}
	})

	t.Run("tree rewrites to ls", func(t *testing.T) {
		decision := rewrite.Analyze("tree ./internal", "szr")
		if !decision.AutoRewrite || decision.Rewrite != "szr ls ./internal" {
			t.Fatalf("unexpected tree rewrite: %#v", decision)
		}
	})

	t.Run("ls flags remain unsupported", func(t *testing.T) {
		decision := rewrite.Analyze("ls -la", "szr")
		if decision.AutoRewrite || decision.Hint == "" {
			t.Fatalf("unexpected ls flag rewrite: %#v", decision)
		}
	})
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
		"python -m pytest tests/unit":         "python",
		"git diff HEAD~1..HEAD --stat | tail": "git diff",
	}
	for command, want := range cases {
		if got := rewrite.Family(command); got != want {
			t.Fatalf("family(%q) = %q, want %q", command, got, want)
		}
	}
}
