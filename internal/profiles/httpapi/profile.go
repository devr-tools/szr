package httpapi

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	httpapifilter "github.com/devr-tools/szr/internal/filters/httpapi"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "http-api",
			Description:      "Summarizes HTTP API fetches around response status, headers, and JSON body structure.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return matchesHTTPAPI(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareHTTPAPI(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return httpapifilter.SummarizeHTTPAPI(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					true,
					func(input string) string {
						return httpapifilter.SummarizeHTTPAPI(input, budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return httpapifilter.HTTPAPIRecoveryInfo(input, budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches API-oriented `curl`, `http`, `httpie`, and stdout-targeted `wget` fetches instead of generic text output.",
				"Adds response headers where safe, then condenses the result into status, content type, and JSON body shape.",
			},
		},
	}
}

func matchesHTTPAPI(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "curl":
		if hasCurlOutputFile(args) {
			return false
		}
		url := firstURLArg(args)
		return url != "" && (looksLikeAPIURL(url) || hasAPIHints(args))
	case "http", "httpie":
		return firstURLArg(args) != ""
	case "wget":
		if !isWgetStdoutFetch(args) {
			return false
		}
		url := firstURLArg(args)
		return url != "" && (looksLikeAPIURL(url) || hasAPIHints(args))
	default:
		return false
	}
}

func prepareHTTPAPI(command []string) []string {
	if len(command) == 0 {
		return command
	}

	switch command[0] {
	case "curl":
		return prepareCurl(command)
	case "http", "httpie":
		return prepareHTTPie(command)
	case "wget":
		return prepareWget(command)
	default:
		return command
	}
}

func prepareCurl(command []string) []string {
	out := append([]string{}, command...)
	if !profilekit.ContainsAny(out[1:], "-s", "-S", "--silent", "--verbose", "-v") {
		out = insertBeforeURL(out, "-sS")
	}
	if !profilekit.ContainsAny(out[1:], "-i", "--include", "-I", "--head", "-D", "--dump-header") && !profilekit.ContainsPrefix(out[1:], "--dump-header=") {
		out = insertBeforeURL(out, "-i")
	}
	return out
}

func prepareHTTPie(command []string) []string {
	out := append([]string{}, command...)
	if profilekit.ContainsAny(out[1:], "--print", "-p", "--headers", "--body", "--download") || profilekit.ContainsPrefix(out[1:], "--print=") {
		return out
	}
	return insertBeforeURL(out, "--print=hb")
}

func prepareWget(command []string) []string {
	out := append([]string{}, command...)
	if profilekit.ContainsAny(out[1:], "-S", "--server-response") {
		return out
	}
	return insertBeforeURL(out, "-S")
}

func insertBeforeURL(command []string, values ...string) []string {
	idx := len(command)
	if urlIdx := firstURLIndex(command); urlIdx >= 0 {
		idx = urlIdx
	}

	out := make([]string, 0, len(command)+len(values))
	out = append(out, command[:idx]...)
	out = append(out, values...)
	out = append(out, command[idx:]...)
	return out
}

func firstURLArg(args []string) string {
	if idx := firstURLIndex(args); idx >= 0 {
		return args[idx]
	}
	return ""
}

func firstURLIndex(args []string) int {
	for i, arg := range args {
		if i == 0 {
			continue
		}
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return i
		}
	}
	return -1
}

func looksLikeAPIURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "api.") ||
		strings.Contains(lower, "/api/") ||
		strings.Contains(lower, "/v1/") ||
		strings.Contains(lower, "/v2/") ||
		strings.Contains(lower, "/graphql")
}

func hasAPIHints(args []string) bool {
	for i, arg := range args {
		lower := strings.ToLower(arg)
		switch {
		case lower == "--json",
			lower == "-d",
			lower == "--data",
			lower == "--data-raw",
			lower == "--form",
			lower == "--request",
			lower == "-x":
			return true
		case strings.Contains(lower, "application/json"),
			strings.Contains(lower, "application/vnd.api+json"),
			strings.Contains(lower, "graphql"):
			return true
		case i > 0 && isHTTPMethod(arg):
			return true
		}
	}
	return false
}

func hasCurlOutputFile(args []string) bool {
	return profilekit.ContainsAny(args[1:], "-o", "-O", "--output", "--remote-name") || profilekit.ContainsPrefix(args[1:], "--output=")
}

func isWgetStdoutFetch(args []string) bool {
	for i, arg := range args[1:] {
		switch {
		case arg == "-qO-", arg == "-O-":
			return true
		case arg == "--output-document=-":
			return true
		case arg == "-O" || arg == "--output-document":
			if i+2 < len(args) && args[i+2] == "-" {
				return true
			}
		}
	}

	return false
}

func isHTTPMethod(arg string) bool {
	switch strings.ToUpper(arg) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
