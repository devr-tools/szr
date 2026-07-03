package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/internal/history"
)

const (
	// dedupMinRenderTokens keeps tiny renders out of session dedup: below
	// this size a reference line costs about as much as the render itself.
	dedupMinRenderTokens = 48
	// dedupFailureHeadFloor is the minimum number of failure lines a dedup
	// reference keeps above the reference for a failing command, so an agent
	// never needs expansion to see WHAT failed.
	dedupFailureHeadFloor = 3
)

// sessionDedupInput carries everything the dedup decision needs after the
// full render pipeline (including the retention verifier) has finished.
type sessionDedupInput struct {
	rendered        string
	rawCombined     string
	rawCapturePath  string
	captureComplete bool
	exitCode        int
	failureExit     bool
	passthrough     bool
	inv             Invocation
	budget          OutputBudget
	commandText     string
}

type sessionDedupOutcome struct {
	rendered string
	// ref is the short reference emitted in place of the render, empty when
	// dedup did not fire.
	ref string
	// deltaRef is the baseline reference behind a delta digest render, empty
	// when delta rendering did not fire.
	deltaRef string
}

// sessionScope resolves the dedup/delta namespace for this run from the
// ScopeEnv environment variable; blank means the machine scope.
func sessionScope() string {
	return strings.TrimSpace(os.Getenv(dedup.ScopeEnv))
}

// sessionDedupWantsRawCapture reports whether the run should retain its tee
// capture stream for session dedup hashing and archival.
func sessionDedupWantsRawCapture(inv Invocation, passthrough bool) bool {
	return inv.Advanced.SessionDedup && !passthrough && !inv.UltraCompact
}

func sessionDedupWindow(minutes int) time.Duration {
	if minutes <= 0 {
		minutes = config.DefaultSessionDedupWindowMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// applySessionDedup fires only when the same command (fingerprint plus
// working directory) recently exited the same way with byte-identical raw
// output, the render is worth referencing, and the referenced artifact still
// verifies on disk. It also records the current run so a following identical
// run can reference it.
func (e *Engine) applySessionDedup(in sessionDedupInput) sessionDedupOutcome {
	source, ok := e.sessionDedupSource(in)
	if !ok {
		return sessionDedupOutcome{rendered: in.rendered}
	}
	store := dedup.New(e.paths.DataDir)
	now := time.Now()
	matches := recentDedupMatches(store, in, source, now)
	out, entry, ok := dedupOutcomeAndEntry(store, in, source, matches, now)
	if !ok {
		return sessionDedupOutcome{rendered: in.rendered}
	}
	_ = store.Append(entry)
	return out
}

// sessionDedupSource gates dedup eligibility and resolves the raw stream to
// hash: dedup must be enabled, the render must be worth referencing, and a
// complete raw source must exist.
func (e *Engine) sessionDedupSource(in sessionDedupInput) (dedupRawSource, bool) {
	if !sessionDedupWantsRawCapture(in.inv, in.passthrough) || e.paths.DataDir == "" {
		return dedupRawSource{}, false
	}
	if history.EstimateTokens(in.rendered) < dedupMinRenderTokens {
		return dedupRawSource{}, false
	}
	return loadDedupRawSource(in.rawCapturePath, in.rawCombined, in.captureComplete)
}

func recentDedupMatches(store *dedup.Store, in sessionDedupInput, source dedupRawSource, now time.Time) []dedup.Entry {
	since := now.Add(-sessionDedupWindow(in.inv.Advanced.SessionDedupWindowMinutes))
	matches, err := store.Matches(dedupKeyForRun(in, source), since)
	if err != nil {
		return nil
	}
	return matches
}

// dedupOutcomeAndEntry decides between emitting a reference to a verified
// previous run and recording this run as a fresh reference target. A match
// whose artifact is missing or corrupt falls through to the fresh-artifact
// path so a dangling reference is never emitted. Fresh runs whose output
// merely changed (rather than repeated) may render as a delta digest against
// the previous run's artifact; byte-identical dedup always takes precedence.
func dedupOutcomeAndEntry(
	store *dedup.Store,
	in sessionDedupInput,
	source dedupRawSource,
	matches []dedup.Entry,
	now time.Time,
) (sessionDedupOutcome, dedup.Entry, bool) {
	entry := dedupEntryForRun(in, source, now)
	if len(matches) > 0 && store.VerifyArtifact(matches[0]) {
		adoptDedupArtifact(&entry, matches[0])
		return dedupReferenceOutcome(in, matches, now), entry, true
	}
	if !storeDedupArtifact(store, &entry, source) {
		return sessionDedupOutcome{rendered: in.rendered}, dedup.Entry{}, false
	}
	if outcome, ok := deltaRenderOutcome(store, in, source, now); ok {
		return outcome, entry, true
	}
	return sessionDedupOutcome{rendered: in.rendered}, entry, true
}

func dedupKeyForRun(in sessionDedupInput, source dedupRawSource) dedup.Key {
	return dedup.Key{
		CommandFingerprint: history.Fingerprint(in.commandText),
		Cwd:                in.inv.Cwd,
		ExitCode:           in.exitCode,
		RawHash:            source.hash,
		Scope:              sessionScope(),
	}
}

func dedupEntryForRun(in sessionDedupInput, source dedupRawSource, now time.Time) dedup.Entry {
	return dedup.Entry{
		Timestamp:          now,
		RawHash:            source.hash,
		Command:            in.commandText,
		CommandFingerprint: history.Fingerprint(in.commandText),
		Cwd:                in.inv.Cwd,
		ExitCode:           in.exitCode,
		RawBytes:           source.rawBytes,
		Truncated:          source.truncated,
		Scope:              sessionScope(),
	}
}

// adoptDedupArtifact points the new entry at the verified artifact the
// previous identical run stored; byte-identical raw output never needs a
// second copy on disk.
func adoptDedupArtifact(entry *dedup.Entry, prev dedup.Entry) {
	entry.ArtifactPath = prev.ArtifactPath
	entry.ArtifactHash = prev.ArtifactHash
	entry.Truncated = prev.Truncated
}

func storeDedupArtifact(store *dedup.Store, entry *dedup.Entry, source dedupRawSource) bool {
	path, hash, err := store.WriteArtifact(source.stored)
	if err != nil {
		return false
	}
	entry.ArtifactPath = path
	entry.ArtifactHash = hash
	return true
}

// dedupReferenceOutcome builds the reference render and only adopts it when
// it is strictly cheaper than the render it replaces.
func dedupReferenceOutcome(in sessionDedupInput, matches []dedup.Entry, now time.Time) sessionDedupOutcome {
	prev := matches[0]
	reference := buildDedupReferenceRender(
		in.rendered,
		prev.Ref(),
		now.Sub(prev.Timestamp),
		len(matches)+1,
		in.failureExit,
		in.budget.MinFailures,
	)
	if history.EstimateTokens(reference) >= history.EstimateTokens(in.rendered) {
		return sessionDedupOutcome{rendered: in.rendered}
	}
	return sessionDedupOutcome{rendered: reference, ref: prev.Ref()}
}

func buildDedupReferenceRender(rendered string, ref string, age time.Duration, count int, failureExit bool, minFailures int) string {
	line := fmt.Sprintf(
		"unchanged from previous run (%s ago, x%d identical) [ref: %s - expand with: szr expand %s]",
		formatDedupAge(age),
		count,
		ref,
		ref,
	)
	head := dedupHeadLines(rendered, failureExit, minFailures)
	if len(head) == 0 {
		return line
	}
	return strings.Join(head, "\n") + "\n" + line
}

// dedupHeadLines keeps the orienting slice of the original render above the
// reference: the first line for successes, and the critical failure lines
// (up to the MinFailures floor) for failing exits.
func dedupHeadLines(rendered string, failureExit bool, minFailures int) []string {
	lines := nonEmptyRenderLines(rendered)
	if len(lines) == 0 {
		return nil
	}
	if !failureExit {
		return lines[:1]
	}
	max := minFailures
	if max < dedupFailureHeadFloor {
		max = dedupFailureHeadFloor
	}
	critical := criticalRenderLines(lines, max)
	if len(critical) == 0 {
		return lines[:1]
	}
	return critical
}

func nonEmptyRenderLines(rendered string) []string {
	raw := strings.Split(rendered, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func criticalRenderLines(lines []string, max int) []string {
	critical := make([]string, 0, max)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isCriticalRetentionLine(trimmed, strings.ToLower(trimmed)) {
			continue
		}
		critical = append(critical, line)
		if len(critical) >= max {
			break
		}
	}
	return critical
}

func formatDedupAge(age time.Duration) string {
	switch {
	case age < time.Second:
		return "0s"
	case age < time.Minute:
		return fmt.Sprintf("%ds", int(age.Seconds()))
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(age.Hours()))
	}
}

// dedupRawSource is the raw stream the dedup key hashes: the full-output
// capture file when the run retained one, otherwise the complete in-memory
// capture. stored holds the (cap-limited) bytes to archive for expansion.
type dedupRawSource struct {
	hash      string
	stored    []byte
	rawBytes  int64
	truncated bool
}

func loadDedupRawSource(capturePath string, rawCombined string, captureComplete bool) (dedupRawSource, bool) {
	if capturePath != "" {
		if source, ok := dedupSourceFromFile(capturePath); ok {
			return source, true
		}
	}
	// A preview-truncated in-memory capture would hash only the head of the
	// stream; two runs that agree on the head but diverge later must never
	// dedup, so an incomplete capture disqualifies the run entirely.
	if captureComplete && rawCombined != "" {
		return dedupSourceFromBytes([]byte(rawCombined)), true
	}
	return dedupRawSource{}, false
}

func dedupSourceFromBytes(data []byte) dedupRawSource {
	stored := data
	truncated := false
	if len(stored) > dedup.MaxArtifactBytes {
		stored = stored[:dedup.MaxArtifactBytes]
		truncated = true
	}
	return dedupRawSource{
		hash:      dedup.HashBytes(data),
		stored:    stored,
		rawBytes:  int64(len(data)),
		truncated: truncated,
	}
}

// dedupSourceFromFile hashes the full capture stream while buffering only
// the first MaxArtifactBytes for archival.
func dedupSourceFromFile(path string) (dedupRawSource, bool) {
	file, err := os.Open(path)
	if err != nil {
		return dedupRawSource{}, false
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	stored, total, ok := hashAndBufferStream(file, hasher)
	if !ok || total == 0 {
		return dedupRawSource{}, false
	}
	return dedupRawSource{
		hash:      hex.EncodeToString(hasher.Sum(nil)),
		stored:    stored,
		rawBytes:  total,
		truncated: total > int64(dedup.MaxArtifactBytes),
	}, true
}

// hashAndBufferStream feeds the whole stream through the hasher and returns
// the first MaxArtifactBytes of it plus the total byte count.
func hashAndBufferStream(reader io.Reader, hasher io.Writer) ([]byte, int64, bool) {
	stored := make([]byte, 0, 64*1024)
	var total int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			total += int64(n)
			_, _ = hasher.Write(buf[:n])
			stored = appendCapped(stored, buf[:n], dedup.MaxArtifactBytes)
		}
		if readErr == io.EOF {
			return stored, total, true
		}
		if readErr != nil {
			return nil, 0, false
		}
	}
}

func appendCapped(dst []byte, chunk []byte, limit int) []byte {
	remaining := limit - len(dst)
	if remaining <= 0 {
		return dst
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
	}
	return append(dst, chunk...)
}

// cleanupDedupCapture removes the retained capture file once dedup has
// archived what it needs, unless the file was persisted as the run's display
// artifact.
func cleanupDedupCapture(rawCapturePath string, displayedTeePath string) {
	if rawCapturePath == "" || rawCapturePath == displayedTeePath {
		return
	}
	_ = os.Remove(rawCapturePath)
}
