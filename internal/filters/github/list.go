package github

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

// ListItem is one row of a pull request, merge request, or issue list in
// either GitHub CLI JSON (number/isDraft/headRefName) or GitLab CLI JSON
// (iid/draft/source_branch) shape.
type ListItem struct {
	Number       int    `json:"number"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	IsDraft      bool   `json:"isDraft"`
	Draft        bool   `json:"draft"`
	HeadRefName  string `json:"headRefName"`
	SourceBranch string `json:"source_branch"`
	BaseRefName  string `json:"baseRefName"`
	TargetBranch string `json:"target_branch"`
}

func SummarizeItemList(input string, maxLines int) string {
	return summarizeItemListResult(input, maxLines).Text
}

func ItemListRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeItemListResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional items", result.OmittedCount))
}

func summarizeItemListResult(input string, maxLines int) githubSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := strings.TrimSpace(shared.StripANSI(input))
	items, ok := decodeListItems(clean)
	if !ok {
		return githubSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	if len(items) == 0 {
		return githubSummaryResult{Text: "items: 0"}
	}

	kept, omitted := selectListItems(items, maxLines)
	out := append([]string{itemListHeader(items)}, kept...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d more items", omitted))
	}
	return githubSummaryResult{Text: strings.Join(out, "\n"), OmittedCount: omitted}
}

func itemListHeader(items []ListItem) string {
	header := fmt.Sprintf("items: %d", len(items))
	if breakdown := itemStateBreakdown(items); breakdown != "" {
		header += " (" + breakdown + ")"
	}
	return header
}

func itemState(item ListItem) string {
	return strings.ToLower(strings.TrimSpace(item.State))
}

func itemStateCounts(items []ListItem) (map[string]int, []string) {
	counts := map[string]int{}
	order := []string{}
	for _, item := range items {
		state := itemState(item)
		if state == "" {
			continue
		}
		if _, seen := counts[state]; !seen {
			order = append(order, state)
		}
		counts[state]++
	}
	return counts, order
}

func decodeListItems(clean string) ([]ListItem, bool) {
	if !strings.HasPrefix(clean, "[") {
		return nil, false
	}
	var items []ListItem
	if err := json.Unmarshal([]byte(clean), &items); err != nil {
		return nil, false
	}
	for _, item := range items {
		if item.Title == "" && item.Number == 0 && item.IID == 0 {
			return nil, false
		}
	}
	return items, true
}

func formatListItem(item ListItem) string {
	number := item.Number
	if number == 0 {
		number = item.IID
	}
	line := fmt.Sprintf("#%d %s %s", number, strings.ToLower(orDefault(item.State, "unknown")), item.Title)
	if item.IsDraft || item.Draft {
		line += " [draft]"
	}
	head := orDefault(item.HeadRefName, item.SourceBranch)
	base := orDefault(item.BaseRefName, item.TargetBranch)
	if strings.TrimSpace(head) != "" || strings.TrimSpace(base) != "" {
		line += fmt.Sprintf(" %s->%s", strings.TrimSpace(head), strings.TrimSpace(base))
	}
	return shared.Clip(line, 160)
}

func itemStateBreakdown(items []ListItem) string {
	counts, order := itemStateCounts(items)
	if len(order) < 2 {
		return ""
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	parts := make([]string, 0, len(order))
	for _, state := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", state, counts[state]))
	}
	return strings.Join(parts, " ")
}

// selectListItems keeps every item line when the budget allows, and otherwise
// minority-state items plus leading items: in a list the odd item out (a
// closed MR among open ones, a conflicting one among mergeable ones) is the
// payload, so positional truncation must never be what drops it.
func selectListItems(items []ListItem, maxLines int) ([]string, int) {
	limit := maxLines - 1
	if limit < 1 {
		limit = 1
	}
	if len(items) <= limit {
		return formatListItems(items, nil), 0
	}
	keep := keepItemIndices(minorityStateItemIndices(items), len(items), limit)
	out := formatListItems(items, keep)
	return out, len(items) - len(out)
}

func formatListItems(items []ListItem, keep map[int]bool) []string {
	out := make([]string, 0, len(items))
	for i, item := range items {
		if keep != nil && !keep[i] {
			continue
		}
		out = append(out, formatListItem(item))
	}
	return out
}

// keepItemIndices marks up to limit indices as kept: the anomalous ones
// first, then leading indices as positional fill.
func keepItemIndices(anomalies []int, total, limit int) map[int]bool {
	keep := map[int]bool{}
	for _, idx := range anomalies {
		if len(keep) >= limit {
			break
		}
		keep[idx] = true
	}
	for i := 0; i < total && len(keep) < limit; i++ {
		keep[i] = true
	}
	return keep
}

func minorityStateItemIndices(items []ListItem) []int {
	dominant := dominantItemState(items)
	if dominant == "" {
		return nil
	}
	out := []int{}
	for i, item := range items {
		state := itemState(item)
		if state != "" && state != dominant {
			out = append(out, i)
		}
	}
	return out
}

// dominantItemState returns the majority state, or "" when no state covers
// more than half of the state-bearing items.
func dominantItemState(items []ListItem) string {
	counts, _ := itemStateCounts(items)
	dominant, dominantCount, total := "", 0, 0
	for state, count := range counts {
		total += count
		if count > dominantCount {
			dominant, dominantCount = state, count
		}
	}
	if dominantCount*2 <= total {
		return ""
	}
	return dominant
}
