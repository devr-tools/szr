package rules_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/rules"
)

func TestDiscoverWithJSONYAMLAndEmptyTree(t *testing.T) {
	t.Parallel()

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

func TestDiscoverEdgeCases(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".szr.json"), []byte(`{"profiles":[{"name":"ok","match":{"command_prefix":["npm"]}}]}`), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	path, format, err := rules.Discover(root)
	if err != nil || format != rules.FormatJSON || !strings.HasSuffix(path, ".szr.json") {
		t.Fatalf("unexpected discover result path=%q format=%q err=%v", path, format, err)
	}

	if err := os.WriteFile(filepath.Join(root, ".szr.yml"), []byte("profiles: []\n"), 0o644); err != nil {
		t.Fatalf("write yml rule file: %v", err)
	}
	path, format, err = rules.DiscoverWith(root, os.Stat)
	if err != nil || format != rules.FormatJSON {
		t.Fatalf("unexpected discover precedence path=%q format=%q err=%v", path, format, err)
	}

	if err := os.Remove(filepath.Join(root, ".szr.json")); err != nil {
		t.Fatalf("remove json rule file: %v", err)
	}
	path, format, err = rules.DiscoverWith(root, os.Stat)
	if err != nil || format != rules.FormatYAML || !strings.HasSuffix(path, ".szr.yml") {
		t.Fatalf("unexpected yml discovery path=%q format=%q err=%v", path, format, err)
	}

	_, _, err = rules.DiscoverWith(root, func(string) (os.FileInfo, error) {
		return nil, errors.New("discover fail")
	})
	if err == nil || !strings.Contains(err.Error(), "discover fail") {
		t.Fatalf("expected discovery stat error, got %v", err)
	}
}
