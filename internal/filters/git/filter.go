package git

import (
	"fmt"
	"strings"

	shared "szr/internal/filters"
)

func SummarizeGitStatus(input string) string {
	reducer := NewGitStatusReducer(8, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func SummarizeGitLog(input string) string {
	reducer := NewGitLogReducer(11, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func SummarizeGitDiff(input string) string {
	reducer := NewGitDiffReducer(9, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

type GitStatusReducer struct {
	scanner        scanner
	maxPreview     int
	bytesParsed    int
	branch         string
	staged         int
	unstaged       int
	untracked      int
	stagedPaths    []string
	unstagedPaths  []string
	untrackedPaths []string
}

func NewGitStatusReducer(maxLines, _ int) *GitStatusReducer {
	maxPreview := maxLines - 2
	if maxPreview <= 0 {
		maxPreview = 4
	}
	return &GitStatusReducer{
		maxPreview:     maxPreview,
		stagedPaths:    make([]string, 0, maxPreview),
		unstagedPaths:  make([]string, 0, maxPreview),
		untrackedPaths: make([]string, 0, maxPreview),
	}
}

func (r *GitStatusReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *GitStatusReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *GitStatusReducer) Result() string {
	r.scanner.Finish(r.recordLine)
	if r.branch == "" && r.staged == 0 && r.unstaged == 0 && r.untracked == 0 && len(r.stagedPaths) == 0 && len(r.unstagedPaths) == 0 && len(r.untrackedPaths) == 0 {
		return "clean"
	}
	summary := []string{}
	if r.branch != "" {
		summary = append(summary, r.branch)
	}
	summary = append(summary, fmt.Sprintf("staged=%d unstaged=%d untracked=%d", r.staged, r.unstaged, r.untracked))
	if line := previewSection("staged", r.stagedPaths, r.staged, 2); line != "" {
		summary = append(summary, line)
	}
	if line := previewSection("unstaged", r.unstagedPaths, r.unstaged, 2); line != "" {
		summary = append(summary, line)
	}
	if line := previewSection("untracked", r.untrackedPaths, r.untracked, 2); line != "" {
		summary = append(summary, line)
	}
	return strings.Join(summary, "\n")
}

func (r *GitStatusReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GitStatusReducer) FallbackUsed() bool {
	return false
}

func (r *GitStatusReducer) Preview() string {
	summary := []string{}
	if r.branch != "" {
		summary = append(summary, r.branch)
	}
	if r.branch != "" || r.staged != 0 || r.unstaged != 0 || r.untracked != 0 {
		summary = append(summary, fmt.Sprintf("staged=%d unstaged=%d untracked=%d", r.staged, r.unstaged, r.untracked))
	}
	if line := previewSection("staged", r.stagedPaths, r.staged, 2); line != "" {
		summary = append(summary, line)
	}
	if line := previewSection("unstaged", r.unstagedPaths, r.unstaged, 2); line != "" {
		summary = append(summary, line)
	}
	if line := previewSection("untracked", r.untrackedPaths, r.untracked, 2); line != "" {
		summary = append(summary, line)
	}
	return strings.Join(summary, "\n")
}

func (r *GitStatusReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitStatusReducer) recordLine(line string) {
	if strings.HasPrefix(line, "## ") {
		r.branch = strings.TrimPrefix(line, "## ")
		return
	}
	if len(line) < 3 {
		return
	}
	x := line[0]
	y := line[1]
	path := strings.TrimSpace(line[3:])
	switch {
	case x == '?' && y == '?':
		r.untracked++
		r.untrackedPaths = appendPreview(r.untrackedPaths, path, r.maxPreview)
	default:
		if x != ' ' {
			r.staged++
			r.stagedPaths = appendPreview(r.stagedPaths, path, r.maxPreview)
		}
		if y != ' ' {
			r.unstaged++
			r.unstagedPaths = appendPreview(r.unstagedPaths, path, r.maxPreview)
		}
	}
}

type GitLogReducer struct {
	scanner     scanner
	maxEntries  int
	bytesParsed int
	total       int
	entries     []gitLogEntry
}

func NewGitLogReducer(maxLines, _ int) *GitLogReducer {
	maxEntries := 2
	if maxEntries <= 0 {
		maxEntries = 2
	}
	return &GitLogReducer{
		maxEntries: maxEntries,
		entries:    make([]gitLogEntry, 0, maxEntries),
	}
}

func (r *GitLogReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *GitLogReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *GitLogReducer) Result() string {
	r.scanner.Finish(r.recordLine)
	if r.total == 0 {
		return "no commits"
	}
	return formatGitLog(r.entries, r.total)
}

func (r *GitLogReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GitLogReducer) FallbackUsed() bool {
	return false
}

func (r *GitLogReducer) Preview() string {
	if r.total == 0 {
		return ""
	}
	return formatGitLog(r.entries, r.total)
}

func (r *GitLogReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitLogReducer) recordLine(line string) {
	r.total++
	hash, subject := splitGitCommit(line)
	if len(r.entries) > 0 && r.entries[len(r.entries)-1].Subject == subject {
		r.entries[len(r.entries)-1].Count++
		return
	}
	if len(r.entries) < r.maxEntries {
		r.entries = append(r.entries, gitLogEntry{
			Hash:    hash,
			Subject: subject,
			Count:   1,
		})
	}
}

func appendPreview(paths []string, path string, max int) []string {
	if path == "" || len(paths) >= max {
		return paths
	}
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}

func previewSection(label string, paths []string, total, limit int) string {
	if total == 0 || len(paths) == 0 {
		return ""
	}
	if total == 1 {
		return fmt.Sprintf("%s: %s", label, paths[0])
	}
	if limit <= 0 {
		limit = 2
	}
	buckets := summarizePathBuckets(paths)
	preview := buckets
	if len(preview) > limit {
		preview = append([]string{}, preview[:limit]...)
	}
	line := fmt.Sprintf("%s: %s", label, strings.Join(preview, ", "))
	if len(buckets) > len(preview) {
		line += fmt.Sprintf(", ... +%d more", len(buckets)-len(preview))
	}
	return line
}

func formatGitLog(entries []gitLogEntry, total int) string {
	if total == 0 {
		return "no commits"
	}
	out := make([]string, 0, len(entries)+1)
	visible := 0
	for _, entry := range entries {
		visible += entry.Count
		out = append(out, entry.Render())
	}
	if total > visible {
		out = append(out, fmt.Sprintf("... +%d more commits", total-visible))
	}
	return strings.Join(out, "\n")
}

type gitLogEntry struct {
	Hash    string
	Subject string
	Count   int
}

func (e gitLogEntry) Render() string {
	line := strings.TrimSpace(strings.TrimSpace(e.Hash + " " + e.Subject))
	if e.Count > 1 {
		line += fmt.Sprintf(" (x%d)", e.Count)
	}
	return line
}

func splitGitCommit(line string) (string, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func summarizePathBuckets(paths []string) []string {
	type bucket struct {
		label string
		count int
	}
	order := []*bucket{}
	index := map[string]*bucket{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		label := bucketLabel(path)
		item := index[label]
		if item == nil {
			item = &bucket{label: label}
			index[label] = item
			order = append(order, item)
		}
		item.count++
	}
	out := make([]string, 0, len(order))
	for _, item := range order {
		if item.count > 1 || strings.HasSuffix(item.label, "/...") {
			out = append(out, fmt.Sprintf("%s (%d)", item.label, item.count))
			continue
		}
		out = append(out, item.label)
	}
	return out
}

func bucketLabel(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if idx := strings.Index(path, "/"); idx > 0 {
		return path[:idx] + "/..."
	}
	return path
}

type GitDiffReducer struct {
	scanner     scanner
	maxSummary  int
	bytesParsed int
	fileCount   int
	additions   int
	deletions   int
	summary     []string
	fallback    *shared.CompactLineReducer
}

func NewGitDiffReducer(maxLines, maxBytes int) *GitDiffReducer {
	maxSummary := maxLines - 1
	if maxSummary <= 0 {
		maxSummary = 8
	}
	return &GitDiffReducer{
		maxSummary: maxSummary,
		summary:    make([]string, 0, maxSummary),
		fallback:   shared.NewCompactLineReducer(12, maxBytes),
	}
}

func (r *GitDiffReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
	r.fallback.ConsumeStdout(chunk)
}

func (r *GitDiffReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
	r.fallback.ConsumeStderr(chunk)
}

func (r *GitDiffReducer) Result() string {
	r.scanner.Finish(r.recordLine)
	header := fmt.Sprintf("files=%d +%d -%d", r.fileCount, r.additions, r.deletions)
	if r.fileCount == 0 && r.additions == 0 && r.deletions == 0 && len(r.summary) == 0 {
		return "no diff"
	}
	if len(r.summary) == 0 {
		return header + "\n" + r.fallback.Result()
	}
	return header + "\n" + strings.Join(r.summary, "\n")
}

func (r *GitDiffReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GitDiffReducer) FallbackUsed() bool {
	return len(r.summary) == 0
}

func (r *GitDiffReducer) Preview() string {
	if r.fileCount == 0 && r.additions == 0 && r.deletions == 0 && len(r.summary) == 0 {
		return ""
	}
	header := fmt.Sprintf("files=%d +%d -%d", r.fileCount, r.additions, r.deletions)
	if len(r.summary) == 0 {
		return header
	}
	return header + "\n" + strings.Join(r.summary, "\n")
}

func (r *GitDiffReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitDiffReducer) recordLine(line string) {
	if strings.HasPrefix(line, "diff --git ") {
		r.fileCount++
	}
	if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
		return
	}
	if strings.HasPrefix(line, "+") {
		r.additions++
	} else if strings.HasPrefix(line, "-") {
		r.deletions++
	}
	if len(r.summary) >= r.maxSummary {
		return
	}
	if strings.Contains(line, "|") || strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
		r.summary = append(r.summary, line)
	}
}

type scanner struct {
	inner shared.LineScanner
}

func (s *scanner) Consume(chunk []byte, emit func(string)) {
	s.inner.Consume(chunk, emit)
}

func (s *scanner) Finish(emit func(string)) {
	s.inner.Finish(emit)
}
