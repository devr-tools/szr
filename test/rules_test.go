package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/rules"
)

func TestRuleParseAndValidate(t *testing.T) {
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

func TestRuleParseFileErrors(t *testing.T) {
	_, err := rules.ParseFile(".szr.yaml", []byte("profiles: []\n"))
	if err == nil || !strings.Contains(err.Error(), "yaml project rules are not supported") {
		t.Fatalf("expected yaml unsupported error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"profiles":[]}`))
	if err == nil || !strings.Contains(err.Error(), "at least one profile") {
		t.Fatalf("expected empty profile error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"profiles":[{"name":"dup","match":{"command_prefix":["npm"]}},{"name":"dup","match":{"command_prefix":["pnpm"]}}]}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate profile name") {
		t.Fatalf("expected duplicate name error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"profiles":[{"name":"bad-match"}]}`))
	if err == nil || !strings.Contains(err.Error(), "match.command_prefix or match.display_prefix is required") {
		t.Fatalf("expected missing matcher error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"profiles":[{"name":"bad-rewrite","match":{"command_prefix":["npm"]},"rewrite":{"mode":"morph"}}]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported rewrite mode") {
		t.Fatalf("expected rewrite mode error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"profiles":[{"name":"bad-render","match":{"command_prefix":["npm"]},"render":{"mode":"llm"}}]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported render mode") {
		t.Fatalf("expected render mode error, got %v", err)
	}

	_, err = rules.ParseJSON([]byte(`{"version":2,"profiles":[{"name":"bad-version","match":{"command_prefix":["npm"]}}]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported project rule version") {
		t.Fatalf("expected version error, got %v", err)
	}
}

func TestRuleDiscovery(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "repo")
	child := filepath.Join(projectRoot, "src", "pkg")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	ruleFile := filepath.Join(projectRoot, ".szr.json")
	if err := os.WriteFile(ruleFile, []byte(`{"profiles":[{"name":"one","match":{"command_prefix":["npm"]}}]}`), 0o644); err != nil {
		t.Fatalf("write json rule file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".szr.yaml"), []byte("profiles: []\n"), 0o644); err != nil {
		t.Fatalf("write yaml rule file: %v", err)
	}

	path, format, err := rules.DiscoverWith(child, os.Stat)
	if err != nil {
		t.Fatalf("discover with json present: %v", err)
	}
	if path != ruleFile || format != rules.FormatJSON {
		t.Fatalf("unexpected discovery result path=%q format=%q", path, format)
	}

	if err := os.Remove(ruleFile); err != nil {
		t.Fatalf("remove json rule file: %v", err)
	}
	path, format, err = rules.DiscoverWith(child, os.Stat)
	if err != nil {
		t.Fatalf("discover with yaml present: %v", err)
	}
	if !strings.HasSuffix(path, ".szr.yaml") || format != rules.FormatYAML {
		t.Fatalf("unexpected yaml discovery result path=%q format=%q", path, format)
	}

	if err := os.Remove(filepath.Join(projectRoot, ".szr.yaml")); err != nil {
		t.Fatalf("remove yaml rule file: %v", err)
	}
	path, format, err = rules.DiscoverWith(child, os.Stat)
	if err != nil {
		t.Fatalf("discover empty tree: %v", err)
	}
	if path != "" || format != rules.FormatUnknown {
		t.Fatalf("expected no rule file, got path=%q format=%q", path, format)
	}
}
