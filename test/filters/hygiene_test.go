package filters_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/declarative"
)

func TestReadMinimalBuiltinHeadBudget(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("line-%02d", i))
	}
	result, err := declarative.ApplyBuiltin("read_minimal", strings.Join(lines, "\n"), declarative.Options{})
	if err != nil {
		t.Fatalf("run read_minimal: %v", err)
	}
	if result.VisibleLines != 40 || result.OmittedAfter != 10 {
		t.Fatalf("expected read_minimal head budget of 40 (visible=%d omitted=%d)", result.VisibleLines, result.OmittedAfter)
	}
	if !strings.Contains(result.Text, "line-39") || strings.Contains(result.Text, "line-40") {
		t.Fatalf("expected exactly 40 leading lines kept:\n%s", result.Text)
	}
	if !strings.HasSuffix(result.Text, "... +10 more lines") {
		t.Fatalf("expected overflow marker, got:\n%s", result.Text)
	}
}

func TestLogLevelTokensRequireWordBoundaries(t *testing.T) {
	t.Parallel()

	// "information ..." must not be dropped as info-level boilerplate and
	// "no errors ..." must not be promoted as an error line.
	reduced := filters.DedupeLines(strings.Join([]string{
		"information for operators",
		"no errors were found in the scan",
		"downloading cache layer",
		"error: real failure",
	}, "\n"), 2)
	if !strings.Contains(reduced, "error: real failure") {
		t.Fatalf("expected true error line to survive, got %q", reduced)
	}
	if !strings.Contains(reduced, "information for operators") {
		t.Fatalf("expected non-boilerplate line to outrank boilerplate, got %q", reduced)
	}
	if strings.Contains(reduced, "no errors were found") {
		t.Fatalf("expected %q to be treated as a plain line, got %q", "no errors", reduced)
	}
	if strings.Contains(reduced, "downloading cache layer") {
		t.Fatalf("expected boilerplate download line to be dropped, got %q", reduced)
	}

	// Bracketed and suffixed level tokens still classify.
	levelled := filters.DedupeLines(strings.Join([]string{
		"[INFO] warming cache",
		"TypeError: cannot read properties",
		"plain progress-free line",
	}, "\n"), 2)
	if !strings.Contains(levelled, "TypeError: cannot read properties") || strings.Contains(levelled, "warming cache") {
		t.Fatalf("expected token-based level classification, got %q", levelled)
	}
}
