package bench

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

var failureTokenPattern = regexp.MustCompile(`([[:alnum:]_./-]+\.[[:alnum:]_./-]+:\d+(?::\d+)?)|([[:alnum:]_./-]+:\d+(?::\d+)?)`)

func evaluateExpectation(fixture Fixture, measurement Measurement) Expectation {
	missing := make([]string, 0, len(fixture.ExpectedContains))
	for _, fragment := range fixture.ExpectedContains {
		if !strings.Contains(measurement.Rendered, fragment) {
			missing = append(missing, fragment)
		}
	}

	expectation := Expectation{
		ContainsOK:      len(missing) == 0,
		TokenSavingsOK:  measurement.TokenSavingsPct >= fixture.MinTokenSavings,
		QualityOK:       measurement.Quality.Score >= fixture.MinQualityScore,
		MissingContains: missing,
	}
	expectation.OK = expectation.ContainsOK && expectation.TokenSavingsOK && expectation.QualityOK
	return expectation
}

func evaluateQuality(fixture Fixture, measurement Measurement) Quality {
	actionable := countActionableLines(measurement.Rendered)
	rawFailureTokens, renderedFailureTokens := qualityFailureIdentifiers(fixture, measurement)
	preservedFailures := countPreservedFailures(rawFailureTokens, renderedFailureTokens)
	score, issues := scoreQuality(actionable, rawFailureTokens, preservedFailures, measurement)

	return Quality{
		Score:              score,
		ActionableLines:    actionable,
		FailureIdentifiers: len(rawFailureTokens),
		PreservedFailures:  preservedFailures,
		FallbackRate:       measurement.FallbackRate,
		ProfileConfidence:  profileConfidence(fixture.ProfileName),
		Issues:             issues,
	}
}

func countActionableLines(rendered string) int {
	actionable := 0
	for _, line := range nonEmptyLines(rendered) {
		if isActionableLine(line) {
			actionable++
		}
	}
	return actionable
}

func qualityFailureIdentifiers(fixture Fixture, measurement Measurement) ([]string, []string) {
	if fixture.Execution.ExitCode == 0 {
		return nil, nil
	}
	return extractFailureIdentifiers(fixture.RawCombined()), extractFailureIdentifiers(measurement.Rendered)
}

func countPreservedFailures(rawFailureTokens, renderedFailureTokens []string) int {
	preservedFailures := 0
	for _, rawToken := range rawFailureTokens {
		for _, renderedToken := range renderedFailureTokens {
			if strings.Contains(renderedToken, rawToken) || strings.Contains(rawToken, renderedToken) {
				preservedFailures++
				break
			}
		}
	}
	return preservedFailures
}

func scoreQuality(actionable int, rawFailureTokens []string, preservedFailures int, measurement Measurement) (int, []string) {
	score := 100
	issues := []string{}
	if actionable == 0 {
		issues = append(issues, "zero_actionable_lines")
		score -= 60
	}
	if len(rawFailureTokens) > 0 && preservedFailures == 0 {
		issues = append(issues, "missing_failure_identifiers")
		score -= 30
	}
	if measurement.FallbackRate >= 100 && hasMaterialTokenVolume(measurement) {
		issues = append(issues, "excessive_fallback_usage")
		score -= 20
	} else if measurement.FallbackRate >= 50 && hasMeaningfulSavingsOpportunity(measurement) {
		issues = append(issues, "fallback_heavy")
		score -= 10
	}
	if hasMaterialLowSavings(measurement) {
		issues = append(issues, "low_token_savings")
		score -= 15
	}
	if measurement.SavedTokens < 0 {
		if isTinyOutputOverhead(measurement) {
			issues = append(issues, "tiny_output_overhead")
			score -= 10
		} else {
			issues = append(issues, "negative_token_savings")
			score -= 40
		}
	}
	if score < 0 {
		score = 0
	}
	return score, issues
}

func hasMaterialTokenVolume(measurement Measurement) bool {
	return measurement.RawTokens >= 96 || measurement.FilteredTokens >= 64
}

func hasMeaningfulSavingsOpportunity(measurement Measurement) bool {
	return measurement.RawTokens >= 128 || measurement.FilteredTokens >= 96 || measurement.SavedTokens >= 24
}

func hasMaterialLowSavings(measurement Measurement) bool {
	if !hasMeaningfulSavingsOpportunity(measurement) {
		return false
	}
	if measurement.SavedTokens < 0 && isTinyOutputOverhead(measurement) {
		return false
	}
	threshold := 8.0
	if measurement.RawTokens >= 256 || measurement.FilteredTokens >= 192 {
		threshold = 10.0
	}
	return measurement.TokenSavingsPct < threshold
}

func isTinyOutputOverhead(measurement Measurement) bool {
	return measurement.RawTokens < 32 && measurement.FilteredTokens < 32 && -measurement.SavedTokens <= 4
}

func parsedProfileInput(fixture Fixture) string {
	stdout := fixture.Execution.Stdout
	stderr := fixture.Execution.Stderr

	switch fixture.ProfileName {
	case "go-test-json", "git-diff":
		return stdout
	case "go-build":
		return joinNonEmpty(stderr, stdout)
	case "generic-test", "generic-summary":
		return joinNonEmpty(stdout, stderr)
	default:
		return fixture.RawCombined()
	}
}

func commandFingerprint(display []string) string {
	if len(display) == 0 {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(display, "\x00")))
	return fmt.Sprintf("%016x", h.Sum64())
}

func normalizedText(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(input, "\r\n", "\n"))
}

func extractFailureIdentifiers(input string) []string {
	out := []string{}
	for _, line := range nonEmptyLines(input) {
		identifier := failureIdentifier(line)
		if identifier == "" {
			continue
		}
		out = append(out, identifier)
		if len(out) >= 8 {
			break
		}
	}
	return filters.UniqueStrings(out)
}

func failureIdentifier(line string) string {
	if matches := failureTokenPattern.FindStringSubmatch(line); len(matches) > 0 {
		for _, match := range matches[1:] {
			if match != "" {
				return match
			}
		}
	}

	keywords := []string{"FAIL", "panic:", "TimeoutError:", "AssertionError:", "Error:", "error:", "warning:", "Warning:"}
	for _, keyword := range keywords {
		if idx := strings.Index(line, keyword); idx >= 0 {
			return strings.TrimSpace(filters.Clip(line[idx:], 120))
		}
	}
	return ""
}

func nonEmptyLines(input string) []string {
	raw := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isActionableLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, "[full output:") {
		return false
	}
	return true
}

func joinNonEmpty(left, right string) string {
	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return left + "\n" + right
	}
}

func profileConfidence(name string) string {
	switch name {
	case "git-status", "git-log", "go-test-json", "vitest-json", "jest-json", "cargo-test":
		return "high"
	case "git-diff", "go-build", "generic-test", "js-package-test", "directory-listing", "cat-read":
		return "medium"
	default:
		return "low"
	}
}
