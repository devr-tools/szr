package engine

import "strings"

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
	captureComplete     bool
}

func (f renderedDisplayFinalizer) finalize() string {
	// Apply the never-worse-than-raw preference to the rendered content
	// before artifact suffixes are attached: the guard must not strip
	// tee/recovery references when it decides tiny raw output is cheaper.
	f.rendered = finalizeSmallOutputPreference(f.profile, f.exitCode, f.rendered, f.rawCombined, f.guardSmallOutput, f.ultraCompact)
	if f.passthrough {
		return f.rendered
	}
	// A successful display that already IS the complete raw output omits
	// nothing; a recovery pointer would spend tokens on pure redundancy.
	// Only a complete capture can prove that - a preview-limited buffer
	// matching the display says nothing about the full stream.
	if f.exitCode == 0 && f.captureComplete && strings.TrimSpace(f.rendered) == strings.TrimSpace(f.rawCombined) {
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
	allowedTokens := compressionContractAllowedTokens(rawTokens, f.budget)
	if final, ok := fitRenderedDisplaySuffix(f.rendered, suffixes, allowedTokens); ok {
		return final
	}
	// Never emit a content-free display: keep the highest-value slice of the
	// render even when the artifact suffix pushes the total past the cap.
	return appendDisplaySuffix(hardCapTokens(f.rendered, allowedTokens), suffixes[len(suffixes)-1])
}
