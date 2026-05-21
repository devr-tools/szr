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
	extra        int
	bytesParsed  int
	fallbackUsed bool
	pendingLine  string
	pendingCount int
	roots        []string
	stacks       []string
	hints        []string
	warnings     []string
	seenLines    map[string]struct{}
	seenStacks   map[string]struct{}
}

func NewGenericFailureReducer(maxLines, maxBytes int) *GenericFailureReducer {
	if maxLines <= 0 {
		maxLines = 12
	}
	return &GenericFailureReducer{
		maxLines:   maxLines,
		maxBytes:   maxBytes,
		head:       make([]string, 0, maxLines),
		roots:      make([]string, 0, maxLines),
		stacks:     make([]string, 0, maxLines/2),
		hints:      make([]string, 0, maxLines/2),
		warnings:   make([]string, 0, maxLines/2),
		seenLines:  map[string]struct{}{},
		seenStacks: map[string]struct{}{},
	}
}

func (r *GenericFailureReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *GenericFailureReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *GenericFailureReducer) Result() string {
	r.scanner.Finish(r.ingestLine)
	r.flushPending()
	if len(r.head) == 0 && !r.hasSignal() {
		return "ok"
	}
	if !r.hasSignal() {
		r.fallbackUsed = true
		out := append([]string{}, r.head...)
		if r.extra > 0 {
			out = append(out, fmt.Sprintf("... +%d more lines", r.extra))
		}
		return strings.Join(out, "\n")
	}
	return strings.Join(r.compose(), "\n")
}

func (r *GenericFailureReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GenericFailureReducer) FallbackUsed() bool {
	return r.fallbackUsed
}

func (r *GenericFailureReducer) Done() bool {
	signalCount := len(r.roots) + len(r.stacks) + len(r.hints) + len(r.warnings)
	return signalCount >= r.maxLines
}

func (r *GenericFailureReducer) Preview() string {
	if r.hasSignal() {
		return strings.Join(r.compose(), "\n")
	}
	if len(r.head) > 0 {
		return strings.Join(r.head, "\n")
	}
	return ""
}

func (r *GenericFailureReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.ingestLine)
}

func (r *GenericFailureReducer) ingestLine(line string) {
	if line == r.pendingLine {
		r.pendingCount++
		return
	}
	r.flushPending()
	r.pendingLine = line
	r.pendingCount = 1
}

func (r *GenericFailureReducer) flushPending() {
	if r.pendingLine == "" {
		return
	}
	line := r.pendingLine
	if r.pendingCount > 1 {
		line = fmt.Sprintf("%s (x%d)", line, r.pendingCount)
	}
	r.recordHead(line)
	r.classifyLine(r.pendingLine, line)
	r.pendingLine = ""
	r.pendingCount = 0
}

func (r *GenericFailureReducer) recordHead(line string) {
	if len(r.head) < r.maxLines {
		r.head = append(r.head, line)
	} else {
		r.extra++
	}
}

func (r *GenericFailureReducer) classifyLine(raw string, display string) {
	if len(display) > 160 {
		display = clip(display, 160)
	}
	switch classifyFailureLine(raw) {
	case "root":
		r.addUnique(&r.roots, display)
	case "stack":
		r.addStack(display, raw)
	case "hint":
		r.addUnique(&r.hints, display)
	case "warning":
		r.addUnique(&r.warnings, display)
	}
}

func (r *GenericFailureReducer) addUnique(target *[]string, line string) {
	if len(*target) >= r.maxLines {
		return
	}
	if _, ok := r.seenLines[line]; ok {
		return
	}
	r.seenLines[line] = struct{}{}
	*target = append(*target, line)
}

func (r *GenericFailureReducer) addStack(display string, raw string) {
	if len(r.stacks) >= maxFailureInt(1, r.maxLines/2) {
		return
	}
	key := stackKey(raw)
	if key == "" {
		key = display
	}
	if _, ok := r.seenStacks[key]; ok {
		return
	}
	r.seenStacks[key] = struct{}{}
	r.stacks = append(r.stacks, display)
}

func (r *GenericFailureReducer) hasSignal() bool {
	return len(r.roots) > 0 || len(r.stacks) > 0 || len(r.hints) > 0 || len(r.warnings) > 0
}

func (r *GenericFailureReducer) compose() []string {
	out := []string{}
	appendLimited := func(lines []string, limit int) {
		for _, line := range lines {
			if len(out) >= r.maxLines || limit <= 0 {
				return
			}
			out = append(out, line)
			limit--
		}
	}

	rootLimit := minInt(r.maxLines, maxFailureInt(1, r.maxLines/2+1))
	stackLimit := maxFailureInt(1, r.maxLines/3)
	hintLimit := maxFailureInt(1, r.maxLines/3)

	appendLimited(r.roots, rootLimit)
	appendLimited(r.stacks, minInt(stackLimit, r.maxLines-len(out)))
	appendLimited(r.hints, minInt(hintLimit, r.maxLines-len(out)))
	appendLimited(r.warnings, r.maxLines-len(out))
	if r.extra > 0 && len(out) < r.maxLines {
		out = append(out, fmt.Sprintf("... +%d more lines", r.extra))
	}
	return out
}

func classifyFailureLine(line string) string {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, "help:"),
		strings.HasPrefix(lower, "hint:"),
		strings.HasPrefix(lower, "note:"),
		strings.Contains(lower, "did you mean"),
		strings.Contains(lower, "available fixtures"),
		strings.Contains(lower, "try using"),
		strings.Contains(lower, "suggestion"):
		return "hint"
	case strings.HasPrefix(line, "at "),
		strings.HasPrefix(line, "#"),
		strings.HasPrefix(line, "Traceback"),
		strings.HasPrefix(line, "File \""),
		strings.Contains(lower, "stack traceback"):
		return "stack"
	case strings.Contains(line, "FAIL"),
		strings.Contains(line, "FAILED"),
		strings.Contains(line, "ERROR"),
		strings.HasPrefix(lower, "error "),
		strings.Contains(lower, "error:"),
		strings.Contains(lower, "panic"),
		strings.Contains(lower, "fatal"),
		strings.Contains(lower, "assert"),
		strings.Contains(lower, "exception"),
		strings.Contains(lower, "caused by"),
		strings.Contains(lower, "undefined reference"),
		strings.Contains(lower, "cannot "),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "does not exist"):
		return "root"
	case failureAnchor(line) != "":
		return "stack"
	case strings.Contains(lower, "warning"):
		return "warning"
	default:
		return ""
	}
}

func stackKey(line string) string {
	if anchor := failureAnchor(line); anchor != "" {
		return anchor
	}
	return strings.TrimSpace(line)
}

func failureAnchor(line string) string {
	lower := strings.ToLower(line)
	for _, ext := range []string{".go:", ".py:", ".rs:", ".ts:", ".tsx:", ".js:", ".jsx:", ".java:", ".c:", ".cc:", ".cpp:", ".h:", ".hpp:"} {
		idx := strings.Index(lower, ext)
		if idx < 0 {
			continue
		}
		start := idx
		for start > 0 && !strings.ContainsRune(" \t([{\"'", rune(line[start-1])) {
			start--
		}
		end := idx + len(ext)
		for end < len(line) && !strings.ContainsRune(" \t)]}\"'", rune(line[end])) {
			end++
		}
		return line[start:end]
	}
	return ""
}

func maxFailureInt(left, right int) int {
	if left > right {
		return left
	}
	return right
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
