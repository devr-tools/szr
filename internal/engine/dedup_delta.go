package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

// Delta rendering is the edit-test-loop companion to session dedup: when a
// command's output CHANGED since the previous run (same command, directory,
// scope, and exit code, within the dedup window), the run can render as a
// compact change digest against the stored baseline artifact instead of the
// full summary. The digest is adopted only when it is strictly cheaper than
// the render it replaces, and it can never drop a changed line the retention
// verifier classifies as critical — a newly-failing test is exactly the line
// the digest exists to surface. Byte-identical reruns never reach this path:
// plain dedup references always take precedence.

const (
	// deltaMaxDiffLines bounds the line diff on both sides. Past this the
	// diff maps stop being allocation-cheap and the digest stops being a
	// plausible win, so the run bails to the normal render.
	deltaMaxDiffLines = 20000
	// deltaMaxDigestLines caps how many changed lines the digest itself
	// shows. Critical added lines are exempt: a digest that cannot afford
	// every critical addition is not emitted at all.
	deltaMaxDigestLines = 24
)

// deltaRenderOutcome renders the run as a change digest against the previous
// run's artifact when that is strictly cheaper than the normal render. The
// bool reports whether the digest was adopted; on false the caller keeps the
// normal render and the freshly stored entry unchanged.
func deltaRenderOutcome(store *dedup.Store, in sessionDedupInput, source dedupRawSource, now time.Time) (sessionDedupOutcome, bool) {
	if !in.inv.Advanced.DeltaRender || source.truncated {
		return sessionDedupOutcome{}, false
	}
	prev, baseline, ok := deltaBaseline(store, in, source, now)
	if !ok {
		return sessionDedupOutcome{}, false
	}
	changes, ok := computeDeltaChanges(baseline, string(source.stored))
	if !ok || (len(changes.added) == 0 && len(changes.removed) == 0) {
		return sessionDedupOutcome{}, false
	}
	digest, ok := buildDeltaDigest(in.rendered, prev, changes, now)
	if !ok {
		return sessionDedupOutcome{}, false
	}
	return sessionDedupOutcome{rendered: digest, deltaRef: prev.Ref()}, true
}

// deltaBaseline resolves the newest same-command entry in the same directory
// and scope within the dedup window, and returns its verified artifact text.
// The newest entry is the honest baseline regardless of how it exited, so an
// exit-code mismatch disqualifies the digest rather than reaching further
// back; a truncated baseline can never be diffed honestly.
func deltaBaseline(store *dedup.Store, in sessionDedupInput, source dedupRawSource, now time.Time) (dedup.Entry, string, bool) {
	since := now.Add(-sessionDedupWindow(in.inv.Advanced.SessionDedupWindowMinutes))
	entries, err := store.CommandMatches(history.Fingerprint(in.commandText), in.inv.Cwd, sessionScope(), since)
	if err != nil || len(entries) == 0 {
		return dedup.Entry{}, "", false
	}
	prev := entries[0]
	if prev.ExitCode != in.exitCode || prev.Truncated || prev.RawHash == source.hash {
		return dedup.Entry{}, "", false
	}
	data, err := store.ReadArtifact(prev)
	if err != nil || dedup.HashBytes(data) != prev.ArtifactHash {
		return dedup.Entry{}, "", false
	}
	return prev, string(data), true
}

// buildDeltaDigest assembles the digest render: a context header carrying the
// change counts and the baseline expansion ref, then the changed lines
// prefixed +/-. The cheapness test is honest about the WHOLE diff: when the
// full set of changed lines would cost at least as much as the render it
// replaces, the run is a rewrite rather than an edit and the digest is not
// emitted at all — truncating the diff to squeak under the render would trade
// fidelity for a marginal saving. The line cap only compacts diffs that
// already won on cost, and it can never evict a critical added line.
func buildDeltaDigest(rendered string, prev dedup.Entry, changes deltaChanges, now time.Time) (string, bool) {
	header := fmt.Sprintf(
		"since last run (%s ago): +%d -%d lines [baseline: szr expand %s]",
		formatDedupAge(now.Sub(prev.Timestamp)),
		len(changes.added),
		len(changes.removed),
		prev.Ref(),
	)
	renderedTokens := history.EstimateTokens(rendered)
	candidates, criticalCount := deltaDigestCandidates(changes)
	if deltaDigestFullCost(header, candidates) >= renderedTokens {
		return "", false
	}
	selected, omitted := filters.FitPriorityLines(candidates, deltaDigestLineCap(criticalCount), 0)
	digest := assembleDeltaDigest(header, selected, omitted)
	if history.EstimateTokens(digest) >= renderedTokens {
		return "", false
	}
	return digest, true
}

// deltaDigestFullCost prices the digest as if every changed line were shown.
func deltaDigestFullCost(header string, candidates []filters.PriorityLine) int {
	cost := filters.LineTokenCost(header)
	for _, candidate := range candidates {
		cost += filters.LineTokenCost(candidate.Text)
	}
	return cost
}

// deltaDigestLineCap keeps the digest compact without ever letting the line
// cap evict a critical added line; the token-side exemption for tier 0 is
// built into the priority fitter.
func deltaDigestLineCap(criticalCount int) int {
	if criticalCount > deltaMaxDigestLines {
		return criticalCount
	}
	return deltaMaxDigestLines
}

// deltaDigestCandidates orders the changed lines for the digest: critical
// added lines first (tier 0, never dropped — for a failing rerun these are
// the newly-failing identifiers), then the remaining added lines, then
// removals. A diff is a set of changes rather than a patch, so leading with
// the critical additions costs nothing in fidelity.
func deltaDigestCandidates(changes deltaChanges) ([]filters.PriorityLine, int) {
	candidates := make([]filters.PriorityLine, 0, len(changes.added)+len(changes.removed))
	critical := 0
	for _, line := range changes.added {
		trimmed := strings.TrimSpace(line)
		if isCriticalRetentionLine(trimmed, strings.ToLower(trimmed)) {
			candidates = append(candidates, filters.PriorityLine{Text: "+" + line, Tier: 0})
			critical++
		}
	}
	for _, line := range changes.added {
		trimmed := strings.TrimSpace(line)
		if !isCriticalRetentionLine(trimmed, strings.ToLower(trimmed)) {
			candidates = append(candidates, filters.PriorityLine{Text: "+" + line, Tier: 1})
		}
	}
	for _, line := range changes.removed {
		candidates = append(candidates, filters.PriorityLine{Text: "-" + line, Tier: 2})
	}
	return candidates, critical
}

func assembleDeltaDigest(header string, selected []string, omitted int) string {
	lines := make([]string, 0, len(selected)+2)
	lines = append(lines, header)
	lines = append(lines, selected...)
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("... +%d more changes", omitted))
	}
	return strings.Join(lines, "\n")
}

// deltaChanges holds the line-level difference between the baseline artifact
// and the new raw output: added lines exist in the new output only, removed
// lines in the baseline only, each in stream order.
type deltaChanges struct {
	added   []string
	removed []string
}

// computeDeltaChanges runs a bounded histogram line diff: trim the common
// prefix and suffix, then count the middle lines so each occurrence in one
// stream consumes one occurrence in the other. Single pass per side, two
// count maps, no quadratic LCS table — reruns in an edit-test loop overlap
// almost entirely, so the trimmed middle stays small.
func computeDeltaChanges(oldText string, newText string) (deltaChanges, bool) {
	oldLines := splitDeltaLines(oldText)
	newLines := splitDeltaLines(newText)
	if len(oldLines) > deltaMaxDiffLines || len(newLines) > deltaMaxDiffLines {
		return deltaChanges{}, false
	}
	oldMid, newMid := trimCommonDeltaEdges(oldLines, newLines)
	return deltaChanges{
		added:   excessDeltaLines(newMid, oldMid),
		removed: excessDeltaLines(oldMid, newMid),
	}, true
}

func splitDeltaLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func trimCommonDeltaEdges(a []string, b []string) ([]string, []string) {
	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	return a[start:endA], b[start:endB]
}

// excessDeltaLines returns the lines of primary not matched one-for-one by
// lines of other, in primary order.
func excessDeltaLines(primary []string, other []string) []string {
	if len(primary) == 0 {
		return nil
	}
	counts := make(map[string]int, len(other))
	for _, line := range other {
		counts[line]++
	}
	var excess []string
	for _, line := range primary {
		if counts[line] > 0 {
			counts[line]--
			continue
		}
		excess = append(excess, line)
	}
	return excess
}
