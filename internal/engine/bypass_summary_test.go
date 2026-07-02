package engine

import "testing"

type bypassStubReducer struct {
	summary  string
	parsed   int
	fallback bool
}

func (r *bypassStubReducer) ConsumeStdout([]byte) {}

func (r *bypassStubReducer) ConsumeStderr([]byte) {}

func (r *bypassStubReducer) Result() string {
	return r.summary
}

func (r *bypassStubReducer) BytesParsed() int {
	return r.parsed
}

func (r *bypassStubReducer) FallbackUsed() bool {
	return r.fallback
}

func TestReducerSummaryForBypassPrefersCheaperFullParse(t *testing.T) {
	t.Parallel()

	raw := "match one\nmatch two\nmatch three\nmatch four"
	reducer := &bypassStubReducer{summary: "4 matches", parsed: len(raw)}
	summary, ok := reducerSummaryForBypass(0, reducer, raw, len(raw), false)
	if !ok || summary != "4 matches" {
		t.Fatalf("expected cheaper reducer summary, got %q ok=%v", summary, ok)
	}
}

func TestReducerSummaryForBypassStaysConservative(t *testing.T) {
	t.Parallel()

	raw := "match one\nmatch two\nmatch three\nmatch four"
	cheap := func() *bypassStubReducer {
		return &bypassStubReducer{summary: "4 matches", parsed: len(raw)}
	}

	cases := []struct {
		name             string
		exitCode         int
		reducer          StreamReducer
		rawBytesRead     int
		captureTruncated bool
	}{
		{name: "nil reducer", rawBytesRead: len(raw)},
		{name: "nonzero exit", exitCode: 1, reducer: cheap(), rawBytesRead: len(raw)},
		{name: "truncated capture", reducer: cheap(), rawBytesRead: len(raw), captureTruncated: true},
		{name: "partial parse", reducer: &bypassStubReducer{summary: "4 matches", parsed: 4}, rawBytesRead: len(raw)},
		{name: "fallback used", reducer: &bypassStubReducer{summary: "4 matches", parsed: len(raw), fallback: true}, rawBytesRead: len(raw)},
		{name: "blank summary", reducer: &bypassStubReducer{summary: "  \n", parsed: len(raw)}, rawBytesRead: len(raw)},
		{name: "summary not cheaper", reducer: &bypassStubReducer{summary: raw, parsed: len(raw)}, rawBytesRead: len(raw)},
	}
	for _, tc := range cases {
		if summary, ok := reducerSummaryForBypass(tc.exitCode, tc.reducer, raw, tc.rawBytesRead, tc.captureTruncated); ok {
			t.Fatalf("%s: expected raw bypass to win, got summary %q", tc.name, summary)
		}
	}
}
