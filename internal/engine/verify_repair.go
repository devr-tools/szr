package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

// retentionRepairHeader introduces the compact repair section the verifier
// appends when a render dropped critical facts. The render itself is never
// replaced — repair only ever adds the missing signal back.
const retentionRepairHeader = "missing detail:"

// retentionVerifyInput carries everything needed to verify one finished
// render against the raw signal it summarizes.
type retentionVerifyInput struct {
	rendered          string
	rawCombined       string
	rawTokens         int
	artifactPath      string
	captureIncomplete bool
	exitCode          int
	profile           Profile
	budget            OutputBudget
	inv               Invocation
	passthrough       bool
	fastPathBypass    bool
	memo              history.TokenMemo
}

// retentionOutcome is the verifier's per-emission telemetry.
type retentionOutcome struct {
	repairs int
	skipped bool
}

// applyRetentionVerifier enforces szr's retention guarantee after the full
// render pipeline: extract critical facts from the raw signal, verify their
// identifying needles survived in the render, and append dropped lines as a
// compact repair section capped by the failure-escape budget.
func applyRetentionVerifier(in retentionVerifyInput) (string, retentionOutcome) {
	if !shouldRunRetentionVerifier(in) {
		return in.rendered, retentionOutcome{}
	}
	raw, ok := retentionRawSource(in)
	if !ok {
		// Capture stopped at the preview limit and no artifact holds the
		// full stream. Verifying against the preview would report phantom
		// drops, so record the skip instead of guessing.
		return in.rendered, retentionOutcome{skipped: true}
	}
	report := VerifyRetention(raw, in.rendered, isFailureExit(in.profile, in.exitCode))
	if len(report.MissingLines) == 0 {
		return in.rendered, retentionOutcome{}
	}
	maxLines := ExpandBudgetForFailureEscape(in.budget, in.inv).MaxLines
	return appendRetentionRepair(in.rendered, report.MissingLines, maxLines)
}

// shouldRunRetentionVerifier keeps verification off the fast paths: raw
// passthrough, the tiny-output bypass, and outputs below the compression
// contract's arming threshold ship (or nearly ship) the raw signal already,
// so there is nothing cheaper to verify than what the caller received.
func shouldRunRetentionVerifier(in retentionVerifyInput) bool {
	if in.passthrough || !in.inv.Advanced.RetentionVerifier || in.fastPathBypass {
		return false
	}
	return trueRawTokenCount(in.rawTokens, in.rawCombined, in.memo) >= compressionContractMinRawTokens
}

// retentionRawSource returns the raw text to verify against: the in-memory
// capture when it is complete, otherwise the tee artifact written during the
// run (bounded read). Reports false when neither holds the full signal.
func retentionRawSource(in retentionVerifyInput) (string, bool) {
	if !in.captureIncomplete {
		return in.rawCombined, true
	}
	if artifact := readFailureArtifact(in.artifactPath); strings.TrimSpace(artifact) != "" {
		return artifact, true
	}
	return "", false
}

func appendRetentionRepair(rendered string, missing []string, maxLines int) (string, retentionOutcome) {
	if maxLines < 1 {
		maxLines = 1
	}
	if len(missing) > maxLines {
		missing = missing[:maxLines]
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimRight(rendered, "\n"))
	builder.WriteString("\n")
	builder.WriteString(retentionRepairHeader)
	for _, line := range missing {
		builder.WriteString("\n")
		builder.WriteString(line)
	}
	return builder.String(), retentionOutcome{repairs: len(missing)}
}
