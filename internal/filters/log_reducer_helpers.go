package filters

import (
	"sort"
	"strings"
)

type logLineKind uint8

const (
	logLineKindDefault logLineKind = iota
	logLineKindBoilerplate
	logLineKindHint
	logLineKindAnchor
	logLineKindError
)

type logLineEntry struct {
	key       string
	line      string
	firstSeen int
	lastSeen  int
	count     int
	kind      logLineKind
}

func orderedReducedLogEntries(entries []logLineEntry, selected map[string]struct{}) []logLineEntry {
	ordered := make([]logLineEntry, 0, len(selected))
	for _, entry := range entries {
		if _, ok := selected[entry.key]; ok {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

//nolint:maintidx // Entry collection intentionally combines dedupe, ordering, and signal promotion in one pass.
func collectLogLineEntries(lines []string) ([]logLineEntry, int) {
	indexByKey := map[string]int{}
	entries := make([]logLineEntry, 0, len(lines))
	total := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		total++

		key := reduceLogLineKey(line)
		if key == "" {
			continue
		}
		kind := classifyLogLine(line)
		if idx, ok := indexByKey[key]; ok {
			entries[idx].count++
			entries[idx].lastSeen = total - 1
			entries[idx].kind = maxLogLineKind(entries[idx].kind, kind)
			continue
		}

		indexByKey[key] = len(entries)
		entries = append(entries, logLineEntry{
			key:       key,
			line:      line,
			firstSeen: total - 1,
			lastSeen:  total - 1,
			count:     1,
			kind:      kind,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].firstSeen != entries[j].firstSeen {
			return entries[i].firstSeen < entries[j].firstSeen
		}
		return entries[i].lastSeen < entries[j].lastSeen
	})
	return entries, total
}

func reduceLogLineKey(line string) string {
	line = similarLineKey(line)
	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}

func classifyLogLine(line string) logLineKind {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case lower == "":
		return logLineKindBoilerplate
	case isErrorLogLine(lower):
		return logLineKindError
	case isAnchorLogLine(line, lower):
		return logLineKindAnchor
	case isHintLogLine(lower):
		return logLineKindHint
	case isBoilerplateLogLine(lower):
		return logLineKindBoilerplate
	default:
		return logLineKindDefault
	}
}

var errorLogLevelTokens = []string{
	"panic",
	"fatal",
	"error",
	"failed",
	"failure",
	"exception",
	"traceback",
	"assertionerror",
}

var hintLogLevelTokens = []string{"warning", "warn", "hint", "help", "note"}

var boilerplateLogLevelTokens = []string{
	"debug",
	"info",
	"trace",
	"progress",
	"downloading",
	"downloaded",
	"fetching",
	"installing",
	"installed",
	"resolving",
	"waiting",
	"retrying",
	"connected",
	"heartbeat",
}

func isErrorLogLine(lower string) bool {
	if strings.Contains(lower, "segmentation fault") {
		return true
	}
	return hasLeadingLogLevelToken(lower, errorLogLevelTokens)
}

func isAnchorLogLine(line, lower string) bool {
	if DiagnosticAnchor(line) != "" {
		return true
	}
	if strings.HasPrefix(lower, "at ") || strings.HasPrefix(lower, "#") {
		return true
	}
	return strings.Contains(lower, "caused by:")
}

func isHintLogLine(lower string) bool {
	return hasLeadingLogLevelToken(lower, hintLogLevelTokens)
}

func isBoilerplateLogLine(lower string) bool {
	if strings.Contains(lower, "listening on") || strings.Contains(lower, "watching for changes") {
		return true
	}
	return hasLeadingLogLevelToken(lower, boilerplateLogLevelTokens)
}

// hasLeadingLogLevelToken reports whether one of the first few
// whitespace/bracket-delimited fields of the (lowercased) line matches a level
// marker. Token matching avoids substring misfires such as "information"
// matching "info" or "no errors" matching "error".
func hasLeadingLogLevelToken(lower string, markers []string) bool {
	const maxLogLevelTokenFields = 3
	fields := 0
	i := 0
	for i < len(lower) && fields < maxLogLevelTokenFields {
		for i < len(lower) && isLogTokenDelimiter(lower[i]) {
			i++
		}
		start := i
		for i < len(lower) && !isLogTokenDelimiter(lower[i]) {
			i++
		}
		if start == i {
			break
		}
		token := strings.Trim(lower[start:i], logTokenPunctuationCutset)
		if token == "" {
			continue
		}
		fields++
		for _, marker := range markers {
			// Exact token match ("error", "[warn]", "info:") or a marker
			// suffix ("typeerror:", "assertionerror").
			if token == marker || strings.HasSuffix(token, marker) {
				return true
			}
		}
	}
	return false
}

const logTokenPunctuationCutset = ":.,;!?\"'`*-"

func isLogTokenDelimiter(c byte) bool {
	switch c {
	case ' ', '\t', '[', ']', '(', ')', '{', '}', '<', '>', '|', '=':
		return true
	}
	return false
}

func maxLogLineKind(left, right logLineKind) logLineKind {
	if right > left {
		return right
	}
	return left
}
