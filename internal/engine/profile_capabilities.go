package engine

func annotateProfilesCapabilities(profiles []Profile) []Profile {
	annotated := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		profile.Capabilities = normalizeProfileCapabilities(profile)
		annotated = append(annotated, profile)
	}
	return annotated
}

func normalizeProfileCapabilities(profile Profile) ProfileCapabilities {
	caps := profile.Capabilities

	if caps.FastPathBypass == "" {
		switch {
		case profile.Name == "passthrough":
			caps.FastPathBypass = FastPathBypassSmallOutput
		case profile.StreamRender == nil:
			caps.FastPathBypass = FastPathBypassNever
		case profile.Confidence == ConfidenceHigh:
			if allowsHighConfidenceBypass(profile.Name) {
				caps.FastPathBypass = FastPathBypassSafeOnly
			} else {
				caps.FastPathBypass = FastPathBypassNever
			}
		default:
			caps.FastPathBypass = FastPathBypassSmallOutput
		}
	}

	if !caps.AllowFailureEscape && profile.Name != "passthrough" && profile.Confidence != ConfidenceHigh {
		caps.AllowFailureEscape = true
	}

	if !caps.RequireFullCapture && profile.Confidence != ConfidenceHigh {
		caps.RequireFullCapture = true
	}

	if !caps.InjectsPrepareArgs && profile.Prepare != nil && profile.Name != "passthrough" {
		caps.InjectsPrepareArgs = true
	}

	return caps
}

func isBenignExit(profile Profile, exitCode int) bool {
	if exitCode == 0 {
		return false
	}
	for _, code := range profile.Capabilities.BenignExitCodes {
		if code == exitCode {
			return true
		}
	}
	return false
}

func isFailureExit(profile Profile, exitCode int) bool {
	return exitCode != 0 && !isBenignExit(profile, exitCode)
}

func shouldBypassForDecisionMode(mode string, decision FastPathDecision) bool {
	switch mode {
	case FastPathBypassSmallOutput:
		return decision.BypassKind == FastPathBypassKindTinyOutput ||
			decision.BypassKind == FastPathBypassKindFamilyRule ||
			decision.BypassKind == FastPathBypassKindEmptyPreferredStream
	case FastPathBypassSafeOnly:
		return decision.BypassKind == FastPathBypassKindFamilyRule ||
			decision.BypassKind == FastPathBypassKindEmptyPreferredStream
	default:
		return false
	}
}
