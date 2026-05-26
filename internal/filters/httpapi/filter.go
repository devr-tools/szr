package httpapi

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeHTTPAPI(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	statusLine, headers, body := splitHTTPExchange(clean)

	out := []string{}
	if summary := summarizeHTTPHeaders(statusLine, headers); summary != "" {
		out = append(out, shared.Clip(summary, 160))
	}

	body = strings.TrimSpace(body)
	if body != "" {
		if looksLikeJSON(body) {
			out = append(out, shared.NonEmptyLines(shared.RenderJSONStructure([]byte(body)))...)
		} else {
			out = append(out, shared.NonEmptyLines(shared.CompactLines(body, maxLines))...)
		}
	}

	if len(out) == 0 {
		return shared.CompactLines(clean, maxLines)
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func splitHTTPExchange(input string) (string, map[string]string, string) {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	statusLine := ""
	headers := map[string]string{}
	body := make([]string, 0, len(lines))
	inHeaders := false

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "HTTP/"):
			statusLine = trimmed
			headers = map[string]string{}
			inHeaders = true
		case inHeaders && trimmed == "":
			inHeaders = false
		case inHeaders:
			name, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				continue
			}
			headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
		default:
			if trimmed == "" || isTransportNoise(trimmed) {
				continue
			}
			body = append(body, line)
		}
	}

	return statusLine, headers, strings.Join(body, "\n")
}

func summarizeHTTPHeaders(statusLine string, headers map[string]string) string {
	if statusLine == "" && len(headers) == 0 {
		return ""
	}

	parts := []string{}
	if statusLine != "" {
		fields := strings.Fields(statusLine)
		if len(fields) >= 3 {
			parts = append(parts, "status="+fields[1]+" "+strings.Join(fields[2:], " "))
		} else {
			parts = append(parts, statusLine)
		}
	}
	if contentType := normalizeContentType(headers["content-type"]); contentType != "" {
		parts = append(parts, "content-type="+contentType)
	}
	if location := headers["location"]; location != "" {
		parts = append(parts, "location="+location)
	}
	return strings.Join(parts, " ")
}

func normalizeContentType(value string) string {
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(strings.ToLower(value))
}

func looksLikeJSON(body string) bool {
	trimmed := strings.TrimSpace(body)
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func isTransportNoise(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, "  % total"),
		strings.HasPrefix(lower, "length:"),
		strings.HasPrefix(lower, "saving to:"),
		strings.HasPrefix(lower, "resolving "),
		strings.HasPrefix(lower, "connecting to "),
		strings.HasPrefix(lower, "http request sent"),
		strings.HasPrefix(lower, "awaiting response"),
		strings.HasPrefix(lower, "response "),
		strings.HasPrefix(lower, "written to stdout"),
		strings.HasPrefix(lower, "* "),
		strings.HasPrefix(lower, "> "),
		strings.HasPrefix(lower, "< "):
		return true
	}

	if strings.HasSuffix(lower, "k/s") || strings.HasSuffix(lower, "m/s") {
		return true
	}
	return false
}

func SummarizeCurlHTTPAPI(input string, maxLines int) string {
	return SummarizeHTTPAPI(input, maxLines)
}

func SummarizeHTTPieAPI(input string, maxLines int) string {
	return SummarizeHTTPAPI(input, maxLines)
}

func SummarizeWgetHTTPAPI(input string, maxLines int) string {
	return SummarizeHTTPAPI(input, maxLines)
}
