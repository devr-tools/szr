package filters

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func SummarizeGenericFailure(input string, maxLines int) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "ok"
	}

	interesting := []string{}
	keywords := []string{"FAIL", "ERROR", "Error", "error", "panic", "warning", "Warning"}
	for _, line := range lines {
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				interesting = append(interesting, clip(line, 160))
				break
			}
		}
	}
	if len(interesting) == 0 {
		return CompactLines(input, maxLines)
	}
	if len(interesting) > maxLines {
		interesting = interesting[:maxLines]
	}
	return strings.Join(interesting, "\n")
}

func SummarizeGoTestJSON(input string) string {
	type event struct {
		Time    string `json:"Time"`
		Action  string `json:"Action"`
		Package string `json:"Package"`
		Test    string `json:"Test"`
		Output  string `json:"Output"`
	}

	type packageState struct {
		Passed bool
		Failed bool
	}

	failures := map[string][]string{}
	packages := map[string]*packageState{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		var ev event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Package != "" {
			if _, ok := packages[ev.Package]; !ok {
				packages[ev.Package] = &packageState{}
			}
		}
		switch ev.Action {
		case "fail":
			if ev.Test != "" {
				failures[ev.Package] = append(failures[ev.Package], ev.Test)
			} else if pkg := packages[ev.Package]; pkg != nil {
				pkg.Failed = true
			}
		case "pass":
			if ev.Test == "" {
				if pkg := packages[ev.Package]; pkg != nil {
					pkg.Passed = true
				}
			}
		case "output":
			if ev.Test != "" && strings.Contains(strings.ToLower(ev.Output), "panic") {
				failures[ev.Package] = append(failures[ev.Package], clip(strings.TrimSpace(ev.Output), 160))
			}
		}
	}

	if len(packages) == 0 {
		return CompactLines(input, 12)
	}

	passed := 0
	failed := 0
	for _, pkg := range packages {
		if pkg.Failed {
			failed++
		} else if pkg.Passed {
			passed++
		}
	}

	var out []string
	out = append(out, fmt.Sprintf("packages: pass=%d fail=%d", passed, failed))
	if len(failures) == 0 {
		out = append(out, "all tests passed")
		return strings.Join(out, "\n")
	}

	keys := make([]string, 0, len(failures))
	for key := range failures {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		unique := uniqueStrings(failures[key])
		out = append(out, fmt.Sprintf("%s", key))
		for i, testName := range unique {
			if i >= 4 {
				out = append(out, fmt.Sprintf("  ... +%d more", len(unique)-4))
				break
			}
			out = append(out, "  "+testName)
		}
	}
	return strings.Join(out, "\n")
}
