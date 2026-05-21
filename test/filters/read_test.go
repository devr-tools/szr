package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestReadLevels(t *testing.T) {
	readNone := filters.ReadLevel([]byte("a\n// comment\n# hash"), "none", false, 0)
	if readNone != "a\n// comment\n# hash" {
		t.Fatalf("unexpected read none: %q", readNone)
	}
	readMinimal := filters.ReadLevel([]byte("a\n// comment\n# hash"), "minimal", false, 0)
	if readMinimal != "a" {
		t.Fatalf("unexpected read minimal: %q", readMinimal)
	}
	readAggressive := filters.ReadLevel([]byte("func x() { return 1 }\n\n# c"), "aggressive", true, 1)
	if !strings.Contains(readAggressive, "func x() { ... }") || strings.Contains(readAggressive, "... +") {
		t.Fatalf("unexpected read aggressive: %q", readAggressive)
	}
	readAggressive = filters.ReadLevel([]byte("line1\nline2\nline3"), "aggressive", false, 2)
	if !strings.Contains(readAggressive, "... +1 more lines") {
		t.Fatalf("expected aggressive max-lines truncation: %q", readAggressive)
	}
}
