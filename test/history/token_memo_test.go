package history_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/history"
)

func TestTokenMemoMatchesEstimateTokens(t *testing.T) {
	t.Parallel()
	memo := history.TokenMemo{}
	samples := []string{
		"",
		"ok",
		"error: undefined symbol RenderWidget\nsrc/ui/widget.go:87",
		"3 failed, 24 passed",
	}
	for _, sample := range samples {
		want := history.EstimateTokens(sample)
		if got := memo.Estimate(sample); got != want {
			t.Fatalf("first estimate of %q = %d, want %d", sample, got, want)
		}
		// The memoized repeat must return the identical count.
		if got := memo.Estimate(sample); got != want {
			t.Fatalf("memoized estimate of %q = %d, want %d", sample, got, want)
		}
	}
}

func TestTokenMemoNilEstimatesDirectly(t *testing.T) {
	t.Parallel()
	var memo history.TokenMemo
	text := "a nil memo must estimate without caching"
	if got, want := memo.Estimate(text), history.EstimateTokens(text); got != want {
		t.Fatalf("nil memo estimate = %d, want %d", got, want)
	}
}

func TestTokenMemoCachesByContent(t *testing.T) {
	t.Parallel()
	memo := history.TokenMemo{}
	first := "identical content tail"
	// Same content in a distinct backing array must share one cache entry.
	second := strings.Join([]string{"identical", "content", "tail"}, " ")
	if memo.Estimate(first) != memo.Estimate(second) {
		t.Fatal("expected equal estimates for equal content")
	}
	if len(memo) != 1 {
		t.Fatalf("expected one cache entry for equal content, got %d", len(memo))
	}
}
