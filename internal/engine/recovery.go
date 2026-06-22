package engine

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const RecoveryKindFullOutput = "full-output"

type RecoveryPlan struct {
	Kind              string
	Summary           string
	RequireRawCapture bool
}

type streamReducerRecoveryProvider interface {
	RecoveryInfo() (kind string, summary string, requireRawCapture bool)
}

func reducerRecoveryPlan(reducer StreamReducer) RecoveryPlan {
	if reducer == nil {
		return RecoveryPlan{}
	}
	provider, ok := reducer.(streamReducerRecoveryProvider)
	if !ok {
		return RecoveryPlan{}
	}
	kind, summary, requireRawCapture := provider.RecoveryInfo()
	return RecoveryPlan{
		Kind:              strings.TrimSpace(kind),
		Summary:           strings.TrimSpace(summary),
		RequireRawCapture: requireRawCapture,
	}
}

func shouldPersistRecoveryArtifact(plan RecoveryPlan, rawCombined string, passthrough bool) bool {
	if passthrough || plan.Kind != RecoveryKindFullOutput {
		return false
	}
	return strings.TrimSpace(plan.Summary) != "" && strings.TrimSpace(rawCombined) != ""
}

func appendRecoveryHint(rendered string, plan RecoveryPlan, artifactPath string, passthrough bool) string {
	if passthrough || plan.Kind == "" || plan.Summary == "" || artifactPath == "" {
		return rendered
	}
	line := "[recovery: " + plan.Summary + "; full output: " + artifactPath + "]"
	if strings.TrimSpace(rendered) == "" {
		return line
	}
	return strings.TrimRight(rendered, "\n") + "\n" + line
}

func finalizeSmallOutputPreference(profile Profile, exitCode int, rendered string, rawCombined string, guardSmallOutput bool, ultraCompact bool) string {
	if !guardSmallOutput || ultraCompact {
		return rendered
	}
	return preferRawSmallOutputForProfile(profile, rendered, rawCombined, exitCode)
}

func fitRenderedDisplaySuffix(rendered string, suffixes []string, allowedTokens int) (string, bool) {
	for _, suffix := range suffixes {
		final, ok := buildRenderedDisplayWithSuffix(rendered, suffix, allowedTokens)
		if ok {
			return final, true
		}
	}
	return "", false
}

func buildRenderedDisplayWithSuffix(rendered string, suffix string, allowedTokens int) (string, bool) {
	suffixTokens := history.EstimateTokens(suffix)
	if suffixTokens >= allowedTokens {
		return "", false
	}
	remaining := allowedTokens - suffixTokens
	shrunk := rendered
	if history.EstimateTokens(rendered) > remaining {
		shrunk = hardCapTokens(rendered, remaining)
	}
	final := appendDisplaySuffix(shrunk, suffix)
	if history.EstimateTokens(final) > allowedTokens {
		return "", false
	}
	return final, true
}

func displayArtifactSuffixes(plan RecoveryPlan, artifactPath string, compactArtifactRefs bool) []string {
	if artifactPath == "" {
		return nil
	}
	ref := artifactDisplayRef(artifactPath, compactArtifactRefs)
	if plan.Kind != "" && plan.Summary != "" {
		return []string{
			"[recovery: " + plan.Summary + "; " + ref + "]",
			"[" + ref + "]",
			"[full output saved]",
		}
	}
	return []string{
		"[" + ref + "]",
		"[full output saved]",
	}
}

func artifactDisplayRef(path string, compact bool) string {
	if !compact {
		return "full output: " + path
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if len(id) > 12 {
		id = id[:12]
	}
	return "tee: " + id
}

func appendDisplaySuffix(rendered string, suffix string) string {
	if strings.TrimSpace(suffix) == "" {
		return rendered
	}
	if strings.TrimSpace(rendered) == "" {
		return suffix
	}
	return strings.TrimRight(rendered, "\n") + "\n" + suffix
}
