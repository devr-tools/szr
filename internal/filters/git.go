package filters

import (
	"fmt"
	"strings"
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
	scanner     lineScanner
	maxPaths    int
	bytesParsed int
	branch      string
	staged      int
	unstaged    int
	untracked   int
	paths       []string
}

func NewGitStatusReducer(maxLines, _ int) *GitStatusReducer {
	maxPaths := maxLines - 2
	if maxPaths <= 0 {
		maxPaths = 6
	}
	return &GitStatusReducer{
		maxPaths: maxPaths,
		paths:    make([]string, 0, maxPaths),
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
	if r.branch == "" && r.staged == 0 && r.unstaged == 0 && r.untracked == 0 && len(r.paths) == 0 {
		return "clean"
	}
	summary := []string{}
	if r.branch != "" {
		summary = append(summary, r.branch)
	}
	summary = append(summary, fmt.Sprintf("staged=%d unstaged=%d untracked=%d", r.staged, r.unstaged, r.untracked))
	if len(r.paths) > 0 {
		summary = append(summary, "files:")
		for _, path := range r.paths {
			summary = append(summary, "  "+path)
		}
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
	for _, path := range r.paths {
		if len(summary) == 2 {
			summary = append(summary, "files:")
		}
		summary = append(summary, "  "+path)
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
	if path != "" && len(r.paths) < r.maxPaths {
		r.paths = append(r.paths, path)
	}
	switch {
	case x == '?' && y == '?':
		r.untracked++
	default:
		if x != ' ' {
			r.staged++
		}
		if y != ' ' {
			r.unstaged++
		}
	}
}

type GitLogReducer struct {
	scanner     lineScanner
	maxEntries  int
	bytesParsed int
	total       int
	entries     []string
}

func NewGitLogReducer(maxLines, _ int) *GitLogReducer {
	maxEntries := maxLines - 1
	if maxEntries <= 0 {
		maxEntries = 10
	}
	return &GitLogReducer{
		maxEntries: maxEntries,
		entries:    make([]string, 0, maxEntries),
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
	return fmt.Sprintf("%d commits\n%s", r.total, strings.Join(r.entries, "\n"))
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
	return fmt.Sprintf("%d commits\n%s", r.total, strings.Join(r.entries, "\n"))
}

func (r *GitLogReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitLogReducer) recordLine(line string) {
	r.total++
	if len(r.entries) < r.maxEntries {
		r.entries = append(r.entries, line)
	}
}

type GitDiffReducer struct {
	scanner     lineScanner
	maxSummary  int
	bytesParsed int
	fileCount   int
	additions   int
	deletions   int
	summary     []string
	fallback    *CompactLineReducer
}

func NewGitDiffReducer(maxLines, maxBytes int) *GitDiffReducer {
	maxSummary := maxLines - 1
	if maxSummary <= 0 {
		maxSummary = 8
	}
	return &GitDiffReducer{
		maxSummary: maxSummary,
		summary:    make([]string, 0, maxSummary),
		fallback:   NewCompactLineReducer(12, maxBytes),
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
