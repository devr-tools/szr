package engine_test

import (
	"strings"
	"testing"

	"szr/internal/engine"
)

func TestHelpers(t *testing.T) {
	cases := []struct {
		stdout string
		stderr string
		want   string
	}{
		{"", "", ""},
		{"out", "", "out"},
		{"", "err", "err"},
		{"out", "err", "out\nerr"},
	}
	for _, tc := range cases {
		if got := engine.CombineStreams(tc.stdout, tc.stderr); got != tc.want {
			t.Fatalf("combine streams mismatch: got %q want %q", got, tc.want)
		}
	}

	if got := engine.SanitizeFileName("***"); got != "output" {
		t.Fatalf("unexpected empty sanitize fallback: %q", got)
	}
	if got := engine.SanitizeFileName("abc-123"); got != "abc_123" {
		t.Fatalf("unexpected sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName("AbC9"); got != "AbC9" {
		t.Fatalf("unexpected uppercase sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName(strings.Repeat("x", 60)); len(got) != 48 {
		t.Fatalf("expected truncated sanitize result, got %d chars", len(got))
	}
}
