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

func finalizeRenderedDisplay(rendered string, rawCombined string, budget OutputBudget, plan RecoveryPlan, artifactPath string, passthrough bool, compactArtifactRefs bool, compressionContract bool, guardSmallOutput bool) string {
	if passthrough {
		if guardSmallOutput {
			return preferRawSmallOutput(rendered, rawCombined)
		}
		return rendered
	}
	suffixes := displayArtifactSuffixes(plan, artifactPath, compactArtifactRefs)
	if len(suffixes) == 0 {
		if guardSmallOutput {
			return preferRawSmallOutput(rendered, rawCombined)
		}
		return rendered
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if !compressionContract || rawTokens < compressionContractMinRawTokens {
		final := appendDisplaySuffix(rendered, suffixes[0])
		if guardSmallOutput {
			return preferRawSmallOutput(final, rawCombined)
		}
		return final
	}
	allowedTokens := compressionContractAllowedTokens(rawTokens, budget)
	for _, suffix := range suffixes {
		suffixTokens := history.EstimateTokens(suffix)
		if suffixTokens >= allowedTokens {
			continue
		}
		remaining := allowedTokens - suffixTokens
		shrunk := rendered
		if history.EstimateTokens(rendered) > remaining {
			shrunk = hardCapTokens(rendered, remaining)
		}
		final := appendDisplaySuffix(shrunk, suffix)
		if history.EstimateTokens(final) <= allowedTokens {
			if guardSmallOutput {
				return preferRawSmallOutput(final, rawCombined)
			}
			return final
		}
	}
	if guardSmallOutput {
		return preferRawSmallOutput(suffixes[len(suffixes)-1], rawCombined)
	}
	return suffixes[len(suffixes)-1]
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
