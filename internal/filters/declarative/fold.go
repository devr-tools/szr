package declarative

import (
	"fmt"
	"strings"
)

// FoldConsecutive folds runs of consecutive duplicate lines into a single
// "line (xN)" entry. Lines are compared after trimming surrounding
// whitespace; empty lines are dropped.
func FoldConsecutive(lines []string) []string {
	return foldConsecutive(lines, func(line string) string { return line })
}

// FoldConsecutiveSimilar folds runs of consecutive lines that normalize to
// the same SimilarLineKey into a single "line (xN)" entry, keeping the first
// line of each run. Empty lines are dropped.
func FoldConsecutiveSimilar(lines []string) []string {
	return foldConsecutive(lines, SimilarLineKey)
}

func foldConsecutive(lines []string, keyFn func(string) string) []string {
	if len(lines) == 0 {
		return nil
	}
	folder := &lineRunFolder{out: make([]string, 0, len(lines))}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		folder.ingest(trimmed, keyFn(trimmed))
	}
	folder.flush()
	return folder.out
}

// lineRunFolder tracks the current run of same-key lines while folding.
type lineRunFolder struct {
	out        []string
	current    string
	currentKey string
	count      int
}

// ingest extends the pending run when key matches, otherwise flushes it and
// starts a new run at the given line.
func (f *lineRunFolder) ingest(trimmed, key string) {
	if f.count > 0 && key == f.currentKey {
		f.count++
		return
	}
	f.flush()
	f.current = trimmed
	f.currentKey = key
	f.count = 1
}

// flush appends the pending run to the output as "line" or "line (xN)".
func (f *lineRunFolder) flush() {
	if f.count == 0 {
		return
	}
	if f.count > 1 {
		f.out = append(f.out, fmt.Sprintf("%s (x%d)", f.current, f.count))
	} else {
		f.out = append(f.out, f.current)
	}
	f.count = 0
}

// SimilarLineKey normalizes a line for fold-similar comparisons: it strips a
// leading timestamp (ISO-8601-like, syslog, bracketed, bare clock, or epoch),
// optionally preceded by a log-level token (which is kept, so ERROR lines
// never fold into INFO runs), plus a trailing counter-like token (numbers,
// percentages, durations), and collapses internal whitespace.
func SimilarLineKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	parts := strings.Fields(trimmed)
	if len(parts) >= 2 && looksLikeLogLevelToken(parts[0]) {
		rest := stripLeadingTimestamp(parts[1:])
		if len(rest) < len(parts)-1 {
			parts = append(parts[:1:1], rest...)
		}
	} else {
		parts = stripLeadingTimestamp(parts)
	}
	if len(parts) >= 2 && looksLikeTrailingCounter(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, " ")
}

// stripLeadingTimestamp removes a leading timestamp, which may span multiple
// tokens (syslog "Jan  2 15:04:05"), when at least one message token remains.
func stripLeadingTimestamp(parts []string) []string {
	n := leadingTimestampTokens(parts)
	if n == 0 || n >= len(parts) {
		return parts
	}
	return parts[n:]
}

func leadingTimestampTokens(parts []string) int {
	if len(parts) == 0 {
		return 0
	}
	if len(parts) >= 3 && looksLikeSyslogMonth(parts[0]) && looksLikeSyslogDay(parts[1]) && looksLikeClockToken(parts[2]) {
		return 3
	}
	if looksLikeTimestampToken(parts[0]) {
		return 1
	}
	return 0
}

// looksLikeTimestampToken recognizes single-token timestamps: ISO-8601-like
// dates, bare clocks ("15:04:05.123"), epoch seconds/milliseconds, and any of
// those wrapped in brackets ("[2024-01-02T10:00:00Z]", "[15:04:05]").
func looksLikeTimestampToken(token string) bool {
	trimmed := strings.Trim(token, "[]")
	if trimmed == "" {
		return false
	}
	return looksLikeTimestampPrefix(trimmed) || looksLikeClockToken(trimmed) || looksLikeEpochToken(trimmed)
}

// looksLikeLogLevelToken reports whether a token is a leading log-level
// marker such as "INFO", "[WARN]", or "ERROR:".
func looksLikeLogLevelToken(token string) bool {
	trimmed := strings.Trim(token, "[]:")
	if len(trimmed) < 4 || len(trimmed) > 7 {
		return false
	}
	switch strings.ToUpper(trimmed) {
	case "INFO", "WARN", "WARNING", "ERROR", "DEBUG", "TRACE", "FATAL":
		return true
	}
	return false
}

func looksLikeTimestampPrefix(value string) bool {
	if len(value) < len("2006-01-02T15:04:05Z") {
		return false
	}
	if value[4] != '-' || value[7] != '-' {
		return false
	}
	return strings.Contains(value, "T") || strings.Contains(value, "_")
}

// looksLikeClockToken reports whether a token is a bare clock prefix like
// "15:04:05", "15:04:05.123", or "15:04:05,123".
func looksLikeClockToken(token string) bool {
	if len(token) < 8 || !looksLikeClockCore(token[:8]) {
		return false
	}
	return looksLikeClockFraction(token[8:])
}

// looksLikeClockCore reports whether an 8-byte segment has the "HH:MM:SS"
// shape: digits everywhere except colons at positions 2 and 5.
func looksLikeClockCore(core string) bool {
	for i := 0; i < 8; i++ {
		if i == 2 || i == 5 {
			if core[i] != ':' {
				return false
			}
			continue
		}
		if core[i] < '0' || core[i] > '9' {
			return false
		}
	}
	return true
}

// looksLikeClockFraction reports whether the remainder after "HH:MM:SS" is
// empty or a '.'/',' separator followed by at least one digit.
func looksLikeClockFraction(rest string) bool {
	if rest == "" {
		return true
	}
	if len(rest) < 2 || (rest[0] != '.' && rest[0] != ',') {
		return false
	}
	return isDigitRun(rest[1:])
}

// isDigitRun reports whether every byte of s is an ASCII digit.
func isDigitRun(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// looksLikeEpochToken reports whether a token is a bare 10-digit (seconds) or
// 13-digit (milliseconds) epoch timestamp.
func looksLikeEpochToken(token string) bool {
	if len(token) != 10 && len(token) != 13 {
		return false
	}
	return isDigitRun(token)
}

func looksLikeSyslogMonth(token string) bool {
	switch token {
	case "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec":
		return true
	}
	return false
}

func looksLikeSyslogDay(token string) bool {
	if len(token) == 0 || len(token) > 2 {
		return false
	}
	return isDigitRun(token)
}

// looksLikeTrailingCounter reports whether a token looks like a bare counter,
// such as "3", "42%", "8s", "120ms", "(3/10)", or "1:23".
func looksLikeTrailingCounter(token string) bool {
	trimmed := strings.Trim(token, "([{)]}")
	for _, suffix := range []string{"%", "ms", "s"} {
		trimmed = strings.TrimSuffix(trimmed, suffix)
	}
	if trimmed == "" {
		return false
	}
	hasDigit := false
	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.' || r == ',' || r == '/' || r == ':' || r == '-':
		default:
			return false
		}
	}
	return hasDigit
}
