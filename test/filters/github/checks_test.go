package github_test

import (
	"strings"
	"testing"

	ghfilter "github.com/devr-tools/szr/internal/filters/github"
)

func TestSummarizeGHPRChecks(t *testing.T) {
	t.Parallel()

	allPass := strings.Join([]string{
		"CodeQL\tpass\t6s\thttps://github.com/devr-tools/szr/runs/84842210450",
		"lint\tpass\t24s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420601",
		"test (ubuntu-latest full)\tpass\t42s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
	}, "\n")
	mixed := strings.Join([]string{
		"CodeQL\tpass\t6s\thttps://github.com/devr-tools/szr/runs/84842210450",
		"test (ubuntu-latest full)\tfail\t42s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
		"coverage\tpending\t0\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420930",
		"docs\tskipping\t0\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420944",
	}, "\n")

	tests := []struct {
		name       string
		input      string
		maxLines   int
		want       []string
		wantAbsent []string
	}{
		{
			name:     "all pass collapses to a single summary line",
			input:    allPass,
			maxLines: 10,
			want:     []string{"checks: 3 pass (3 total)"},
			wantAbsent: []string{
				"https://github.com",
				"CodeQL",
				"\n",
			},
		},
		{
			name:     "mixed keeps failing and pending rows with full url",
			input:    mixed,
			maxLines: 10,
			want: []string{
				"checks: 1 pass, 1 fail, 1 pending, 1 skipping (4 total)",
				"fail: test (ubuntu-latest full) 42s https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
				"pending: coverage https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420930",
			},
			wantAbsent: []string{
				"https://github.com/devr-tools/szr/runs/84842210450",
				"docs\t",
			},
		},
		{
			name:     "watch repaints dedupe to the final state",
			input:    allPass + "\n\n" + allPass + "\n\n" + strings.Replace(allPass, "lint\tpass\t24s", "lint\tfail\t31s", 1),
			maxLines: 10,
			want: []string{
				"checks: 2 pass, 1 fail (3 total) (watched 2 updates)",
				"fail: lint 31s https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420601",
			},
		},
		{
			name:     "column aligned symbol rows parse",
			input:    "✓  CodeQL   6s   https://github.com/devr-tools/szr/runs/84842210450\nX  test (ubuntu-latest full)   42s   https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
			maxLines: 10,
			want: []string{
				"checks: 1 pass, 1 fail (2 total)",
				"fail: test (ubuntu-latest full) 42s https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
			},
		},
		{
			name:     "malformed rows kept as unrecognized lines within budget",
			input:    mixed + "\nsomething odd happened here",
			maxLines: 10,
			want: []string{
				"checks: 1 pass, 1 fail, 1 pending, 1 skipping (4 total)",
				"something odd happened here",
			},
		},
		{
			name:     "budget caps failing rows with more marker",
			input:    mixed,
			maxLines: 2,
			want: []string{
				"checks: 1 pass, 1 fail, 1 pending, 1 skipping (4 total)",
				"... +1 more lines",
			},
			wantAbsent: []string{"pending: coverage"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ghfilter.SummarizeGHPRChecks(tc.input, tc.maxLines)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q in gh pr checks summary:\n%s", want, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Fatalf("expected %q to be absent from gh pr checks summary:\n%s", absent, got)
				}
			}
		})
	}
}

func TestSummarizeGHPRChecksFallbacks(t *testing.T) {
	t.Parallel()

	if got := ghfilter.SummarizeGHPRChecks("", 5); got != "ok" {
		t.Fatalf("expected ok for empty checks input, got %q", got)
	}

	errorOnly := "no checks reported on the 'feature/reducers' branch"
	got := ghfilter.SummarizeGHPRChecks(errorOnly, 5)
	if !strings.Contains(got, "no checks reported") {
		t.Fatalf("expected non-table output to survive, got %q", got)
	}
}

func TestGHPRChecksRecoveryInfo(t *testing.T) {
	t.Parallel()

	table := strings.Join([]string{
		"CodeQL\tpass\t6s\thttps://github.com/devr-tools/szr/runs/84842210450",
		"lint\tfail\t24s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420601",
	}, "\n")

	// One pass row is folded into the counts line, so recovery reports it.
	if kind, summary, requireRawCapture := ghfilter.GHPRChecksRecoveryInfo(table, 10); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh pr checks recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	// Repaints add the deduped duplicate rows to the omitted count.
	watched := table + "\n\n" + table
	if kind, summary, requireRawCapture := ghfilter.GHPRChecksRecoveryInfo(watched, 10); kind != "full-output" || summary != "omitted 3 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected watched recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	if kind, summary, requireRawCapture := ghfilter.GHPRChecksRecoveryInfo("lint\tfail\t24s\thttps://example.test/j/1", 10); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("expected no recovery when nothing was omitted: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
