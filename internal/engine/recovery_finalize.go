package engine

import "github.com/devr-tools/szr/internal/history"

type renderedDisplayFinalizer struct {
	profile             Profile
	exitCode            int
	rendered            string
	rawCombined         string
	budget              OutputBudget
	plan                RecoveryPlan
	artifactPath        string
	passthrough         bool
	compactArtifactRefs bool
	compressionContract bool
	guardSmallOutput    bool
	ultraCompact        bool
}

func (f renderedDisplayFinalizer) finalize() string {
	if f.passthrough {
		return finalizeSmallOutputPreference(f.profile, f.exitCode, f.rendered, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	}
	suffixes := displayArtifactSuffixes(f.plan, f.artifactPath, f.compactArtifactRefs)
	if len(suffixes) == 0 {
		return finalizeSmallOutputPreference(f.profile, f.exitCode, f.rendered, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	}
	rawTokens := history.EstimateTokens(f.rawCombined)
	if !f.compressionContract || rawTokens < compressionContractMinRawTokens {
		final := appendDisplaySuffix(f.rendered, suffixes[0])
		return finalizeSmallOutputPreference(f.profile, f.exitCode, final, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	}
	if final, ok := fitRenderedDisplaySuffix(f.rendered, suffixes, compressionContractAllowedTokens(rawTokens, f.budget)); ok {
		return finalizeSmallOutputPreference(f.profile, f.exitCode, final, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	}
	return finalizeSmallOutputPreference(f.profile, f.exitCode, suffixes[len(suffixes)-1], f.rawCombined, f.guardSmallOutput, f.ultraCompact)
}
