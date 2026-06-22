package engine

import "strings"

func ultraCompactLineScore(line string, lineIndex int, exitCode int) int {
	score := 0
	if lineIndex == 1 {
		score += 8
	}
	lower := strings.ToLower(line)
	score += ultraCompactSummaryPatternScore(line, lower)
	score += ultraCompactFailurePatternScore(lower, exitCode)
	for _, field := range strings.Fields(line) {
		score += compressionAnchorScore(field)
	}
	score += compressionContextScore(line)
	return score
}

func ultraCompactSummaryPatternScore(line string, lower string) int {
	switch {
	case strings.HasPrefix(line, "[recovery:"), strings.HasPrefix(line, "[full output"):
		return -32
	case strings.Contains(lower, "matches across"),
		strings.Contains(lower, "files="),
		strings.HasPrefix(lower, "dirs:"),
		strings.HasPrefix(lower, "... +"):
		return 18
	case strings.HasPrefix(lower, "examples:"):
		return 10
	case strings.Contains(lower, "match") && strings.Contains(line, "("):
		return 14
	default:
		return 0
	}
}

func ultraCompactFailurePatternScore(lower string, exitCode int) int {
	if exitCode != 0 && (strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic")) {
		return 18
	}
	return 0
}
