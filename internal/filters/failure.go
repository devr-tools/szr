package filters

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func SummarizeGenericFailure(input string, maxLines int) string {
	reducer := NewGenericFailureReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

type GenericFailureReducer struct {
	scanner      lineScanner
	maxLines     int
	maxBytes     int
	head         []string
	interesting  []string
	extra        int
	bytesParsed  int
	fallbackUsed bool
}

func NewGenericFailureReducer(maxLines, maxBytes int) *GenericFailureReducer {
	if maxLines <= 0 {
		maxLines = 12
	}
	return &GenericFailureReducer{
		maxLines:    maxLines,
		maxBytes:    maxBytes,
		head:        make([]string, 0, maxLines),
		interesting: make([]string, 0, maxLines),
	}
}

func (r *GenericFailureReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *GenericFailureReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *GenericFailureReducer) Result() string {
	r.scanner.Finish(r.recordLine)
	if len(r.head) == 0 && len(r.interesting) == 0 {
		return "ok"
	}
	if len(r.interesting) == 0 {
		r.fallbackUsed = true
		out := append([]string{}, r.head...)
		if r.extra > 0 {
			out = append(out, fmt.Sprintf("... +%d more lines", r.extra))
		}
		return strings.Join(out, "\n")
	}
	return strings.Join(r.interesting, "\n")
}

func (r *GenericFailureReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GenericFailureReducer) FallbackUsed() bool {
	return r.fallbackUsed
}

func (r *GenericFailureReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.recordLine)
}

func (r *GenericFailureReducer) recordLine(line string) {
	if len(r.head) < r.maxLines {
		r.head = append(r.head, line)
	} else {
		r.extra++
	}

	if len(r.interesting) >= r.maxLines {
		return
	}
	for _, keyword := range []string{"FAIL", "ERROR", "Error", "error", "panic", "warning", "Warning"} {
		if strings.Contains(line, keyword) {
			if r.maxBytes > 0 {
				line = clip(line, minInt(160, r.maxBytes))
			} else {
				line = clip(line, 160)
			}
			r.interesting = append(r.interesting, line)
			return
		}
	}
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
