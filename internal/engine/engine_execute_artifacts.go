package engine

import "os"

type streamingArtifactInput struct {
	teePath        string
	exitCode       int
	rawCombined    string
	command        []string
	profile        Profile
	fallbackUsed   bool
	recoveryPlan   RecoveryPlan
	passthrough    bool
	keepRawCapture bool
}

func (e *Engine) ensureStreamingArtifactPath(input streamingArtifactInput) string {
	if input.teePath != "" {
		return e.retainExistingTeeArtifact(input)
	}
	return e.writeDeferredTeeArtifact(input)
}

func (e *Engine) retainExistingTeeArtifact(input streamingArtifactInput) string {
	if shouldPersistRecoveryArtifact(input.recoveryPlan, input.rawCombined, input.passthrough) {
		return input.teePath
	}
	if isFailureExit(input.profile, input.exitCode) && e.config.TeeOnFailure && shouldPersistFailureArtifact(input.profile, input.fallbackUsed, input.passthrough) {
		return input.teePath
	}
	if !input.keepRawCapture {
		_ = os.Remove(input.teePath)
	}
	return ""
}

func (e *Engine) writeDeferredTeeArtifact(input streamingArtifactInput) string {
	if input.rawCombined == "" {
		return ""
	}
	if shouldPersistRecoveryArtifact(input.recoveryPlan, input.rawCombined, input.passthrough) {
		return e.writeTeeArtifact(input.rawCombined, input.command)
	}
	if !isFailureExit(input.profile, input.exitCode) || !e.config.TeeOnFailure || !shouldPersistFailureArtifact(input.profile, input.fallbackUsed, input.passthrough) {
		return ""
	}
	return e.writeTeeArtifact(input.rawCombined, input.command)
}

func (e *Engine) writeTeeArtifact(rawCombined string, command []string) string {
	path, teeErr := e.writeTee(rawCombined, command)
	if teeErr != nil {
		return ""
	}
	return path
}

func shouldPersistFailureArtifact(profile Profile, fallbackUsed bool, passthrough bool) bool {
	return passthrough || profile.Name == "passthrough" || fallbackUsed || profile.Confidence != ConfidenceHigh
}
