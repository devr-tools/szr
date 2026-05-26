package fs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

func SummarizeDirectoryListing(input string, maxLines int) string {
	return summarizeDirectoryListingResult(input, maxLines).Text
}

func DirectoryListingRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDirectoryListingResult(input, maxLines)
	if !result.Grouped || result.EntryCount == 0 {
		return filters.NoRecovery()
	}
	return filters.FullOutputRecovery(fmt.Sprintf("omitted %d directory entries", result.EntryCount))
}

func summarizeDirectoryListingResult(input string, maxLines int) listingSummaryResult {
	if maxLines <= 0 {
		maxLines = 6
	}

	dirs := []string{}
	files := []string{}
	hidden := 0
	for _, line := range filters.NonEmptyLines(filters.StripANSI(input)) {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "total ") {
			continue
		}
		if strings.HasPrefix(entry, ".") {
			hidden++
		}
		if strings.HasSuffix(entry, "/") {
			dirs = append(dirs, entry)
			continue
		}
		files = append(files, entry)
	}

	dirs = filters.UniqueStrings(dirs)
	files = filters.UniqueStrings(files)
	if len(dirs) == 0 && len(files) == 0 {
		return listingSummaryResult{Text: "empty"}
	}
	entryCount := len(dirs) + len(files)
	if entryCount <= minDirListThreshold(maxLines) {
		entries := append([]string{}, dirs...)
		entries = append(entries, files...)
		return listingSummaryResult{
			Text:       strings.Join(entries, "\n"),
			EntryCount: entryCount,
		}
	}

	out := []string{}
	if line := summarizeListingGroup("dirs", dirs, 3); line != "" {
		out = append(out, line)
	}
	if line := summarizeListingGroup("files", files, 3); line != "" {
		out = append(out, line)
	}
	if hidden > 0 {
		out = append(out, fmt.Sprintf("hidden: %d", hidden))
	}
	return listingSummaryResult{
		Text:       filters.JoinLimitedLines(out, maxLines),
		EntryCount: entryCount,
		Grouped:    true,
	}
}

type listingSummaryResult struct {
	Text       string
	EntryCount int
	Grouped    bool
}

func minDirListThreshold(maxLines int) int {
	threshold := maxLines - 1
	if threshold < 4 {
		return 4
	}
	return threshold
}

func summarizeListingGroup(label string, entries []string, limit int) string {
	if len(entries) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = 3
	}
	preview := append([]string{}, entries...)
	sort.Strings(preview)
	if len(preview) > limit {
		preview = append(preview[:limit], fmt.Sprintf("+%d", len(preview)-limit))
	}
	return fmt.Sprintf("%s: %s", label, strings.Join(preview, " "))
}
