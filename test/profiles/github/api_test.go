package profiles

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
)

func findGHAPIProfile(t *testing.T) engine.Profile {
	t.Helper()
	for _, profile := range profiles.Builtins(12) {
		if profile.Name == "gh-api" {
			return profile
		}
	}
	t.Fatal("gh-api profile not registered")
	return engine.Profile{}
}

func TestGHAPIProfileMatch(t *testing.T) {
	profile := findGHAPIProfile(t)
	cases := []struct {
		name  string
		args  []string
		match bool
	}{
		{"plain endpoint", []string{"gh", "api", "repos/devr-tools/szr"}, true},
		{"repo flag before subcommand", []string{"gh", "-R", "devr-tools/szr", "api", "user"}, true},
		{"paginated", []string{"gh", "api", "--paginate", "repos/o/r/issues"}, true},
		{"jq filtered", []string{"gh", "api", "repos/o/r", "--jq", ".name"}, false},
		{"jq shorthand", []string{"gh", "api", "repos/o/r", "-q", ".name"}, false},
		{"jq inline", []string{"gh", "api", "repos/o/r", "--jq=.name"}, false},
		{"template", []string{"gh", "api", "repos/o/r", "--template", "{{.name}}"}, false},
		{"silent", []string{"gh", "api", "repos/o/r", "--silent"}, false},
		{"pr checks unaffected", []string{"gh", "pr", "checks"}, false},
		{"bare gh api no endpoint", []string{"gh", "api"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := engine.Classify(engine.Invocation{Command: tc.args, Display: tc.args})
			if got := profile.Match(inv); got != tc.match {
				t.Fatalf("Match(%v) = %v, want %v", tc.args, got, tc.match)
			}
		})
	}
}

func TestGHAPIProfileRenderJSON(t *testing.T) {
	profile := findGHAPIProfile(t)
	stdout := `{"name":"szr","full_name":"devr-tools/szr","private":false,"description":"token-efficient command output",` +
		`"topics":["cli","llm","tokens"],"stargazers_count":42,"open_issues_count":7,` +
		`"owner":{"login":"devr-tools","id":123,"type":"Organization"},` +
		`"license":{"key":"apache-2.0","name":"Apache License 2.0"}}`
	inv := engine.Classify(engine.Invocation{Command: []string{"gh", "api", "repos/devr-tools/szr"}, Display: []string{"gh", "api", "repos/devr-tools/szr"}})
	rendered := profile.Render(inv, engine.Execution{Stdout: stdout})
	if strings.TrimSpace(rendered) == "" {
		t.Fatal("expected non-empty rendered output")
	}
	if rendered == stdout {
		t.Fatal("expected summarized output, got raw JSON passthrough")
	}
	if !strings.Contains(rendered, "name") {
		t.Fatalf("expected summary to surface keys, got:\n%s", rendered)
	}
	if len(rendered) >= len(stdout) {
		t.Fatalf("expected summary shorter than raw (%d >= %d)", len(rendered), len(stdout))
	}
}

func TestGHAPIProfileRenderNonJSONFallback(t *testing.T) {
	profile := findGHAPIProfile(t)
	inv := engine.Classify(engine.Invocation{Command: []string{"gh", "api", "repos/o/r/readme"}, Display: []string{"gh", "api", "repos/o/r/readme"}})
	rendered := profile.Render(inv, engine.Execution{Stdout: "plain text body\nsecond line"})
	if !strings.Contains(rendered, "plain text body") {
		t.Fatalf("expected fallback to keep plain lines, got:\n%s", rendered)
	}
}

func TestGHAPIProfileStreamRender(t *testing.T) {
	profile := findGHAPIProfile(t)
	inv := engine.Classify(engine.Invocation{Command: []string{"gh", "api", "user"}, Display: []string{"gh", "api", "user"}})
	reducer := profile.StreamRender(inv, engine.OutputBudget{MaxLines: 10, MaxBytes: 1600, MaxTokens: 320})
	if reducer == nil {
		t.Fatal("expected stream reducer")
	}
	reducer.ConsumeStdout([]byte(`{"login":"alex","id":9,"plan":{"name":"pro","space":100}}`))
	result := reducer.Result()
	if strings.TrimSpace(result) == "" {
		t.Fatal("expected non-empty stream result")
	}
	if !strings.Contains(result, "login") {
		t.Fatalf("expected stream summary to surface keys, got:\n%s", result)
	}
}
