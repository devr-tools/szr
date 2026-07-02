package profiles

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

const (
	goModTopDownloads = 3
	goModMaxChanges   = 6
)

//nolint:maintidx // Profile constructors are declarative and intentionally keep match/render behavior together.
func goModProfile(maxLines int) engine.Profile {
	budgetLines := profilekit.AtLeast(maxLines, 8)
	return engine.Profile{
		Name:        "go-mod",
		Description: "Collapses module download noise and keeps dependency changes and errors.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStderrFirst,
		Budget:           profilekit.OutputBudget(budgetLines),
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match: func(inv engine.Invocation) bool {
			return matchGoModCommand(inv.Command) || matchGoModCommand(inv.Display)
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return summarizeGoModOutput(exec.Stderr+"\n"+exec.Stdout, budgetLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewBufferedTextReducerWithRecovery(true, true, func(input string) string {
				return summarizeGoModOutput(input, budget.MaxLines)
			}, func(input string) (string, string, bool) {
				return goModRecoveryInfo(input, budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseStderrFirst,
		Explain: []string{
			"Collapses runs of `go: downloading module vX.Y.Z` lines into a single downloaded-modules summary.",
			"Keeps `go: added/upgraded/removed` dependency changes (capped) and always keeps error lines.",
		},
	}
}

func matchGoModCommand(args []string) bool {
	return profilekit.HasCommand(args, "go", "mod") ||
		profilekit.HasCommand(args, "go", "get") ||
		profilekit.HasCommand(args, "go", "install")
}

func summarizeGoModOutput(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}
	lines := filters.NonEmptyLines(filters.StripANSI(input))
	if len(lines) == 0 {
		return "ok"
	}
	downloads, changes, others := classifyGoModLines(lines)
	out := renderGoModSummary(downloads, changes, others, maxLines)
	if len(out) == 0 {
		return "ok"
	}
	return strings.Join(out, "\n")
}

func renderGoModSummary(downloads, changes, others []string, maxLines int) []string {
	out := []string{}
	if len(downloads) > 0 {
		out = append(out, renderGoModDownloads(downloads))
	}
	out = append(out, limitGoModLines(changes, goModMaxChanges, "dependency changes")...)
	otherBudget := maxLines - len(out)
	if otherBudget < 1 {
		otherBudget = 1
	}
	return append(out, limitGoModLines(others, otherBudget, "lines")...)
}

func classifyGoModLines(lines []string) ([]string, []string, []string) {
	downloads := []string{}
	seenDownloads := map[string]struct{}{}
	changes := []string{}
	others := []string{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
		case strings.HasPrefix(line, "go: downloading "):
			module := goModDownloadModule(line)
			if _, ok := seenDownloads[module]; !ok && module != "" {
				seenDownloads[module] = struct{}{}
				downloads = append(downloads, module)
			}
		case isGoModChangeLine(line):
			changes = append(changes, line)
		default:
			others = append(others, line)
		}
	}
	return downloads, filters.UniqueStrings(changes), others
}

func goModDownloadModule(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "go: downloading "))
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func isGoModChangeLine(line string) bool {
	for _, prefix := range []string{
		"go: added ", "go: upgraded ", "go: downgraded ", "go: removed ", "go: found ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func renderGoModDownloads(modules []string) string {
	top := modules
	if len(top) > goModTopDownloads {
		top = top[:goModTopDownloads]
	}
	preview := strings.Join(top, ", ")
	if len(modules) > len(top) {
		preview += ", ..."
	}
	return fmt.Sprintf("downloaded %d modules (top: %s)", len(modules), preview)
}

func limitGoModLines(lines []string, limit int, noun string) []string {
	if len(lines) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 1
	}
	if len(lines) <= limit {
		return lines
	}
	out := append([]string{}, lines[:limit]...)
	return append(out, fmt.Sprintf("... +%d more %s", len(lines)-limit, noun))
}

func goModRecoveryInfo(input string, maxLines int) (string, string, bool) {
	rawLines := filters.NonEmptyLines(filters.StripANSI(input))
	renderedLines := filters.NonEmptyLines(summarizeGoModOutput(input, maxLines))
	if len(rawLines) == 0 || len(renderedLines) >= len(rawLines) {
		return filters.NoRecovery()
	}
	return filters.FullOutputRecovery("omitted module download noise")
}
