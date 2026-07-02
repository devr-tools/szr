package engine

type renderedDisplayFinalizer struct {
	profile             Profile
	exitCode            int
	rendered            string
	rawCombined         string
	rawTokens           int
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
	// Apply the never-worse-than-raw preference to the rendered content
	// before artifact suffixes are attached: the guard must not strip
	// tee/recovery references when it decides tiny raw output is cheaper.
	f.rendered = finalizeSmallOutputPreference(f.profile, f.exitCode, f.rendered, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	if f.passthrough {
		return f.rendered
	}
	suffixes := displayArtifactSuffixes(f.plan, f.artifactPath, f.compactArtifactRefs)
	if len(suffixes) == 0 {
		return f.rendered
	}
	rawTokens := trueRawTokenCount(f.rawTokens, f.rawCombined)
	if !f.compressionContract || rawTokens < compressionContractMinRawTokens {
		return appendDisplaySuffix(f.rendered, suffixes[0])
	}
	if final, ok := fitRenderedDisplaySuffix(f.rendered, suffixes, compressionContractAllowedTokens(rawTokens, f.budget)); ok {
		return final
	}
	return suffixes[len(suffixes)-1]
}
