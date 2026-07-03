package git

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
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
	return reducer.Reduce(input)
}

type GitStatusReducer struct {
	scanner        scanner
	maxPreview     int
	bytesParsed    int
	branch         string
	changedEntries int
	staged         int
	unstaged       int
	untracked      int
	entries        []string
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
		entries:        make([]string, 0, maxPreview),
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
	if compact := r.compactSummary(); compact != "" {
		return compact
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
	if compact := r.compactSummary(); compact != "" {
		return compact
	}
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
	r.changedEntries++
	r.entries = appendPreview(r.entries, strings.TrimRight(line, " "), r.maxPreview)
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

func (r *GitStatusReducer) compactSummary() string {
	if r.changedEntries == 0 {
		return r.branch
	}
	if r.changedEntries <= 2 && len(r.entries) == r.changedEntries {
		summary := []string{}
		if r.branch != "" {
			summary = append(summary, r.branch)
		}
		summary = append(summary, r.entries...)
		return strings.Join(summary, "\n")
	}
	return ""
}

type GitLogReducer struct {
	scanner           scanner
	maxEntries        int
	bytesParsed       int
	total             int
	entries           []gitLogEntry
	fullFormat        bool
	pendingHash       string
	pendingHasSubject bool
}

func NewGitLogReducer(maxLines, maxBytes int) *GitLogReducer {
	return NewGitLogReducerWithEntries(maxLines, maxBytes, 0)
}

// NewGitLogReducerWithEntries keeps up to maxEntries commit lines visible so
// explicit user counts (for example `git log -5`) survive summarization.
func NewGitLogReducerWithEntries(_, _, maxEntries int) *GitLogReducer {
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

func (r *GitLogReducer) RecoveryInfo() (string, string, bool) {
	visible := 0
	for _, entry := range r.entries {
		visible += entry.Count
	}
	if extra := r.total - visible; extra > 0 {
		return shared.FullOutputRecovery(fmt.Sprintf("omitted %d commits", extra))
	}
	return shared.NoRecovery()
}

func (r *GitLogReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitLogReducer) recordLine(line string) {
	if hash, ok := gitCommitHeaderHash(line); ok {
		r.fullFormat = true
		r.total++
		r.pendingHash = hash
		r.pendingHasSubject = false
		return
	}
	if r.fullFormat {
		r.recordFullFormatLine(line)
		return
	}
	r.total++
	hash, subject := splitGitCommit(line)
	r.appendLogEntry(hash, subject)
}

// recordFullFormatLine extracts the subject of the pending default-format
// commit: the first indented message line after a `commit <hash>` header.
func (r *GitLogReducer) recordFullFormatLine(line string) {
	if r.pendingHasSubject || !strings.HasPrefix(line, "    ") {
		return
	}
	r.pendingHasSubject = true
	r.appendLogEntry(r.pendingHash, strings.TrimSpace(line))
}

func (r *GitLogReducer) appendLogEntry(hash, subject string) {
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

func gitCommitHeaderHash(line string) (string, bool) {
	if !strings.HasPrefix(line, "commit ") {
		return "", false
	}
	fields := strings.Fields(strings.TrimPrefix(line, "commit "))
	if len(fields) == 0 {
		return "", false
	}
	hash := fields[0]
	if len(hash) < 7 || !isHexString(hash) {
		return "", false
	}
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return hash, true
}

func isHexString(value string) bool {
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return value != ""
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
	out := make([]string, 0, len(entries)+2)
	out = append(out, fmt.Sprintf("%d commits", total))
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

// Small diffs render their changed lines verbatim instead of a stats-only
// summary: the +/- content of a 15-line diff is the payload the agent asked
// for, and eliding it to hunk counts destroys it for trivial savings. The
// caps bound the mode to genuinely small diffs (rendered lines including one
// header per file, and bytes of retained content).
const (
	gitDiffVerbatimMaxLines = 40
	gitDiffVerbatimMaxBytes = 2048
)

type GitDiffReducer struct {
	scanner               scanner
	maxSummary            int
	bytesParsed           int
	fileCount             int
	additions             int
	deletions             int
	summary               []string
	fallback              *shared.CompactLineReducer
	patchFiles            []gitDiffPatchFile
	currentPatch          *gitDiffPatchFile
	statEntries           []gitDiffStatEntry
	maxTokens             int
	aggressive            bool
	largeFileThreshold    int
	largeSummaryTopN      int
	aggressiveSummaryTopN int
	verbatimDisabled      bool
	verbatimLines         int
	verbatimBytes         int
}

type GitDiffReducerOptions struct {
	MaxLines int
	MaxBytes int
	// MaxTokens mirrors the profile budget's token cap so the reducer's
	// self-capping renders (verbatim and inventory modes) predict the same
	// allowance the engine compression contract will enforce.
	MaxTokens             int
	Aggressive            bool
	LargeFileThreshold    int
	LargeSummaryTopN      int
	AggressiveSummaryTopN int
}

type gitDiffPatchFile struct {
	Path         string
	Hunks        int
	Anchors      []string
	Additions    int
	Deletions    int
	IsNew        bool
	IsDeleted    bool
	IsRenamed    bool
	IsConflicted bool
	OldPath      string
	NewPath      string
	Lines        []string
	Snippets     []string
}

func NewGitDiffReducer(maxLines, maxBytes int) *GitDiffReducer {
	return NewGitDiffReducerWithOptions(GitDiffReducerOptions{
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
		LargeFileThreshold:    8,
		LargeSummaryTopN:      5,
		AggressiveSummaryTopN: 3,
	})
}

func NewGitDiffReducerWithOptions(opts GitDiffReducerOptions) *GitDiffReducer {
	maxSummary := opts.MaxLines - 1
	if maxSummary <= 0 {
		maxSummary = 8
	}
	if opts.LargeFileThreshold <= 0 {
		opts.LargeFileThreshold = 8
	}
	if opts.LargeSummaryTopN <= 0 {
		opts.LargeSummaryTopN = 5
	}
	if opts.AggressiveSummaryTopN <= 0 {
		opts.AggressiveSummaryTopN = 3
	}
	return &GitDiffReducer{
		maxSummary:            maxSummary,
		summary:               make([]string, 0, maxSummary),
		fallback:              shared.NewCompactLineReducer(12, opts.MaxBytes),
		patchFiles:            make([]gitDiffPatchFile, 0, maxSummary),
		statEntries:           make([]gitDiffStatEntry, 0, maxSummary),
		maxTokens:             opts.MaxTokens,
		aggressive:            opts.Aggressive,
		largeFileThreshold:    opts.LargeFileThreshold,
		largeSummaryTopN:      opts.LargeSummaryTopN,
		aggressiveSummaryTopN: opts.AggressiveSummaryTopN,
		verbatimDisabled:      opts.Aggressive,
	}
}

func (r *GitDiffReducer) Reduce(input string) string {
	r.ConsumeStdout([]byte(input))
	return r.Result()
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
	header := fmt.Sprintf("files=%d +%d -%d", r.displayFileCount(), r.additions, r.deletions)
	if r.fileCount == 0 && r.additions == 0 && r.deletions == 0 && len(r.summary) == 0 {
		return "no diff"
	}
	if r.shouldCondenseStatSummary() {
		return header + "\n" + strings.Join(r.renderCondensedSummary(), "\n")
	}
	if len(r.summary) == 0 && len(r.patchFiles) > 0 {
		if r.verbatimRenderable() {
			return header + "\n" + strings.Join(r.renderVerbatimPatch(verbatimLineCost(header)), "\n")
		}
		return header + "\n" + strings.Join(r.renderPatchSummary(), "\n")
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
	return len(r.summary) == 0 && len(r.patchFiles) == 0
}

func (r *GitDiffReducer) Preview() string {
	if r.fileCount == 0 && r.additions == 0 && r.deletions == 0 && len(r.summary) == 0 {
		return ""
	}
	header := fmt.Sprintf("files=%d +%d -%d", r.displayFileCount(), r.additions, r.deletions)
	if r.shouldCondenseStatSummary() {
		return header + "\n" + strings.Join(r.renderCondensedSummary(), "\n")
	}
	if len(r.summary) == 0 {
		if len(r.patchFiles) > 0 {
			if r.verbatimRenderable() {
				return header + "\n" + strings.Join(r.renderVerbatimPatch(verbatimLineCost(header)), "\n")
			}
			return header + "\n" + strings.Join(r.renderPatchSummary(), "\n")
		}
		return header
	}
	return header + "\n" + strings.Join(r.summary, "\n")
}

func (r *GitDiffReducer) RecoveryInfo() (string, string, bool) {
	switch {
	case r.fileCount == 0 && len(r.patchFiles) == 0 && len(r.summary) == 0:
		return shared.NoRecovery()
	case len(r.patchFiles) > 0 && len(r.summary) == 0 && r.verbatimRenderable():
		return shared.FullOutputRecovery("omitted diff context lines")
	case len(r.patchFiles) > 0:
		return shared.FullOutputRecovery("omitted full diff hunks")
	case len(r.statEntries) > len(r.summary):
		return shared.FullOutputRecovery(fmt.Sprintf("omitted %d diff summary entries", len(r.statEntries)-len(r.summary)))
	default:
		return shared.NoRecovery()
	}
}

func (r *GitDiffReducer) displayFileCount() int {
	if r.fileCount > 0 {
		return r.fileCount
	}
	return len(r.statEntries)
}

func (r *GitDiffReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GitDiffReducer) recordLine(line string) {
	if strings.HasPrefix(line, "diff --git ") {
		r.fileCount++
		r.startPatchFile(line)
	} else if path, ok := combinedDiffPath(line); ok {
		r.fileCount++
		r.startCombinedPatchFile(path)
	}
	if r.handlePatchMetadata(line) {
		return
	}
	if isDiffFilenameMarker(line) {
		return
	}
	if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "@@@ ") {
		r.recordPatchAnchor(line)
		return
	}
	r.recordPatchDelta(line)
	r.recordAddedSnippet(line)
	r.captureVerbatimLine(line)
	if isDiffSummaryLine(line) {
		r.recordSummaryTotals(line)
		if entry, ok := parseDiffStatEntry(line); ok {
			r.statEntries = append(r.statEntries, entry)
		}
	}
	if len(r.summary) >= r.maxSummary {
		return
	}
	if isDiffSummaryLine(line) {
		r.summary = append(r.summary, line)
	}
}

func (r *GitDiffReducer) handlePatchMetadata(line string) bool {
	if r.currentPatch == nil {
		return false
	}
	switch {
	case strings.HasPrefix(line, "rename from "):
		r.currentPatch.IsRenamed = true
		r.currentPatch.OldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
		return true
	case strings.HasPrefix(line, "rename to "):
		r.currentPatch.IsRenamed = true
		r.currentPatch.NewPath = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
		if r.currentPatch.Path == "" {
			r.currentPatch.Path = r.currentPatch.NewPath
		}
		return true
	case strings.HasPrefix(line, "new file mode "):
		r.currentPatch.IsNew = true
		return true
	case strings.HasPrefix(line, "deleted file mode "):
		r.currentPatch.IsDeleted = true
		return true
	default:
		return false
	}
}

func isDiffFilenameMarker(line string) bool {
	return strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ")
}

func (r *GitDiffReducer) recordPatchDelta(line string) {
	switch classifyDiffDelta(line) {
	case 1:
		r.additions++
		if r.currentPatch != nil {
			r.currentPatch.Additions++
		}
	case -1:
		r.deletions++
		if r.currentPatch != nil {
			r.currentPatch.Deletions++
		}
	}
}

func classifyDiffDelta(line string) int {
	switch {
	case strings.HasPrefix(line, "+"):
		return 1
	case strings.HasPrefix(line, "-"):
		return -1
	default:
		return 0
	}
}

// gitDiffSnippet* bound the added-line snippets retained per file for
// inventory anchors: the first couple of changed lines are what make a file
// recognizable when the diff is too large to replay its hunks.
const (
	gitDiffSnippetMax     = 2
	gitDiffSnippetClipLen = 80
)

// recordAddedSnippet retains the first added content lines of each file as
// inventory anchors. Unlike verbatim capture this survives large diffs on
// purpose — the inventory render needs a recognizable snippet per file
// precisely when the diff is too big to replay.
func (r *GitDiffReducer) recordAddedSnippet(line string) {
	if r.currentPatch == nil || len(r.currentPatch.Snippets) >= gitDiffSnippetMax || classifyDiffDelta(line) != 1 {
		return
	}
	content := strings.TrimSpace(strings.TrimPrefix(line, "+"))
	if content == "" || strings.HasPrefix(content, "+") {
		return
	}
	r.currentPatch.Snippets = append(r.currentPatch.Snippets, "+"+shared.Clip(content, gitDiffSnippetClipLen))
}

// captureVerbatimLine retains the changed (+/-) lines of a still-small diff
// so the render can replay them verbatim. Context lines, index/---/+++ noise,
// and hunk headers never reach this point. Once the diff exceeds the verbatim
// caps all retained lines are dropped and the reducer stays in summary mode.
func (r *GitDiffReducer) captureVerbatimLine(line string) {
	if r.verbatimDisabled || r.currentPatch == nil || !isVerbatimDiffLine(line, r.currentPatch.IsConflicted) {
		return
	}
	if !r.consumeVerbatimBudget(line) {
		return
	}
	r.currentPatch.Lines = append(r.currentPatch.Lines, line)
}

// consumeVerbatimBudget charges one rendered line against the verbatim caps
// and disables verbatim mode (releasing retained lines) when they are
// exceeded.
func (r *GitDiffReducer) consumeVerbatimBudget(line string) bool {
	r.verbatimLines++
	r.verbatimBytes += len(line) + 1
	if r.verbatimLines > gitDiffVerbatimMaxLines || r.verbatimBytes > gitDiffVerbatimMaxBytes {
		r.disableVerbatim()
		return false
	}
	return true
}

func (r *GitDiffReducer) disableVerbatim() {
	r.verbatimDisabled = true
	for i := range r.patchFiles {
		r.patchFiles[i].Lines = nil
	}
}

// isVerbatimDiffLine reports whether a patch content line carries changed
// content worth replaying. Plain diffs mark changes with a single leading
// +/-; combined (conflict) diffs use two marker columns, so any line whose
// second column marks a change is kept as well.
func isVerbatimDiffLine(line string, conflicted bool) bool {
	if classifyDiffDelta(line) != 0 {
		return true
	}
	if !conflicted || len(line) < 2 {
		return false
	}
	first, second := line[0], line[1]
	return (first == ' ' || first == '+' || first == '-') && (second == '+' || second == '-')
}

func (r *GitDiffReducer) verbatimRenderable() bool {
	return !r.verbatimDisabled
}

// renderVerbatimPatch renders each file as its summary header followed by
// its +/- lines verbatim (headerTokens is the token cost of the `files=...`
// line emitted above the returned lines).
//
// When the engine compression contract is disarmed (small raw output) the
// full render is returned as-is, headers included with hunk counts and
// anchors. When the contract would cap the display, the render self-caps to
// the predicted allowance instead: the reducer knows what matters in a diff
// — every filename first, then as many changed lines as fit — while the
// generic token capper does not, so fitting the allowance here keeps the
// contract from crushing filenames in favor of identifier-dense content
// lines.
func (r *GitDiffReducer) renderVerbatimPatch(headerTokens int) []string {
	full := make([]string, 0, len(r.patchFiles)+r.verbatimLines)
	fullCost := 0
	for _, file := range r.patchFiles {
		header := formatPatchFileSummary(file)
		full = append(full, header)
		fullCost += verbatimLineCost(header)
		for _, line := range file.Lines {
			full = append(full, line)
			fullCost += verbatimLineCost(line)
		}
	}
	allowance := r.verbatimTokenAllowance()
	if allowance == 0 {
		return full
	}
	budget := allowance - headerTokens
	if fullCost <= budget {
		return full
	}
	return r.renderVerbatimPatchCapped(budget)
}

// renderVerbatimPatchCapped fits the verbatim render into the remaining
// token budget: label-only file headers first (hunk counts, churn, and
// anchors are substitutes for the content that follows and would crowd out
// filenames), then the cheapest changed lines first — under a hard token cap
// more surviving lines cover more of the diff — with a bare "..." marker
// when lines had to be dropped.
func (r *GitDiffReducer) renderVerbatimPatchCapped(budget int) []string {
	headers := make([]string, len(r.patchFiles))
	cost := 0
	for i, file := range r.patchFiles {
		headers[i] = patchFileLabel(file)
		cost += verbatimLineCost(headers[i])
	}
	kept, dropped := r.selectVerbatimLines(budget - cost)
	out := make([]string, 0, len(r.patchFiles)+r.verbatimLines+1)
	for i, file := range r.patchFiles {
		out = append(out, headers[i])
		for j, line := range file.Lines {
			if kept[i][j] {
				out = append(out, line)
			}
		}
	}
	if dropped > 0 {
		out = append(out, "...")
	}
	return out
}

const (
	// gitDiffVerbatimContractRawTokens is a conservative proxy for the
	// engine compression contract's 200-raw-token arming threshold. The
	// reducer only sees bytes, and the contract's lexical token estimate can
	// exceed the byte-based one, so the proxy arms a little early rather
	// than ever predicting "disarmed" for an armed display.
	gitDiffVerbatimContractRawTokens = 150
	// gitDiffVerbatimSuffixReserve leaves room for the recovery/tee suffix
	// the display finalizer appends inside the same contract allowance.
	gitDiffVerbatimSuffixReserve = 16
	// gitDiffVerbatimMinTokens mirrors the contract's 48-token usable floor.
	gitDiffVerbatimMinTokens = 48
)

// verbatimTokenAllowance predicts the engine compression contract's
// retained-token budget (1/5 of the raw tokens with a usable floor, minus
// the recovery-suffix reserve) using the bytes this reducer parsed as the
// raw-size signal. Returns 0 when the contract is predicted to stay
// disarmed, meaning the verbatim render needs no self-cap. The byte-based
// raw estimate never exceeds the contract's own, so a render within this
// allowance is never crushed downstream.
func (r *GitDiffReducer) verbatimTokenAllowance() int {
	rawTokens := (r.bytesParsed + 3) / 4
	if rawTokens < gitDiffVerbatimContractRawTokens {
		return 0
	}
	allowed := (rawTokens + 4) / 5
	if r.maxTokens > 0 && r.maxTokens < allowed {
		// The contract also caps at the profile budget's token allowance.
		allowed = r.maxTokens
	}
	if allowed < gitDiffVerbatimMinTokens {
		allowed = gitDiffVerbatimMinTokens
	}
	return allowed - gitDiffVerbatimSuffixReserve
}

// verbatimLineCost prices one rendered line, including its newline, so the
// per-line sum is a safe upper bound for the whole-render token estimate the
// compression contract measures.
func verbatimLineCost(line string) int {
	return history.EstimateTokens(line + "\n")
}

type verbatimLineCandidate struct{ file, idx, cost int }

// selectVerbatimLines keeps every changed line when they fit the remaining
// budget, and otherwise the cheapest lines first. Returns kept flags per
// file plus the number of dropped lines.
func (r *GitDiffReducer) selectVerbatimLines(budget int) ([][]bool, int) {
	kept := make([][]bool, len(r.patchFiles))
	for i, file := range r.patchFiles {
		kept[i] = make([]bool, len(file.Lines))
	}
	candidates, total := r.verbatimLineCandidates()
	if total <= budget {
		for _, item := range candidates {
			kept[item.file][item.idx] = true
		}
		return kept, 0
	}
	return kept, keepCheapestVerbatimLines(kept, candidates, budget-1) // -1 reserves the trailing "..." marker
}

// keepCheapestVerbatimLines marks the cheapest candidates that fit the
// budget as kept — under a hard token cap more surviving lines cover more of
// the diff — and returns how many were dropped.
func keepCheapestVerbatimLines(kept [][]bool, candidates []verbatimLineCandidate, budget int) int {
	sort.SliceStable(candidates, func(a, b int) bool {
		return candidates[a].cost < candidates[b].cost
	})
	dropped := 0
	for _, item := range candidates {
		if item.cost > budget {
			dropped++
			continue
		}
		kept[item.file][item.idx] = true
		budget -= item.cost
	}
	return dropped
}

func (r *GitDiffReducer) verbatimLineCandidates() ([]verbatimLineCandidate, int) {
	candidates := make([]verbatimLineCandidate, 0, r.verbatimLines)
	total := 0
	for i, file := range r.patchFiles {
		for j, line := range file.Lines {
			cost := verbatimLineCost(line)
			total += cost
			candidates = append(candidates, verbatimLineCandidate{file: i, idx: j, cost: cost})
		}
	}
	return candidates, total
}

func isDiffSummaryLine(line string) bool {
	return strings.Contains(line, "|") || strings.Contains(line, "files changed") || strings.Contains(line, "file changed")
}

func (r *GitDiffReducer) startPatchFile(line string) {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		r.currentPatch = nil
		return
	}
	left := strings.TrimPrefix(parts[2], "a/")
	right := strings.TrimPrefix(parts[3], "b/")
	path := right
	if path == "" || path == "/dev/null" {
		path = left
	}
	file := gitDiffPatchFile{
		Path:    path,
		OldPath: left,
		NewPath: right,
		Anchors: make([]string, 0, 3),
	}
	r.patchFiles = append(r.patchFiles, file)
	r.currentPatch = &r.patchFiles[len(r.patchFiles)-1]
	if !r.verbatimDisabled {
		// Each file costs one rendered header line in verbatim mode.
		r.consumeVerbatimBudget(path)
	}
}

// combinedDiffPath recognizes combined-diff headers (`diff --cc <path>` and
// `diff --combined <path>`) that git emits for unmerged, conflicted paths.
func combinedDiffPath(line string) (string, bool) {
	for _, prefix := range []string{"diff --cc ", "diff --combined "} {
		if strings.HasPrefix(line, prefix) {
			if path := strings.TrimSpace(strings.TrimPrefix(line, prefix)); path != "" {
				return path, true
			}
		}
	}
	return "", false
}

func (r *GitDiffReducer) startCombinedPatchFile(path string) {
	file := gitDiffPatchFile{
		Path:         path,
		IsConflicted: true,
		Anchors:      make([]string, 0, 3),
	}
	r.patchFiles = append(r.patchFiles, file)
	r.currentPatch = &r.patchFiles[len(r.patchFiles)-1]
	if !r.verbatimDisabled {
		r.consumeVerbatimBudget(path)
	}
}

func (r *GitDiffReducer) recordPatchAnchor(line string) {
	if r.currentPatch == nil {
		return
	}
	r.currentPatch.Hunks++
	anchor := parseDiffAnchor(line)
	if anchor == "" {
		return
	}
	for _, existing := range r.currentPatch.Anchors {
		if existing == anchor {
			return
		}
	}
	if len(r.currentPatch.Anchors) < 3 {
		r.currentPatch.Anchors = append(r.currentPatch.Anchors, anchor)
	}
}

func parseDiffAnchor(line string) string {
	idx := strings.LastIndex(line, "@@")
	if idx < 0 {
		return ""
	}
	anchor := strings.TrimSpace(line[idx+2:])
	if anchor == "" {
		return ""
	}
	return anchor
}

func (r *GitDiffReducer) renderPatchSummary() []string {
	visible := r.maxSummary
	if visible <= 0 {
		visible = 1
	}
	if len(r.patchFiles) > visible {
		// A diff that touches more files than the summary budget still has to
		// account for every file: the inventory render keeps each filename
		// discoverable instead of hiding the tail behind a bare count.
		return r.renderPatchInventory()
	}
	out := make([]string, 0, visible)
	for _, file := range r.patchFiles {
		out = append(out, formatPatchFileSummary(file))
	}
	return out
}

func patchFileLabel(file gitDiffPatchFile) string {
	label := file.Path
	switch {
	case file.IsConflicted:
		label = label + " [conflict]"
	case file.IsRenamed && file.OldPath != "" && file.NewPath != "":
		label = file.OldPath + " -> " + file.NewPath
	case file.IsDeleted:
		label = label + " [deleted]"
	case file.IsNew:
		label = label + " [new]"
	}
	return label
}

func formatPatchFileSummary(file gitDiffPatchFile) string {
	parts := []string{patchFileLabel(file)}
	if file.Hunks > 0 {
		parts = append(parts, fmt.Sprintf("hunks=%d", file.Hunks))
	}
	if file.Additions > 0 || file.Deletions > 0 {
		parts = append(parts, fmt.Sprintf("+%d -%d", file.Additions, file.Deletions))
	}
	if len(file.Anchors) > 0 {
		parts = append(parts, strings.Join(file.Anchors, " | "))
	}
	return strings.Join(parts, "  ")
}

type gitDiffStatEntry struct {
	Line    string
	Path    string
	Changes int
}

func (r *GitDiffReducer) shouldCondenseStatSummary() bool {
	if len(r.statEntries) == 0 {
		return false
	}
	if r.aggressive {
		return len(r.statEntries) > 3
	}
	return len(r.statEntries) > r.largeFileThreshold
}

func (r *GitDiffReducer) renderCondensedSummary() []string {
	entries := append([]gitDiffStatEntry(nil), r.statEntries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Changes == entries[j].Changes {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Changes > entries[j].Changes
	})
	limit := r.largeSummaryTopN
	if r.aggressive {
		limit = r.aggressiveSummaryTopN
	}
	if limit > r.maxSummary {
		limit = r.maxSummary
	}
	if limit <= 0 {
		limit = 1
	}
	if len(entries) < limit {
		limit = len(entries)
	}
	out := make([]string, 0, limit+1)
	for _, entry := range entries[:limit] {
		out = append(out, entry.Line)
	}
	if len(entries) > limit {
		out = append(out, fmt.Sprintf("... +%d more files", len(entries)-limit))
	}
	return out
}

func parseDiffStatEntry(line string) (gitDiffStatEntry, bool) {
	left, right, ok := strings.Cut(line, "|")
	if !ok {
		return gitDiffStatEntry{}, false
	}
	path := strings.TrimSpace(left)
	if path == "" {
		return gitDiffStatEntry{}, false
	}
	fields := strings.Fields(strings.TrimSpace(right))
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		return gitDiffStatEntry{
			Line:    strings.TrimSpace(line),
			Path:    path,
			Changes: value,
		}, true
	}
	return gitDiffStatEntry{}, false
}

func (r *GitDiffReducer) recordSummaryTotals(line string) {
	if !strings.Contains(line, "file changed") && !strings.Contains(line, "files changed") {
		return
	}
	fields := strings.Fields(strings.ReplaceAll(line, ",", " "))
	for i := 0; i < len(fields)-1; i++ {
		value, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "file", "files":
			if r.fileCount == 0 {
				r.fileCount = value
			}
		case "insertion(+)", "insertions(+)":
			if r.additions == 0 {
				r.additions = value
			}
		case "deletion(-)", "deletions(-)":
			if r.deletions == 0 {
				r.deletions = value
			}
		}
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
