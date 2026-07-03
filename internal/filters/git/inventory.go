package git

import (
	"fmt"
	"sort"
	"strings"
)

// The inventory render answers "what did this diff touch?" for diffs whose
// file count exceeds the per-file summary budget. The top files by churn keep
// their full summary line (hunks, +/- counts, anchors); every remaining file
// stays discoverable through directory-grouped basename lines. Losing the
// file list is losing the diff: a "+292 more files" tail makes the touched
// paths unrecoverable, while grouped basenames cost a fraction of a line each.
const (
	inventoryTopFileCount    = 5
	inventoryGroupLineLength = 140
)

type inventoryLine struct {
	text  string
	files int
}

func (r *GitDiffReducer) renderPatchInventory() []string {
	topIndices := r.topChurnFileIndices(inventoryTopFileCount)
	lines := make([]inventoryLine, 0, len(topIndices)+len(r.patchFiles)/4+2)
	for _, idx := range topIndices {
		lines = append(lines, inventoryLine{text: formatPatchFileSummary(r.patchFiles[idx]), files: 1})
	}
	lines = append(lines, r.directoryGroupLines(topIndices)...)
	fitted, dropped, leftover := r.fitInventoryLines(lines)
	return r.appendInventorySnippets(fitted, topIndices, dropped, leftover)
}

// appendInventorySnippets spends leftover contract budget on added-line
// anchors for the top-churn files, in churn order: the inventory names every
// touched file, and the anchors make the highest-churn files' content
// recognizable, so an agent hunting one changed line learns which file to
// open without the full hunks. A negative leftover means the contract is
// predicted to stay disarmed and the anchors are free.
func (r *GitDiffReducer) appendInventorySnippets(fitted []string, topIndices []int, dropped, leftover int) []string {
	if dropped > 0 {
		return fitted
	}
	exhausted := false
	for i, idx := range topIndices {
		if exhausted || i >= len(fitted) {
			break
		}
		fitted[i], leftover, exhausted = appendLineSnippets(fitted[i], r.patchFiles[idx].Snippets, leftover)
	}
	return fitted
}

// appendLineSnippets appends the snippets to one inventory line while they
// fit the leftover budget (negative means unlimited). Returns the updated
// line, the remaining budget, and whether the budget ran out.
func appendLineSnippets(line string, snippets []string, leftover int) (string, int, bool) {
	for _, snippet := range snippets {
		if strings.Contains(line, snippet) {
			continue
		}
		addition := "  " + snippet
		cost := verbatimLineCost(addition)
		if leftover >= 0 && cost > leftover {
			return line, leftover, true
		}
		line += addition
		if leftover >= 0 {
			leftover -= cost
		}
	}
	return line, leftover, false
}

// topChurnFileIndices returns the indices of the most-changed files, ordered
// by churn (additions plus deletions) descending.
func (r *GitDiffReducer) topChurnFileIndices(limit int) []int {
	if limit > len(r.patchFiles) {
		limit = len(r.patchFiles)
	}
	indices := make([]int, len(r.patchFiles))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		left := r.patchFiles[indices[a]].Additions + r.patchFiles[indices[a]].Deletions
		right := r.patchFiles[indices[b]].Additions + r.patchFiles[indices[b]].Deletions
		return left > right
	})
	return indices[:limit]
}

// directoryGroupLines lists every file not already summarized individually,
// grouped by directory with per-directory counts and churn totals. Long
// groups wrap onto continuation lines so no line grows unbounded.
func (r *GitDiffReducer) directoryGroupLines(topIndices []int) []inventoryLine {
	skip := map[int]bool{}
	for _, idx := range topIndices {
		skip[idx] = true
	}
	dirOrder, dirFiles := r.groupFilesByDirectory(skip)
	out := []inventoryLine{}
	for _, dir := range dirOrder {
		out = append(out, renderDirectoryGroup(dir, dirFiles[dir])...)
	}
	return out
}

func (r *GitDiffReducer) groupFilesByDirectory(skip map[int]bool) ([]string, map[string][]gitDiffPatchFile) {
	order := []string{}
	files := map[string][]gitDiffPatchFile{}
	for i, file := range r.patchFiles {
		if skip[i] {
			continue
		}
		dir := "./"
		if idx := strings.LastIndex(file.Path, "/"); idx >= 0 {
			dir = file.Path[:idx+1]
		}
		if _, seen := files[dir]; !seen {
			order = append(order, dir)
		}
		files[dir] = append(files[dir], file)
	}
	return order, files
}

func renderDirectoryGroup(dir string, files []gitDiffPatchFile) []inventoryLine {
	additions, deletions := 0, 0
	for _, file := range files {
		additions += file.Additions
		deletions += file.Deletions
	}
	prefix := fmt.Sprintf("%s (%d files +%d -%d): ", dir, len(files), additions, deletions)
	if len(files) == 1 {
		prefix = dir + ": "
	}
	return wrapGroupNames(prefix, files)
}

// wrapGroupNames emits the group's basenames, starting a continuation line
// (same prefix) whenever the current line would exceed the wrap length.
func wrapGroupNames(prefix string, files []gitDiffPatchFile) []inventoryLine {
	out := []inventoryLine{}
	current := prefix
	count := 0
	for _, file := range files {
		name := baseName(patchFileLabel(file))
		if count > 0 && len(current)+len(name)+1 > inventoryGroupLineLength {
			out = append(out, inventoryLine{text: current, files: count})
			current, count = prefix, 0
		}
		if count > 0 {
			current += " "
		}
		current += name
		count++
	}
	if count > 0 {
		out = append(out, inventoryLine{text: current, files: count})
	}
	return out
}

func baseName(label string) string {
	if idx := strings.LastIndex(label, "/"); idx >= 0 && idx+1 < len(label) {
		return label[idx+1:]
	}
	return label
}

// fitInventoryLines self-caps the inventory to the predicted engine
// compression-contract allowance (see verbatimTokenAllowance): a render
// within the allowance is never crushed downstream, so fitting here keeps
// the filenames instead of letting a generic token capper pick survivors.
// An allowance of 0 means the contract is predicted to stay disarmed and the
// full inventory renders as-is (reported as a negative leftover). Returns
// the fitted lines, the number of files dropped, and the unspent budget.
func (r *GitDiffReducer) fitInventoryLines(lines []inventoryLine) ([]string, int, int) {
	budget := r.inventoryTokenBudget()
	if budget <= 0 {
		return inventoryLineTexts(lines), 0, -1
	}
	out := make([]string, 0, len(lines))
	dropped := 0
	for _, line := range lines {
		cost := verbatimLineCost(line.text)
		if dropped > 0 || (cost > budget && len(out) > 0) {
			dropped += line.files
			continue
		}
		budget -= cost
		out = append(out, line.text)
	}
	if dropped > 0 {
		out = append(out, fmt.Sprintf("... +%d more files", dropped))
	}
	return out, dropped, budget
}

// inventoryTokenBudget derives the inventory's line budget from the predicted
// contract allowance, reserving the header and truncation-marker costs. A
// non-positive budget means the contract is predicted to stay disarmed.
func (r *GitDiffReducer) inventoryTokenBudget() int {
	allowance := r.verbatimTokenAllowance()
	if allowance <= 0 {
		return 0
	}
	header := fmt.Sprintf("files=%d +%d -%d", r.displayFileCount(), r.additions, r.deletions)
	return allowance - verbatimLineCost(header) - verbatimLineCost("... +999 more files")
}

func inventoryLineTexts(lines []inventoryLine) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.text)
	}
	return out
}
