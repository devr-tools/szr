package filters

import (
	"fmt"
	"sort"
	"strings"
)

var reducerOnlySearchNoiseDirs = []string{
	".gradle",
	".mypy_cache",
	".nox",
	".nuxt",
	".output",
	".parcel-cache",
	".pnpm-store",
	".ruff_cache",
	".svelte-kit",
	".venv",
	".yarn",
	"out",
	"tmp",
}

func SummarizeFindPaths(paths []string, maxLines int) string {
	if len(paths) == 0 {
		return "no matches"
	}
	if maxLines <= 0 {
		maxLines = 8
	}
	normalized := make([]string, 0, len(paths))
	suppressed := map[string]int{}
	for _, path := range paths {
		path = normalizeSearchPath(path)
		if path == "" {
			continue
		}
		if bucket := searchReducerNoiseBucket(path); bucket != "" {
			suppressed[bucket]++
			continue
		}
		normalized = append(normalized, path)
	}
	if len(normalized) == 0 {
		if len(suppressed) > 0 {
			return "no matches"
		}
		return "no matches"
	}
	sort.Strings(normalized)
	return renderFindSummary(normalized, summarizeTopLevelBuckets(normalized), suppressed, maxLines)
}

func SummarizeFindOutput(stdout, stderr string, maxLines int) string {
	lines := NonEmptyLines(StripANSI(stdout))
	if len(lines) > 0 {
		return SummarizeFindPaths(lines, maxLines)
	}
	if strings.TrimSpace(stderr) != "" {
		return CompactLines(StripANSI(stderr), maxLines)
	}
	return "no matches"
}

type FindReducer struct {
	stdoutScanner lineScanner
	stderrReducer *CompactLineReducer
	maxLines      int
	sampleLimit   int
	bytesParsed   int
	matches       []string
	seen          map[string]struct{}
	totalMatches  int
	suppressed    map[string]int
	dirCounts     map[string]int
}

func NewFindReducer(maxLines int) *FindReducer {
	if maxLines <= 0 {
		maxLines = 8
	}
	sampleLimit := maxLines - 1
	if sampleLimit < 1 {
		sampleLimit = 1
	}
	return &FindReducer{
		stderrReducer: NewCompactLineReducer(maxLines, 0),
		maxLines:      maxLines,
		sampleLimit:   sampleLimit,
		matches:       make([]string, 0, sampleLimit),
		seen:          make(map[string]struct{}, sampleLimit),
		suppressed:    map[string]int{},
		dirCounts:     map[string]int{},
	}
}

func (r *FindReducer) ConsumeStdout(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stdoutScanner.Consume(chunk, r.ingestPath)
}

func (r *FindReducer) ConsumeStderr(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stderrReducer.ConsumeStderr(chunk)
}

func (r *FindReducer) Result() string {
	r.stdoutScanner.Finish(r.ingestPath)
	return r.render(false)
}

func (r *FindReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *FindReducer) FallbackUsed() bool {
	return false
}

func (r *FindReducer) Done() bool {
	return r.stderrReducer.Preview() == "" && len(r.matches) >= r.sampleLimit && r.totalMatches > len(r.matches)
}

func (r *FindReducer) Preview() string {
	return r.render(true)
}

func (r *FindReducer) RecoveryInfo() (string, string, bool) {
	extra := r.totalMatches - len(r.matches)
	if extra <= 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(fmt.Sprintf("omitted %d additional matches", extra))
}

func (r *FindReducer) ingestPath(line string) {
	path := normalizeSearchPath(line)
	if path == "" {
		return
	}
	if bucket := searchReducerNoiseBucket(path); bucket != "" {
		r.suppressed[bucket]++
		return
	}
	if _, ok := r.seen[path]; ok {
		return
	}
	r.seen[path] = struct{}{}
	r.totalMatches++
	r.dirCounts[pathTopLevelBucket(path)]++
	if len(r.matches) < r.sampleLimit {
		r.matches = append(r.matches, path)
		sort.Strings(r.matches)
	}
}

func (r *FindReducer) render(preview bool) string {
	if stderr := strings.TrimSpace(r.stderrReducer.Preview()); stderr != "" {
		return stderr
	}
	if len(r.matches) == 0 {
		return "no matches"
	}
	return renderFindSummary(r.matches, r.dirCounts, r.suppressed, r.maxLines)
}

func searchReducerNoiseBucket(path string) string {
	normalized := normalizeSearchPath(path)
	if normalized == "" {
		return ""
	}
	lower := strings.ToLower(normalized)
	switch {
	case strings.HasSuffix(lower, ".min.js"):
		return "minified assets"
	case strings.HasSuffix(lower, ".min.css"):
		return "minified assets"
	case strings.HasSuffix(lower, ".js.map"):
		return "source maps"
	case strings.HasSuffix(lower, ".css.map"):
		return "source maps"
	}
	if bucket := SearchNoiseBucket(normalized); bucket != "" {
		return bucket
	}
	trimmed := strings.TrimPrefix(normalized, "/")
	parts := strings.Split(trimmed, "/")
	for idx, part := range parts {
		if strings.HasPrefix(normalized, "/") && idx == 0 {
			continue
		}
		for _, dir := range reducerOnlySearchNoiseDirs {
			if part == dir {
				return dir
			}
		}
	}
	return ""
}

func summarizeSuppressedSearchBuckets(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	type bucket struct {
		name  string
		count int
	}
	order := make([]bucket, 0, len(counts))
	total := 0
	for name, count := range counts {
		if count <= 0 {
			continue
		}
		order = append(order, bucket{name: name, count: count})
		total += count
	}
	if len(order) == 0 {
		return ""
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].count == order[j].count {
			return order[i].name < order[j].name
		}
		return order[i].count > order[j].count
	})
	labels := make([]string, 0, minInt(3, len(order)))
	for _, item := range order[:minInt(3, len(order))] {
		labels = append(labels, item.name)
	}
	return fmt.Sprintf("suppressed noisy paths: %d (%s)", total, strings.Join(labels, ", "))
}

func renderFindSummary(samples []string, dirCounts map[string]int, suppressed map[string]int, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}
	totalMatches := 0
	for _, count := range dirCounts {
		totalMatches += count
	}
	if totalMatches == 0 {
		totalMatches = len(samples)
	}
	lines := []string{fmt.Sprintf("%d matches across %d dirs", totalMatches, len(dirCounts))}
	if line := summarizeTopLevelBucketsLine(dirCounts); line != "" && len(lines) < maxLines {
		lines = append(lines, line)
	}
	reserved := 0
	if totalMatches > len(samples) {
		reserved++
	}
	if summarizeSuppressedSearchBuckets(suppressed) != "" {
		reserved++
	}
	visible := maxLines - len(lines) - reserved
	if visible < 1 {
		visible = 1
	}
	if visible > 2 {
		visible = 2
	}
	if visible > len(samples) {
		visible = len(samples)
	}
	lines = append(lines, samples[:visible]...)
	if extra := totalMatches - visible; extra > 0 && len(lines) < maxLines {
		lines = append(lines, fmt.Sprintf("... +%d more matches", extra))
	}
	if line := summarizeSuppressedSearchBuckets(suppressed); line != "" && len(lines) < maxLines {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func summarizeTopLevelBuckets(paths []string) map[string]int {
	counts := map[string]int{}
	for _, path := range paths {
		counts[pathTopLevelBucket(path)]++
	}
	return counts
}

func pathTopLevelBucket(path string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(path), "./")
	if trimmed == "" {
		return "."
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) <= 1 {
		return "."
	}
	return parts[0] + "/"
}

func summarizeTopLevelBucketsLine(counts map[string]int) string {
	type bucket struct {
		name  string
		count int
	}
	order := make([]bucket, 0, len(counts))
	for name, count := range counts {
		if count > 0 {
			order = append(order, bucket{name: name, count: count})
		}
	}
	if len(order) == 0 {
		return ""
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].count == order[j].count {
			return order[i].name < order[j].name
		}
		return order[i].count > order[j].count
	})
	parts := make([]string, 0, minInt(3, len(order)))
	for _, item := range order[:minInt(3, len(order))] {
		parts = append(parts, fmt.Sprintf("%s (%d)", item.name, item.count))
	}
	return "top dirs: " + strings.Join(parts, ", ")
}
