package github_test

import (
	"fmt"
	"strings"
	"testing"

	ghfilter "github.com/devr-tools/szr/internal/filters/github"
)

// TestSummarizeItemListGitLabShape pins the GitLab CLI JSON shape
// (iid/source_branch/draft): every item renders as one bounded line so MR
// titles survive instead of a raw JSON head.
func TestSummarizeItemListGitLabShape(t *testing.T) {
	input := `[
		{"iid":218,"title":"feat(auth): rotate session keys on login","state":"opened","draft":false,"source_branch":"feat/session-rotation","target_branch":"main"},
		{"iid":214,"title":"fix(ledger): carry remainder cents on split","state":"merged","draft":false,"source_branch":"fix/split-remainder","target_branch":"main"},
		{"iid":209,"title":"docs: describe backfill runbook","state":"opened","draft":true,"source_branch":"docs/backfill","target_branch":"main"}
	]`

	got := ghfilter.SummarizeItemList(input, 8)
	for _, want := range []string{
		"items: 3 (opened=2 merged=1)",
		"#218 opened feat(auth): rotate session keys on login feat/session-rotation->main",
		"#214 merged fix(ledger): carry remainder cents on split",
		"#209 opened docs: describe backfill runbook [draft]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in item list summary:\n%s", want, got)
		}
	}
}

// TestSummarizeItemListGitHubShape pins the GitHub CLI JSON shape
// (number/headRefName/isDraft).
func TestSummarizeItemListGitHubShape(t *testing.T) {
	input := `[
		{"number":77,"title":"Add retry budget to fetcher","state":"OPEN","isDraft":false,"headRefName":"retry-budget","baseRefName":"main"},
		{"number":74,"title":"Bump minimum runtime","state":"MERGED","isDraft":false,"headRefName":"runtime-bump","baseRefName":"main"}
	]`

	got := ghfilter.SummarizeItemList(input, 6)
	for _, want := range []string{
		"items: 2",
		"#77 open Add retry budget to fetcher retry-budget->main",
		"#74 merged Bump minimum runtime",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in item list summary:\n%s", want, got)
		}
	}
}

// TestSummarizeItemListKeepsMinorityState pins the anomaly rule for lists:
// the odd item out (one closed MR among open ones) survives even beyond the
// positional budget.
func TestSummarizeItemListKeepsMinorityState(t *testing.T) {
	items := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		state := "opened"
		title := fmt.Sprintf("chore(deps): weekly bump %d", i)
		if i == 10 {
			state = "closed"
			title = "fix(cache): expire stale tenants"
		}
		items = append(items, fmt.Sprintf(`{"iid":%d,"title":%q,"state":%q}`, 300+i, title, state))
	}
	input := "[" + strings.Join(items, ",") + "]"

	got := ghfilter.SummarizeItemList(input, 5)
	for _, want := range []string{
		"items: 12 (opened=11 closed=1)",
		"#310 closed fix(cache): expire stale tenants",
		"... +8 more items",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in minority-state list summary:\n%s", want, got)
		}
	}
}

// TestItemListRecoveryInfo pins recovery metadata when items are omitted.
func TestItemListRecoveryInfo(t *testing.T) {
	input := `[
		{"iid":1,"title":"one","state":"opened"},
		{"iid":2,"title":"two","state":"opened"},
		{"iid":3,"title":"three","state":"opened"}
	]`
	if kind, summary, requireRawCapture := ghfilter.ItemListRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 1 additional items" || !requireRawCapture {
		t.Fatalf("unexpected item list recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
	if kind, summary, requireRawCapture := ghfilter.ItemListRecoveryInfo(input, 8); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("expected no recovery for a fully rendered list: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
