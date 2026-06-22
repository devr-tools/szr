package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeGitAdd(inv engine.Invocation, input string) string {
	summary, ok, _ := summarizeGitAdd(inv, input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitCommit(inv engine.Invocation, input string) string {
	summary, ok, _ := summarizeGitCommit(inv, input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitPush(input string) string {
	summary, ok, _ := summarizeGitPush(input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitPull(input string) string {
	summary, ok, _ := summarizeGitPull(input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

type GitSuccessPathReducer struct {
	mode        string
	inv         engine.Invocation
	maxLines    int
	bytesParsed int
	stdoutScan  shared.LineScanner
	stderrScan  shared.LineScanner
	stdoutLines []string
	stderrLines []string
	finalized   bool
	summary     string
	matched     bool
	omitted     bool
}

func NewGitSuccessPathReducer(mode string, inv engine.Invocation, maxLines, _ int) *GitSuccessPathReducer {
	if maxLines <= 0 {
		maxLines = 6
	}
	return &GitSuccessPathReducer{
		mode:        mode,
		inv:         inv,
		maxLines:    maxLines,
		stdoutLines: make([]string, 0, maxLines),
		stderrLines: make([]string, 0, maxLines),
	}
}

func (r *GitSuccessPathReducer) ConsumeStdout(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stdoutScan.Consume(chunk, func(line string) {
		r.stdoutLines = append(r.stdoutLines, line)
	})
}

func (r *GitSuccessPathReducer) ConsumeStderr(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stderrScan.Consume(chunk, func(line string) {
		r.stderrLines = append(r.stderrLines, line)
	})
}

func (r *GitSuccessPathReducer) Result() string {
	r.finish()
	if r.matched {
		return r.summary
	}
	return shared.CompactLines(r.input(), r.maxLines)
}

func (r *GitSuccessPathReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GitSuccessPathReducer) FallbackUsed() bool {
	r.finish()
	return !r.matched
}

func (r *GitSuccessPathReducer) Preview() string {
	r.finish()
	if r.matched {
		return r.summary
	}
	return strings.TrimSpace(shared.JoinLimitedLines(r.lines(), r.maxLines))
}

func (r *GitSuccessPathReducer) RecoveryInfo() (string, string, bool) {
	r.finish()
	if !r.matched || !r.omitted {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted git %s success details", r.mode))
}

func (r *GitSuccessPathReducer) finish() {
	if r.finalized {
		return
	}
	r.stdoutScan.Finish(func(line string) {
		r.stdoutLines = append(r.stdoutLines, line)
	})
	r.stderrScan.Finish(func(line string) {
		r.stderrLines = append(r.stderrLines, line)
	})
	r.summary, r.matched, r.omitted = summarizeGitSuccess(r.mode, r.inv, r.input())
	r.finalized = true
}

func (r *GitSuccessPathReducer) lines() []string {
	lines := make([]string, 0, len(r.stdoutLines)+len(r.stderrLines))
	lines = append(lines, r.stdoutLines...)
	lines = append(lines, r.stderrLines...)
	return lines
}

func (r *GitSuccessPathReducer) input() string {
	return strings.Join(r.lines(), "\n")
}

func summarizeGitSuccess(mode string, inv engine.Invocation, input string) (string, bool, bool) {
	switch mode {
	case "add":
		return summarizeGitAdd(inv, input)
	case "commit":
		return summarizeGitCommit(inv, input)
	case "push":
		return summarizeGitPush(input)
	case "pull":
		return summarizeGitPull(input)
	default:
		return "", false, false
	}
}

func summarizeGitAdd(inv engine.Invocation, input string) (string, bool, bool) {
	if strings.TrimSpace(input) != "" {
		return "", false, false
	}
	pathspecs, broad := gitAddPathspecs(inv)
	switch {
	case broad:
		return "staged all changes", true, false
	case len(pathspecs) == 0:
		return "staged changes", true, false
	default:
		return "staged " + strings.Join(summarizePathBuckets(pathspecs), ", "), true, false
	}
}

//nolint:maintidx // Success-path commit parsing is kept linear so git output handling stays easy to audit.
func summarizeGitCommit(inv engine.Invocation, input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}

	lineIndex := -1
	hash := ""
	subject := ""
	for i, line := range lines {
		hash, subject = parseGitCommitHeadline(line)
		if hash != "" || subject != "" {
			lineIndex = i
			break
		}
	}
	if lineIndex == -1 {
		return "", false, false
	}

	files, additions, deletions := 0, 0, 0
	for _, line := range lines[lineIndex+1:] {
		if parsedFiles, parsedAdditions, parsedDeletions, ok := parseGitChangeTotals(line); ok {
			files, additions, deletions = parsedFiles, parsedAdditions, parsedDeletions
			break
		}
	}

	parts := []string{"committed"}
	if hash != "" {
		parts = append(parts, hash)
	}
	if subject != "" {
		parts = append(parts, subject)
	} else if fallbackSubject := parseGitCommitSubjectFromArgs(inv); fallbackSubject != "" {
		parts = append(parts, fallbackSubject)
	}
	if files > 0 {
		parts = append(parts, fmt.Sprintf("files=%d +%d -%d", files, additions, deletions))
	}
	return strings.Join(parts, " "), true, len(lines) > 1
}

//nolint:maintidx // Push output varies by remote and ref type, so the parser is intentionally explicit.
func summarizeGitPush(input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}

	if len(lines) == 1 && lines[0] == "Everything up-to-date" {
		return "push up-to-date", true, false
	}

	refSummaries := make([]string, 0, 2)
	ignored := 0
	for _, line := range lines {
		switch {
		case line == "Everything up-to-date":
			return "push up-to-date", true, len(lines) > 1
		case isGitPushNoise(line):
			ignored++
		default:
			summary, ok := parseGitPushRefSummary(line)
			if !ok {
				return "", false, false
			}
			refSummaries = append(refSummaries, summary)
		}
	}
	if len(refSummaries) == 0 {
		return "", false, false
	}
	if len(refSummaries) == 1 {
		return refSummaries[0], true, ignored > 0
	}
	return fmt.Sprintf("pushed %d refs", len(refSummaries)), true, true
}

func summarizeGitPull(input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}
	if summary, ok, ignored := summarizeGitPullUpToDate(lines); ok {
		return summary, true, ignored
	}
	state, ok := parseGitPullState(lines)
	if !ok || !state.usedSignal {
		return "", false, false
	}
	return buildGitPullSummary(state), true, state.ignored > 0 || len(lines) > minUsedGitPullLines(state.rangeText, state.mode, state.files)
}

type gitPullState struct {
	rangeText  string
	mode       string
	files      int
	additions  int
	deletions  int
	usedSignal bool
	ignored    int
}

func summarizeGitPullUpToDate(lines []string) (string, bool, bool) {
	if len(lines) == 1 && lines[0] == "Already up to date." {
		return "pull up-to-date", true, false
	}
	for _, line := range lines {
		if line == "Already up to date." {
			return "pull up-to-date", true, len(lines) > 1
		}
	}
	return "", false, false
}

func parseGitPullState(lines []string) (gitPullState, bool) {
	state := gitPullState{}
	for _, line := range lines {
		if parseGitPullSignal(line, &state) {
			continue
		}
		if isGitPullNoise(line) {
			state.ignored++
			continue
		}
		if !parseGitPullTotals(line, &state) {
			return gitPullState{}, false
		}
	}
	return state, true
}

func parseGitPullSignal(line string, state *gitPullState) bool {
	switch {
	case strings.HasPrefix(line, "Updating "):
		state.rangeText = strings.TrimSpace(strings.TrimPrefix(line, "Updating "))
	case line == "Fast-forward":
		state.mode = "fast-forward"
	case strings.HasPrefix(line, "Merge made by"):
		state.mode = "merge"
	case strings.HasPrefix(line, "Successfully rebased and updated "):
		state.mode = "rebase"
	default:
		return false
	}
	state.usedSignal = true
	return true
}

func parseGitPullTotals(line string, state *gitPullState) bool {
	parsedFiles, parsedAdditions, parsedDeletions, ok := parseGitChangeTotals(line)
	if !ok {
		return false
	}
	state.files = parsedFiles
	state.additions = parsedAdditions
	state.deletions = parsedDeletions
	state.usedSignal = true
	return true
}

func buildGitPullSummary(state gitPullState) string {
	parts := []string{"pulled"}
	if state.rangeText != "" {
		parts = append(parts, state.rangeText)
	}
	if state.mode != "" {
		parts = append(parts, state.mode)
	}
	if state.files > 0 {
		parts = append(parts, fmt.Sprintf("files=%d +%d -%d", state.files, state.additions, state.deletions))
	}
	return strings.Join(parts, " ")
}

//nolint:maintidx // Pathspec extraction needs to stay close to git's option semantics.
func gitAddPathspecs(inv engine.Invocation) ([]string, bool) {
	args := gitSubcommandArgs(inv)
	pathspecs := make([]string, 0, len(args))
	broad := false
	afterSeparator := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if afterSeparator {
			pathspecs = append(pathspecs, arg)
			if isBroadGitPathspec(arg) {
				broad = true
			}
			continue
		}
		switch {
		case arg == "--":
			afterSeparator = true
		case gitAddOptionConsumesValue(arg):
			i++
		case strings.HasPrefix(arg, "-"):
			if arg == "-A" || arg == "--all" || arg == "-u" || arg == "--update" {
				broad = true
			}
		default:
			pathspecs = append(pathspecs, arg)
			if isBroadGitPathspec(arg) {
				broad = true
			}
		}
	}
	return pathspecs, broad
}

func parseGitCommitHeadline(line string) (string, string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return "", ""
	}
	closeIdx := strings.Index(line, "]")
	if closeIdx <= 0 {
		return "", ""
	}
	head := strings.TrimSpace(line[1:closeIdx])
	subject := ""
	if closeIdx+1 < len(line) {
		subject = strings.TrimSpace(line[closeIdx+1:])
	}
	fields := strings.Fields(head)
	for i := len(fields) - 1; i >= 0; i-- {
		if looksLikeGitHash(fields[i]) {
			return fields[i], subject
		}
	}
	return "", subject
}

func parseGitCommitSubjectFromArgs(inv engine.Invocation) string {
	args := gitSubcommandArgs(inv)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-m" || arg == "--message":
			if i+1 < len(args) {
				return firstLine(args[i+1])
			}
		case strings.HasPrefix(arg, "--message="):
			return firstLine(strings.TrimPrefix(arg, "--message="))
		case strings.HasPrefix(arg, "-m") && len(arg) > 2:
			return firstLine(arg[2:])
		}
	}
	return ""
}

func parseGitPushRefSummary(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.Contains(trimmed, "[new branch]"):
		return parseGitPushCreatedRef(trimmed, "new branch")
	case strings.Contains(trimmed, "[new tag]"):
		return parseGitPushCreatedRef(trimmed, "new tag")
	case strings.Contains(trimmed, "->"):
		return parseGitPushUpdatedRef(trimmed)
	default:
		return "", false
	}
}

func parseGitPushCreatedRef(line string, kind string) (string, bool) {
	ref := gitPushArrowRef(line)
	if ref == "" {
		return "", false
	}
	return "pushed " + kind + " " + ref, true
}

func parseGitPushUpdatedRef(line string) (string, bool) {
	left := leftOfArrow(line)
	right := gitPushTargetRef(line)
	if right == "" {
		return "", false
	}
	rangeText := firstToken(left)
	if strings.Contains(line, "(forced update)") {
		return gitPushForcedSummary(right, rangeText), true
	}
	if isGitRevisionRange(rangeText) {
		return fmt.Sprintf("pushed %s %s", right, rangeText), true
	}
	return "pushed " + right, true
}

func gitPushArrowRef(line string) string {
	ref := gitPushTargetRef(line)
	if ref == "" {
		ref = leftOfArrow(line)
	}
	return ref
}

func gitPushTargetRef(line string) string {
	return firstToken(strings.TrimSpace(strings.TrimSuffix(rightOfArrow(line), "(forced update)")))
}

func gitPushForcedSummary(ref string, rangeText string) string {
	if isGitRevisionRange(rangeText) {
		return fmt.Sprintf("force-pushed %s %s", ref, rangeText)
	}
	return "force-pushed " + ref
}

func parseGitChangeTotals(line string) (int, int, int, bool) {
	if !strings.Contains(line, "file changed") && !strings.Contains(line, "files changed") {
		return 0, 0, 0, false
	}
	files, additions, deletions := 0, 0, 0
	fields := strings.Fields(strings.ReplaceAll(line, ",", " "))
	for i := 0; i < len(fields)-1; i++ {
		value, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "file", "files":
			files = value
		case "insertion(+)", "insertions(+)":
			additions = value
		case "deletion(-)", "deletions(-)":
			deletions = value
		}
	}
	return files, additions, deletions, files > 0
}

func gitSubcommandArgs(inv engine.Invocation) []string {
	args := engine.CanonicalArgsForClassification(inv.Command)
	if len(args) < 2 || args[0] != "git" {
		args = engine.CanonicalArgsForClassification(inv.Display)
	}
	if len(args) < 2 || args[0] != "git" {
		return nil
	}
	return args[2:]
}

func gitAddOptionConsumesValue(arg string) bool {
	switch arg {
	case "--chmod", "--pathspec-from-file":
		return true
	default:
		return false
	}
}

func isBroadGitPathspec(arg string) bool {
	switch arg {
	case ".", ":/", ":.", "*":
		return true
	default:
		return false
	}
}

func isGitPushNoise(line string) bool {
	return strings.HasPrefix(line, "To ") ||
		strings.HasPrefix(line, "branch '") ||
		strings.HasPrefix(line, "Enumerating objects:") ||
		strings.HasPrefix(line, "Counting objects:") ||
		strings.HasPrefix(line, "Compressing objects:") ||
		strings.HasPrefix(line, "Writing objects:") ||
		strings.HasPrefix(line, "Total ") ||
		strings.HasPrefix(line, "remote:")
}

func isGitPullNoise(line string) bool {
	return strings.HasPrefix(line, "From ") ||
		strings.HasPrefix(line, " * branch") ||
		strings.HasPrefix(line, " = [up to date]") ||
		strings.Contains(line, "-> origin/") ||
		strings.HasPrefix(line, "remote:")
}

func minUsedGitPullLines(rangeText, mode string, files int) int {
	used := 0
	if rangeText != "" {
		used++
	}
	if mode != "" {
		used++
	}
	if files > 0 {
		used++
	}
	if used == 0 {
		return 1
	}
	return used
}

func rightOfArrow(line string) string {
	parts := strings.SplitN(line, "->", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func leftOfArrow(line string) string {
	parts := strings.SplitN(line, "->", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func firstToken(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func firstLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func looksLikeGitHash(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func isGitRevisionRange(value string) bool {
	parts := strings.SplitN(value, "..", 2)
	return len(parts) == 2 && looksLikeGitHash(parts[0]) && looksLikeGitHash(parts[1])
}
