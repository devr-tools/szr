package rules_test

import (
	"strings"
	"testing"

	"szr/internal/rules"
)

func TestParseJSONAndValidate(t *testing.T) {
	file, err := rules.ParseJSON([]byte(`{
		"version": 1,
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
	profile := file.Profiles[0]
	if profile.Name != "pnpm-test" || profile.Rewrite.Mode != "append" || profile.Render.Mode != "failure" {
		t.Fatalf("unexpected parsed profile: %#v", profile)
	}
}

func TestParseFileAndJSONErrors(t *testing.T) {
	_, err := rules.ParseFile(".szr.yaml", []byte("profiles: []\n"))
	if err == nil || !strings.Contains(err.Error(), "yaml project rules are not supported") {
		t.Fatalf("expected yaml unsupported error, got %v", err)
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
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"empty profiles", `{"profiles":[]}`, "at least one profile"},
		{"duplicate profile", `{"profiles":[{"name":"dup","match":{"command_prefix":["npm"]}},{"name":"dup","match":{"command_prefix":["pnpm"]}}]}`, "duplicate profile name"},
		{"missing matcher", `{"profiles":[{"name":"bad-match"}]}`, "match.command_prefix or match.display_prefix is required"},
		{"bad rewrite mode", `{"profiles":[{"name":"bad-rewrite","match":{"command_prefix":["npm"]},"rewrite":{"mode":"morph"}}]}`, "unsupported rewrite mode"},
		{"bad render mode", `{"profiles":[{"name":"bad-render","match":{"command_prefix":["npm"]},"render":{"mode":"llm"}}]}`, "unsupported render mode"},
		{"bad version", `{"version":2,"profiles":[{"name":"bad-version","match":{"command_prefix":["npm"]}}]}`, "unsupported project rule version"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rules.ParseJSON([]byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
		})
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
}
