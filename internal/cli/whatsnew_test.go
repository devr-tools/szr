package cli

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const sampleChangelog = `# Changelog

## [0.9.0](https://github.com/devr-tools/szr/compare/v0.8.0...v0.9.0) (2026-07-02)


### Features

* **filters:** add declarative dedup/fold primitives ([bafe5a9](https://github.com/devr-tools/szr/commit/bafe5a9))
* raise token savings with broader profile coverage ([9fc299e](https://github.com/devr-tools/szr/commit/9fc299e))
* raise token savings with broader profile coverage ([1848079](https://github.com/devr-tools/szr/commit/1848079))
* summarize kubectl mutations and docker transfers ([53ae21c](https://github.com/devr-tools/szr/commit/53ae21c))


### Bug Fixes

* **engine:** treat grep no-match exits as benign ([21f0c80](https://github.com/devr-tools/szr/commit/21f0c80)), closes [#12](https://github.com/devr-tools/szr/issues/12)

## [0.8.0](https://github.com/devr-tools/szr/compare/v0.7.0...v0.8.0) (2026-06-25)


### Features

* old release bullet that must not appear ([f5708e4](https://github.com/devr-tools/szr/commit/f5708e4))
`

func TestParseLatestWhatsNew(t *testing.T) {
	rel := parseLatestWhatsNew(sampleChangelog)
	if rel == nil {
		t.Fatal("expected a release from sample changelog")
	}
	if rel.version != "0.9.0" {
		t.Fatalf("version = %q, want 0.9.0", rel.version)
	}
	want := []string{
		"Filters: add declarative dedup/fold primitives",
		"Raise token savings with broader profile coverage",
		"Summarize kubectl mutations and docker transfers",
	}
	if len(rel.bullets) != len(want) {
		t.Fatalf("bullets = %#v, want %#v", rel.bullets, want)
	}
	for i, bullet := range rel.bullets {
		if bullet != want[i] {
			t.Fatalf("bullet %d = %q, want %q", i, bullet, want[i])
		}
	}
}

func TestParseLatestWhatsNewFixesFillRemainingSlots(t *testing.T) {
	changelog := `## [1.2.3](url) (2026-01-01)

### Features

* only feature ([abc1234](url))

### Bug Fixes

* **engine:** treat grep no-match exits as benign ([21f0c80](url)), closes [#12](url)
`
	rel := parseLatestWhatsNew(changelog)
	if rel == nil {
		t.Fatal("expected a release")
	}
	want := []string{
		"Only feature",
		"Engine: treat grep no-match exits as benign",
	}
	if len(rel.bullets) != len(want) || rel.bullets[0] != want[0] || rel.bullets[1] != want[1] {
		t.Fatalf("bullets = %#v, want %#v", rel.bullets, want)
	}
}

func TestParseLatestWhatsNewEmptyOrHeaderless(t *testing.T) {
	if rel := parseLatestWhatsNew(""); rel != nil {
		t.Fatalf("expected nil for empty changelog, got %#v", rel)
	}
	if rel := parseLatestWhatsNew("# Changelog\n\nno releases yet\n"); rel != nil {
		t.Fatalf("expected nil for changelog without releases, got %#v", rel)
	}
	if rel := parseLatestWhatsNew("## [1.0.0](url) (2026-01-01)\n"); rel != nil {
		t.Fatalf("expected nil for release without bullets, got %#v", rel)
	}
}

func TestCleanWhatsNewBulletTruncates(t *testing.T) {
	long := strings.Repeat("a", whatsNewMaxBulletWidth+20)
	got := cleanWhatsNewBullet(long)
	if utf8.RuneCountInString(got) != whatsNewMaxBulletWidth {
		t.Fatalf("truncated width = %d, want %d", utf8.RuneCountInString(got), whatsNewMaxBulletWidth)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestLatestWhatsNewFromEmbeddedChangelog(t *testing.T) {
	rel := latestWhatsNew()
	if rel == nil {
		t.Fatal("expected the embedded CHANGELOG.md to yield a release")
	}
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(rel.version) {
		t.Fatalf("version %q does not look like semver; changelog format may have drifted", rel.version)
	}
	if len(rel.bullets) == 0 || len(rel.bullets) > whatsNewMaxBullets {
		t.Fatalf("expected 1..%d bullets, got %#v", whatsNewMaxBullets, rel.bullets)
	}
}

func TestFormatMenuVersion(t *testing.T) {
	cases := map[string]string{
		"0.9.0":   "v0.9.0",
		"v0.9.0":  "v0.9.0",
		"dev":     "dev",
		"test":    "test",
		"":        "",
		" 1.2.3 ": "v1.2.3",
	}
	for input, want := range cases {
		if got := formatMenuVersion(input); got != want {
			t.Fatalf("formatMenuVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestWhatsNewLinesNilWhenEmpty(t *testing.T) {
	if lines := whatsNewLines(nil); lines != nil {
		t.Fatalf("expected nil for nil release, got %v", lines)
	}
	if lines := whatsNewLines(&whatsNewRelease{version: "1.0.0"}); lines != nil {
		t.Fatalf("expected nil for release without bullets, got %v", lines)
	}
}

func TestWhatsNewLinesStructureAndAlignment(t *testing.T) {
	rel := &whatsNewRelease{
		version: "0.7.0",
		bullets: []string{"short", "a noticeably longer highlight line"},
	}
	lines := whatsNewLines(rel)

	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[1], "What's New in v0.7.0") {
		t.Fatalf("title line missing version: %q", lines[1])
	}

	width := utf8.RuneCountInString(lines[0])
	for i, line := range lines {
		if got := utf8.RuneCountInString(line); got != width {
			t.Fatalf("line %d width %d, want %d: %q", i, got, width, line)
		}
	}

	if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
		t.Fatalf("unexpected top border: %q", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Fatalf("unexpected bottom border: %q", lines[len(lines)-1])
	}
}
