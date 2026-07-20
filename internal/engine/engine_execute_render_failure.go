package engine

import (
	"io"
	"os"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

// ensureInformativeFailureRender guarantees a failing command never renders
// content-free output (only ellipsis markers or artifact bookkeeping). The
// failure escape earlier in the pipeline can be undone by the compression
// contract or an over-aggressive reducer; when that happens, compact raw
// lines within the failure-escape budget are strictly more useful than a
// marker that spends tokens on zero signal. Fidelity beats savings on
// failures, but never past the raw output itself: the final
// never-worse-than-raw guard still caps the finished display at raw cost.
func ensureInformativeFailureRender(input streamingRenderInput, rendered string) string {
	if input.passthrough || !isFailureExit(input.profile, input.exec.ExitCode) || !isContentFreeRender(rendered) {
		return rendered
	}
	raw := failureEscapeSource(input.raw, input.artifactPath)
	if raw == "" {
		return rendered
	}
	escapeBudget := ExpandBudgetForFailureEscape(input.budget, input.inv)
	if escaped := filters.CompactLines(raw, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
		return escaped
	}
	return rendered
}

// failureEscapeSource returns the raw text the failure escape should compact:
// the in-memory capture when present, otherwise the tee artifact.
func failureEscapeSource(rawCombined string, artifactPath string) string {
	if strings.TrimSpace(rawCombined) != "" {
		return rawCombined
	}
	if artifact := readFailureArtifact(artifactPath); strings.TrimSpace(artifact) != "" {
		return artifact
	}
	return ""
}

const failureArtifactReadLimit = 256 * 1024

// readFailureArtifact recovers raw output for the failure escape when the
// in-memory capture is empty. Stream profiles that reduce only one stream
// buffer only a bounded preview of the other stream — yet on failure that
// other stream usually carries the diagnostics. The tee artifact written
// during the run holds the full interleaved output; a bounded prefix is
// enough for a compact escape render.
func readFailureArtifact(path string) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	data := make([]byte, failureArtifactReadLimit)
	n, _ := io.ReadFull(file, data)
	return string(data[:n])
}
