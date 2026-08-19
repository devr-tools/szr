package rules_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/rules"
)

func TestParseJSONAndValidate(t *testing.T) {
	t.Parallel()

	file, err := rules.ParseJSON([]byte(`{
		"version": 1,
		"preferences": [
			{
				"name": "internal-cli-json",
				"description": "Normalizes generated tooling output",
				"match": {
					"command_prefix": ["internal-cli", "run"]
				},
				"rewrite": {
					"mode": "append",
					"placement": "before-terminator",
					"args": ["--format", "json"],
					"skip_if_has_any": ["--format"]
				}
			}
		],
		"profiles": [
			{
				"name": "pnpm-test",
				"description": "Local test profile",
				"explain": ["Appends a compact reporter."],
				"match": {
					"command_prefix": ["pnpm", "test"],
					"all_args": ["--watch=false"],
					"exclude_args": ["--json"]
				},
				"rewrite": {
					"mode": "append",
					"args": ["--reporter", "dot"],
					"skip_if_has_any": ["--reporter"]
				},
				"render": {
					"mode": "failure",
					"max_lines": 6
				}
			}
		]
	}`))
	if err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if len(file.Profiles) != 1 {
		t.Fatalf("expected one profile, got %#v", file)
	}
	if len(file.Preferences) != 1 || file.Preferences[0].Rewrite.Placement != "before-terminator" {
		t.Fatalf("unexpected parsed preferences: %#v", file.Preferences)
	}
	profile := file.Profiles[0]
	if profile.Name != "pnpm-test" || profile.Rewrite.Mode != "append" || profile.Render.Mode != "failure" {
		t.Fatalf("unexpected parsed profile: %#v", profile)
	}
}

func TestParseFileAndJSONErrors(t *testing.T) {
	t.Parallel()

	file, err := rules.ParseFile(".szr.yaml", []byte(`version: 1
preferences:
  - name: internal-cli-json
    match:
      command_prefix:
        - internal-cli
        - run
    rewrite:
      placement: before-terminator
      args:
        - --format
        - json
profiles:
  - name: pnpm-test
    explain:
      - Uses the repository-local reporter.
    match:
      command_prefix:
        - pnpm
        - test
      cwd_contains:
        - packages/web
    rewrite:
      mode: append
      args:
        - --reporter
        - dot
    render:
      mode: failure
      max_lines: 4
`))
	if err != nil {
		t.Fatalf("expected yaml parse success, got %v", err)
	}
	if len(file.Profiles) != 1 || file.Profiles[0].Match.CwdContains[0] != "packages/web" || file.Profiles[0].Render.MaxLines != 4 {
		t.Fatalf("unexpected yaml parse result: %#v", file)
	}
	if len(file.Preferences) != 1 || file.Preferences[0].Rewrite.Args[0] != "--format" {
		t.Fatalf("unexpected yaml preference parse result: %#v", file.Preferences)
	}

	_, err = rules.ParseFile(".szr.txt", []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "unsupported project rule file format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}

	if _, err := rules.ParseJSON([]byte("{bad")); err == nil {
		t.Fatal("expected invalid json parse error")
	}
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"empty profiles", `{"profiles":[],"preferences":[]}`, "at least one profile or preference"},
		{"duplicate profile", `{"profiles":[{"name":"dup","match":{"command_prefix":["npm"]}},{"name":"dup","match":{"command_prefix":["pnpm"]}}]}`, "duplicate profile name"},
		{"duplicate preference", `{"preferences":[{"name":"dup","match":{"command_prefix":["internal-cli"]},"rewrite":{"args":["--json"]}},{"name":"dup","match":{"command_prefix":["generated-cli"]},"rewrite":{"args":["--json"]}}]}`, "duplicate preference name"},
		{"missing matcher", `{"profiles":[{"name":"bad-match"}]}`, "match.command_prefix or match.display_prefix is required"},
		{"bad rewrite mode", `{"profiles":[{"name":"bad-rewrite","match":{"command_prefix":["npm"]},"rewrite":{"mode":"morph"}}]}`, "unsupported rewrite mode"},
		{"preference replace mode", `{"preferences":[{"name":"unsafe-replace","match":{"command_prefix":["git","status"]},"rewrite":{"mode":"replace","args":["sh","-c","echo unsafe"]}}]}`, "rewrite mode \"replace\" is not allowed"},
		{"bad rewrite placement", `{"preferences":[{"name":"bad-placement","match":{"command_prefix":["internal-cli"]},"rewrite":{"placement":"middle","args":["--json"]}}]}`, "unsupported rewrite placement"},
		{"bad render mode", `{"profiles":[{"name":"bad-render","match":{"command_prefix":["npm"]},"render":{"mode":"llm"}}]}`, "unsupported render mode"},
		{"bad version", `{"version":2,"profiles":[{"name":"bad-version","match":{"command_prefix":["npm"]}}]}`, "unsupported project rule version"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := rules.ParseJSON([]byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
		})
	}

	if _, err := rules.ParseJSON([]byte(`{"profiles":[{"name":"replace-profile","match":{"command_prefix":["go","test"]},"rewrite":{"mode":"replace","args":["go","test","-json"]}}]}`)); err != nil {
		t.Fatalf("expected profile replace mode to remain valid, got %v", err)
	}

	if err := rules.Validate(rules.File{Profiles: []rules.Profile{{
		Name:  "rewrite-missing-args",
		Match: rules.Match{CommandPrefix: []string{"npm"}},
		Rewrite: rules.Rewrite{
			Mode: "append",
		},
	}}}); err == nil || !strings.Contains(err.Error(), "rewrite.args is required") {
		t.Fatalf("expected rewrite args validation error, got %v", err)
	}

	if err := rules.Validate(rules.File{Profiles: []rules.Profile{{
		Name:  "negative-lines",
		Match: rules.Match{CommandPrefix: []string{"npm"}},
		Render: rules.Render{
			Mode:     "compact",
			MaxLines: -1,
		},
	}}}); err == nil || !strings.Contains(err.Error(), "render.max_lines must be >= 0") {
		t.Fatalf("expected negative max lines error, got %v", err)
	}

	if err := rules.Validate(rules.File{Preferences: []rules.Preference{{
		Name:  "missing-rewrite",
		Match: rules.Match{CommandPrefix: []string{"internal-cli"}},
	}}}); err == nil || !strings.Contains(err.Error(), "rewrite.args is required") {
		t.Fatalf("expected preference rewrite args error, got %v", err)
	}

	if _, err := rules.ParseFile(".szr.yaml", []byte("profiles:\n  not-a-list\n")); err == nil || !strings.Contains(err.Error(), "expected profile list item") {
		t.Fatalf("expected yaml structure error, got %v", err)
	}
}
