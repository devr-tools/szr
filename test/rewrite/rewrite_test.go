package rewrite_test

import (
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
		decision := rewrite.Analyze("/usr/bin/grep -rn needle .", "szr")
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
}

func TestFamily(t *testing.T) {
	cases := map[string]string{
		"git diff HEAD~1..HEAD --stat":        "git diff",
		"/usr/bin/git status --short":         "git status",
		"/usr/bin/find /repo -name users.py":  "find",
		"/usr/bin/grep -rn needle /repo":      "grep",
		"python -m pytest tests/unit":         "python",
		"git diff HEAD~1..HEAD --stat | tail": "git diff",
	}
	for command, want := range cases {
		if got := rewrite.Family(command); got != want {
			t.Fatalf("family(%q) = %q, want %q", command, got, want)
		}
	}
}
