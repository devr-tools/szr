package git

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

const (
	gitOpMaxRefPreview   = 5
	gitOpMaxHintPreview  = 4
	gitOpMaxOtherStored  = 32
	gitOpAutoMergePathsN = 3
)

// SummarizeGitOp condenses noisy transfer-style git subcommand output
// (fetch, clone, merge, rebase, checkout, switch, reset, stash, cherry-pick)
// into ref/branch signal, conflict lists, and error lines while suppressing
// progress meters and remote chatter.
func SummarizeGitOp(kind string, input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}
	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return ""
	}
	state := &gitOpState{kind: kind}
	for _, line := range lines {
		state.record(strings.TrimSpace(line))
	}
	return state.render(maxLines)
}

func GitOpRecoveryInfo(kind string, input string, maxLines int) (string, string, bool) {
	rawLines := shared.NonEmptyLines(shared.StripANSI(input))
	renderedLines := shared.NonEmptyLines(SummarizeGitOp(kind, input, maxLines))
	if len(rawLines) == 0 || len(renderedLines) >= len(rawLines) {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted git %s progress and detail lines", kind))
}

// SummarizeGitBranches compacts long branch listings into a grouped preview
// while leaving short listings untouched.
func SummarizeGitBranches(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}
	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return ""
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	current, names := parseGitBranchNames(lines)
	header := fmt.Sprintf("%d branches", len(names))
	if current != "" {
		header += " (current: " + current + ")"
	}
	buckets := summarizePathBuckets(names)
	body := shared.JoinLimitedLines(buckets, maxLines-1)
	return header + "\n" + body
}

func parseGitBranchNames(lines []string) (string, []string) {
	current := ""
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isCurrent := strings.HasPrefix(trimmed, "* ")
		trimmed = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "* "), "+ "))
		name := firstToken(trimmed)
		if name == "" {
			continue
		}
		if isCurrent {
			current = name
		}
		names = append(names, name)
	}
	return current, names
}

type gitOpEntry struct {
	kind string
	text string
	must bool
}

type gitOpState struct {
	kind            string
	entries         []gitOpEntry
	autoMergePaths  []string
	autoMergeTotal  int
	refPreview      []string
	refTotal        int
	hintCount       int
	hintExtra       int
	otherStored     int
	otherExtra      int
	progressDropped int
	remoteDropped   int
	objectTotal     string
	objectSize      string
	deltaTotal      string
	objectsQueued   bool
}

func (s *gitOpState) record(line string) {
	if line == "" {
		return
	}
	if body, ok := strings.CutPrefix(line, "remote:"); ok {
		s.recordRemote(strings.TrimSpace(body))
		return
	}
	s.recordLocal(line)
}

func (s *gitOpState) recordRemote(body string) {
	switch {
	case body == "":
		s.remoteDropped++
	case isGitOpErrorLine(body):
		s.addEntry("remote: "+body, true)
	case s.recordProgress(body):
	default:
		s.remoteDropped++
	}
}

func (s *gitOpState) recordLocal(line string) {
	switch {
	case strings.HasPrefix(line, "CONFLICT"):
		s.addEntry(line, true)
	case isGitOpErrorLine(line):
		s.addEntry(line, true)
	case strings.HasPrefix(line, "hint:"):
		s.recordHint(line)
	case s.recordProgress(line):
	case strings.HasPrefix(line, "Auto-merging "):
		s.recordAutoMerge(strings.TrimSpace(strings.TrimPrefix(line, "Auto-merging ")))
	case isGitOpRefLine(line):
		s.recordRef(line)
	case isGitOpSignalLine(line):
		s.addEntry(line, true)
	default:
		s.recordOther(line)
	}
}

func (s *gitOpState) recordHint(line string) {
	if s.hintCount >= gitOpMaxHintPreview {
		s.hintExtra++
		return
	}
	s.hintCount++
	s.addEntry(line, true)
}

func (s *gitOpState) recordOther(line string) {
	if s.otherStored >= gitOpMaxOtherStored {
		s.otherExtra++
		return
	}
	s.otherStored++
	s.addEntry(line, false)
}

func (s *gitOpState) recordAutoMerge(path string) {
	s.autoMergeTotal++
	if s.autoMergeTotal == 1 {
		s.entries = append(s.entries, gitOpEntry{kind: "automerge"})
	}
	if path != "" && len(s.autoMergePaths) < gitOpAutoMergePathsN {
		s.autoMergePaths = append(s.autoMergePaths, path)
	}
}

func (s *gitOpState) recordRef(line string) {
	s.refTotal++
	if s.refTotal == 1 {
		s.entries = append(s.entries, gitOpEntry{kind: "refs"})
	}
	if len(s.refPreview) < gitOpMaxRefPreview {
		s.refPreview = append(s.refPreview, condenseSpaces(line))
	}
}

func (s *gitOpState) recordProgress(line string) bool {
	prefix := gitOpProgressPrefix(line)
	if prefix == "" {
		return false
	}
	s.progressDropped++
	switch prefix {
	case "Receiving objects", "Unpacking objects", "Writing objects":
		s.captureObjectCounts(line)
	case "Resolving deltas":
		if total := parenTotal(line); total != "" {
			s.deltaTotal = total
		}
	}
	if !s.objectsQueued {
		s.objectsQueued = true
		s.entries = append(s.entries, gitOpEntry{kind: "objects"})
	}
	return true
}

func (s *gitOpState) captureObjectCounts(line string) {
	if total := parenTotal(line); total != "" {
		s.objectTotal = total
	}
	if size := gitOpTransferSize(line); size != "" {
		s.objectSize = size
	}
}

func (s *gitOpState) addEntry(text string, must bool) {
	s.entries = append(s.entries, gitOpEntry{kind: "text", text: text, must: must})
}

func (s *gitOpState) render(maxLines int) string {
	lines := s.expand()
	if len(lines) == 0 {
		return fmt.Sprintf("git %s: no output", s.kind)
	}
	return joinGitOpLines(lines, maxLines)
}

func (s *gitOpState) expand() []gitOpEntry {
	out := make([]gitOpEntry, 0, len(s.entries)+2)
	for _, entry := range s.entries {
		out = append(out, s.expandEntry(entry)...)
	}
	if s.hintExtra > 0 {
		out = append(out, gitOpEntry{kind: "text", text: fmt.Sprintf("... +%d more hints", s.hintExtra), must: true})
	}
	return out
}

func (s *gitOpState) expandEntry(entry gitOpEntry) []gitOpEntry {
	switch entry.kind {
	case "automerge":
		return s.expandAutoMerge()
	case "refs":
		return s.expandRefs()
	case "objects":
		return s.expandObjects()
	default:
		return []gitOpEntry{entry}
	}
}

func (s *gitOpState) expandAutoMerge() []gitOpEntry {
	if s.autoMergeTotal == 0 {
		return nil
	}
	text := "Auto-merging: " + strings.Join(s.autoMergePaths, ", ")
	if s.autoMergeTotal > len(s.autoMergePaths) {
		text += fmt.Sprintf(" (+%d more)", s.autoMergeTotal-len(s.autoMergePaths))
	}
	return []gitOpEntry{{kind: "text", text: text}}
}

func (s *gitOpState) expandRefs() []gitOpEntry {
	out := make([]gitOpEntry, 0, len(s.refPreview)+1)
	for _, ref := range s.refPreview {
		out = append(out, gitOpEntry{kind: "text", text: ref})
	}
	if s.refTotal > len(s.refPreview) {
		out = append(out, gitOpEntry{kind: "text", text: fmt.Sprintf("... +%d more refs", s.refTotal-len(s.refPreview))})
	}
	return out
}

func (s *gitOpState) expandObjects() []gitOpEntry {
	line := s.objectsLine()
	if line == "" {
		return nil
	}
	return []gitOpEntry{{kind: "text", text: line}}
}

func (s *gitOpState) objectsLine() string {
	parts := []string{}
	if s.objectTotal != "" {
		text := "objects: " + s.objectTotal
		if s.objectSize != "" {
			text += " (" + s.objectSize + ")"
		}
		parts = append(parts, text)
	}
	if s.deltaTotal != "" {
		parts = append(parts, "deltas: "+s.deltaTotal)
	}
	return strings.Join(parts, ", ")
}

func countGitOpMustEntries(entries []gitOpEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.must {
			count++
		}
	}
	return count
}

func joinGitOpLines(entries []gitOpEntry, maxLines int) string {
	budget := maxLines - countGitOpMustEntries(entries)
	out := make([]string, 0, minGitOpInt(len(entries), maxLines)+1)
	skipped := 0
	for _, entry := range entries {
		if entry.must {
			out = append(out, entry.text)
			continue
		}
		if budget > 0 {
			out = append(out, entry.text)
			budget--
			continue
		}
		skipped++
	}
	if skipped > 0 {
		out = append(out, fmt.Sprintf("... +%d more lines", skipped))
	}
	return strings.Join(out, "\n")
}

func isGitOpErrorLine(line string) bool {
	for _, prefix := range []string{
		"error:", "fatal:", "warning:", "ERROR:",
		"Automatic merge failed", "Could not apply ",
		"You are in 'detached HEAD'",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func gitOpProgressPrefix(line string) string {
	for _, prefix := range []string{
		"Enumerating objects", "Counting objects", "Compressing objects",
		"Receiving objects", "Resolving deltas", "Writing objects",
		"Unpacking objects", "Updating files", "Checking out files",
		"Checking connectivity", "Total ",
	} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSuffix(prefix, " ")
		}
	}
	return ""
}

func isGitOpRefLine(line string) bool {
	return strings.Contains(line, " -> ")
}

func isGitOpSignalLine(line string) bool {
	if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
		return true
	}
	for _, prefix := range []string{
		"From ", "Cloning into", "Fast-forward", "Updating ", "Merge made by",
		"Already up to date", "Already up-to-date", "Switched to ", "Your branch",
		"HEAD is now at ", "Saved working directory", "Dropped ", "Deleted branch",
		"Renamed branch", "Note: switching to", "Previous HEAD position",
		"Successfully rebased", "branch '", "stash@{", "[",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func parenTotal(line string) string {
	open := strings.Index(line, "(")
	closing := strings.Index(line, ")")
	if open < 0 || closing < open {
		return ""
	}
	inner := line[open+1 : closing]
	if idx := strings.Index(inner, "/"); idx >= 0 {
		inner = inner[idx+1:]
	}
	return strings.TrimSpace(inner)
}

func gitOpTransferSize(line string) string {
	closing := strings.Index(line, ")")
	if closing < 0 {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[closing+1:]), ","))
	if idx := strings.IndexAny(rest, "|,"); idx >= 0 {
		rest = rest[:idx]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.HasPrefix(rest, "done") {
		return ""
	}
	return rest
}

func condenseSpaces(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

func minGitOpInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
