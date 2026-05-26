package sqlquery

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var sqlCommandTagPattern = regexp.MustCompile(`^(SELECT|INSERT|UPDATE|DELETE|MERGE|COPY) [0-9]+$`)

func SummarizeSQLQuery(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return "ok"
	}

	if jsonSummary := summarizeJSONResult(lines, maxLines); jsonSummary != "" {
		return jsonSummary
	}

	errors := []string{}
	summaries := []string{}
	rows := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isSQLNoise(trimmed) || isSQLBorder(trimmed) {
			continue
		}
		switch {
		case isSQLError(trimmed):
			errors = append(errors, shared.Clip(trimmed, 180))
		case isSQLSummary(trimmed):
			summaries = append(summaries, shared.Clip(trimmed, 180))
		default:
			rows = append(rows, shared.Clip(trimmed, 180))
		}
	}

	errors = shared.UniqueStrings(shared.FoldConsecutiveLines(errors))
	summaries = shared.UniqueStrings(shared.FoldConsecutiveLines(summaries))
	rows = shared.UniqueStrings(shared.FoldConsecutiveLines(rows))

	if len(errors) > 0 {
		out := append([]string{}, errors...)
		out = append(out, summaries...)
		return shared.JoinLimitedLines(out, maxLines)
	}

	if len(rows) == 0 && len(summaries) == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}

	if len(rows) > 0 {
		limit := maxLines
		if len(summaries) > 0 {
			limit--
		}
		if limit < 1 {
			limit = 1
		}
		if len(rows) > limit {
			rows = append(rows[:limit], fmt.Sprintf("... +%d more rows", len(rows)-limit))
		}
	}

	out := append([]string{}, rows...)
	out = append(out, summaries...)
	return shared.JoinLimitedLines(out, maxLines)
}

func summarizeJSONResult(lines []string, maxLines int) string {
	payload := strings.TrimSpace(strings.Join(lines, "\n"))
	if payload == "" || (!strings.HasPrefix(payload, "[") && !strings.HasPrefix(payload, "{")) {
		return ""
	}

	var decoded any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return ""
	}

	switch v := decoded.(type) {
	case []any:
		out := []string{fmt.Sprintf("%d row(s)", len(v))}
		limit := maxLines - 1
		if limit < 1 {
			limit = 1
		}
		for i := 0; i < len(v) && i < limit; i++ {
			encoded, err := json.Marshal(v[i])
			if err != nil {
				continue
			}
			out = append(out, string(encoded))
		}
		if len(v) > limit {
			out = append(out, fmt.Sprintf("... +%d more rows", len(v)-limit))
		}
		return strings.Join(out, "\n")
	case map[string]any:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(encoded)
	default:
		return ""
	}
}

func isSQLNoise(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(line, "Pager usage is off."):
		return true
	case strings.HasPrefix(lower, "sqlite version "),
		strings.HasPrefix(lower, "enter \".help\""),
		strings.HasPrefix(lower, "connected to a transient in-memory database"):
		return true
	case strings.HasPrefix(lower, "welcome to the mysql monitor"),
		strings.HasPrefix(lower, "type 'help;'"),
		strings.HasPrefix(lower, "your mysql connection id is "):
		return true
	case strings.HasPrefix(lower, "duckdb "),
		strings.HasPrefix(lower, "enter \".help\" for usage hints"):
		return true
	default:
		return false
	}
}

func isSQLBorder(line string) bool {
	if line == "" {
		return false
	}
	for _, r := range line {
		if !strings.ContainsRune("+-=|:._ \t┌┐└┘├┤┬┴┼─━╔╗╚╝╠╣╦╩╬", r) {
			return false
		}
	}
	return true
}

func isSQLError(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(line, "ERROR:"),
		strings.HasPrefix(line, "Error:"),
		strings.HasPrefix(line, "Parse error"),
		strings.HasPrefix(line, "Runtime error"),
		strings.HasPrefix(line, "LINE "),
		strings.HasPrefix(line, "Catalog Error:"),
		strings.HasPrefix(line, "Parser Error:"),
		strings.HasPrefix(line, "Binder Error:"):
		return true
	case strings.Contains(line, "SQLSTATE["),
		strings.Contains(lower, "syntax error"),
		strings.Contains(lower, "no such table"),
		strings.Contains(lower, "does not exist"),
		strings.Contains(lower, "unknown column"),
		strings.Contains(lower, "unknown database"),
		strings.Contains(lower, "access denied"),
		strings.Contains(lower, "constraint failed"):
		return true
	default:
		return false
	}
}

func isSQLSummary(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case sqlCommandTagPattern.MatchString(line):
		return true
	case strings.HasPrefix(line, "(") && strings.HasSuffix(lower, " row)") ||
		strings.HasPrefix(line, "(") && strings.HasSuffix(lower, " rows)") ||
		strings.HasSuffix(lower, " row in set") ||
		strings.Contains(lower, " rows in set") ||
		strings.HasPrefix(line, "Empty set") ||
		strings.HasPrefix(line, "Query OK"):
		return true
	case strings.HasPrefix(line, "Run Time:"),
		strings.HasPrefix(line, "Time:"):
		return true
	case strings.HasPrefix(line, "CREATE "),
		strings.HasPrefix(line, "ALTER "),
		strings.HasPrefix(line, "DROP "),
		strings.HasPrefix(line, "VACUUM"),
		strings.HasPrefix(line, "ANALYZE"):
		return true
	default:
		return false
	}
}
