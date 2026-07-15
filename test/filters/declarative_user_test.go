package filters_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters/declarative"
	"github.com/devr-tools/szr/test/testutil"
)

func TestLoadUserSpecsSkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(dir, "mytool-warnings.json"), `{
		"description": "Keeps warnings and errors from mytool runs.",
		"match": {"command_prefix": ["mytool"]},
		"keep_patterns": ["^(WARN|ERROR) "],
		"head": 4,
		"tail": 2
	}`)
	testutil.MustWriteFile(t, filepath.Join(dir, "broken.json"), "{not json")
	testutil.MustWriteFile(t, filepath.Join(dir, "no-match.json"), `{"name": "no-match", "head": 2}`)
	testutil.MustWriteFile(t, filepath.Join(dir, "bad-head.json"), `{"name": "bad-head", "head": -1, "match": {"command_prefix": ["x"]}}`)
	testutil.MustWriteFile(t, filepath.Join(dir, "bad-regex.json"), `{"name": "bad-regex", "keep_patterns": ["["], "match": {"command_prefix": ["x"]}}`)
	testutil.MustWriteFile(t, filepath.Join(dir, "notes.txt"), "not a spec")

	specs, warnings := declarative.LoadUserSpecs(dir)
	if len(specs) != 1 {
		t.Fatalf("expected 1 valid spec, got %#v", specs)
	}
	if specs[0].Name != "mytool-warnings" {
		t.Fatalf("expected name defaulted from filename, got %q", specs[0].Name)
	}
	if specs[0].Path != filepath.Join(dir, "mytool-warnings.json") {
		t.Fatalf("unexpected spec path: %q", specs[0].Path)
	}
	if len(specs[0].Match.CommandPrefix) != 1 || specs[0].Match.CommandPrefix[0] != "mytool" {
		t.Fatalf("unexpected match section: %#v", specs[0].Match)
	}
	if len(warnings) != 4 {
		t.Fatalf("expected 4 warnings, got %#v", warnings)
	}
	for _, file := range []string{"broken.json", "no-match.json", "bad-head.json", "bad-regex.json"} {
		if !containsWarning(warnings, file) {
			t.Fatalf("expected warning for %s, got %#v", file, warnings)
		}
	}
}

func TestLoadUserSpecsMissingDir(t *testing.T) {
	specs, warnings := declarative.LoadUserSpecs(filepath.Join(t.TempDir(), "missing"))
	if len(specs) != 0 || len(warnings) != 0 {
		t.Fatalf("expected empty result for missing dir, got %#v %#v", specs, warnings)
	}
}

func TestLoadUserSpecsAppliesLoadedSpec(t *testing.T) {
	dir := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(dir, "keep-errors.json"), `{
		"match": {"command_prefix": ["mytool"]},
		"keep_patterns": ["^ERROR "],
		"head": 3
	}`)
	specs, warnings := declarative.LoadUserSpecs(dir)
	if len(specs) != 1 || len(warnings) != 0 {
		t.Fatalf("unexpected load result: %#v %#v", specs, warnings)
	}
	result, err := declarative.Apply(specs[0].Spec, "info a\nERROR boom\ninfo b\n", declarative.Options{})
	if err != nil {
		t.Fatalf("apply loaded spec: %v", err)
	}
	if result.Text != "ERROR boom" {
		t.Fatalf("unexpected apply output: %q", result.Text)
	}
}

func TestHasUserSpecFiles(t *testing.T) {
	dir := t.TempDir()
	if declarative.HasUserSpecFiles(dir) {
		t.Fatal("empty dir should not report spec files")
	}
	if declarative.HasUserSpecFiles(filepath.Join(dir, "missing")) {
		t.Fatal("missing dir should not report spec files")
	}
	testutil.MustWriteFile(t, filepath.Join(dir, "notes.txt"), "not a spec")
	if declarative.HasUserSpecFiles(dir) {
		t.Fatal("non-json files should not report spec files")
	}
	testutil.MustWriteFile(t, filepath.Join(dir, "spec.json"), "{}")
	if !declarative.HasUserSpecFiles(dir) {
		t.Fatal("expected spec files to be reported")
	}
}

func containsWarning(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
