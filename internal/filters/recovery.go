package filters

import "github.com/devr-tools/szr/internal/filters/declarative"

const RecoveryKindFullOutput = "full-output"

func NoRecovery() (string, string, bool) {
	return "", "", false
}

func FullOutputRecovery(summary string) (string, string, bool) {
	if summary == "" {
		return "", "", true
	}
	return RecoveryKindFullOutput, summary, true
}

func DeclarativeFullOutputRecovery(result declarative.Result, noun string) (string, string, bool) {
	if result.OmittedCount() <= 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(result.RecoverySummary(noun))
}
