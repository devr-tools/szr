package engine

import "strings"

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
