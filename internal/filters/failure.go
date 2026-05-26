package filters

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func SummarizeGenericFailure(input string, maxLines int) string {
	return SummarizeGenericFailureWithOptions(input, GenericFailureReducerOptions{
		MaxLines:           maxLines,
		NoisePrefiltering:  true,
		SemanticCompaction: true,
	})
}

func SummarizeGenericFailureWithOptions(input string, opts GenericFailureReducerOptions) string {
	reducer := NewGenericFailureReducerWithOptions(opts)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

type GenericFailureReducer struct {
	scanner            lineScanner
	maxLines           int
	maxBytes           int
	minFailures        int
	minAnchors         int
	minHints           int
	noisePrefiltering  bool
	semanticCompaction bool
	head               []string
	extra              int
	bytesParsed        int
	fallbackUsed       bool
	pendingLine        string
	pendingCount       int
	roots              []failureItem
	stacks             []failureItem
	hints              []failureItem
	warnings           []failureItem
	seenRoots          map[string]int
	seenStacks         map[string]int
	seenHints          map[string]int
	seenWarnings       map[string]int
	droppedNoise       map[string]int
}

type failureItem struct {
	Text    string
	Similar int
}

type failureLine struct {
	Kind      string
	Display   string
	Key       string
	DropClass string
}

type GenericFailureReducerOptions struct {
	MaxLines           int
	MaxBytes           int
	MinFailures        int
	MinAnchors         int
	MinHints           int
	NoisePrefiltering  bool
	SemanticCompaction bool
}

func NewGenericFailureReducer(maxLines, maxBytes int) *GenericFailureReducer {
	return NewGenericFailureReducerWithOptions(GenericFailureReducerOptions{
		MaxLines:           maxLines,
		MaxBytes:           maxBytes,
		NoisePrefiltering:  true,
		SemanticCompaction: true,
	})
}

func NewGenericFailureReducerWithContract(maxLines, maxBytes, minFailures, minAnchors, minHints int) *GenericFailureReducer {
	return NewGenericFailureReducerWithOptions(GenericFailureReducerOptions{
		MaxLines:           maxLines,
		MaxBytes:           maxBytes,
		MinFailures:        minFailures,
		MinAnchors:         minAnchors,
		MinHints:           minHints,
		NoisePrefiltering:  true,
		SemanticCompaction: true,
	})
}

func NewGenericFailureReducerWithOptions(opts GenericFailureReducerOptions) *GenericFailureReducer {
	if opts.MaxLines <= 0 {
		opts.MaxLines = 12
	}
	return &GenericFailureReducer{
		maxLines:           opts.MaxLines,
		maxBytes:           opts.MaxBytes,
		minFailures:        opts.MinFailures,
		minAnchors:         opts.MinAnchors,
		minHints:           opts.MinHints,
		noisePrefiltering:  opts.NoisePrefiltering,
		semanticCompaction: opts.SemanticCompaction,
		head:               make([]string, 0, opts.MaxLines),
		roots:              make([]failureItem, 0, opts.MaxLines),
		stacks:             make([]failureItem, 0, opts.MaxLines/2),
		hints:              make([]failureItem, 0, opts.MaxLines/2),
		warnings:           make([]failureItem, 0, opts.MaxLines/2),
		seenRoots:          map[string]int{},
		seenStacks:         map[string]int{},
		seenHints:          map[string]int{},
		seenWarnings:       map[string]int{},
		droppedNoise:       map[string]int{},
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
		if summary := r.omissionSummary(len(out)); summary != "" {
			out = append(out, summary)
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
	r.classifyLine(r.pendingLine, r.pendingCount)
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

func (r *GenericFailureReducer) classifyLine(raw string, count int) {
	line := analyzeFailureLine(raw, r.noisePrefiltering, r.semanticCompaction)
	if line.DropClass != "" {
		r.droppedNoise[line.DropClass] += count
		return
	}
	if line.Kind == "" {
		display := line.Display
		if count > 1 {
			display = fmt.Sprintf("%s (x%d)", display, count)
		}
		r.recordHead(display)
		return
	}
	if count > 1 {
		defer r.bumpSimilar(line.Kind, line.Key, count-1)
	}
	switch line.Kind {
	case "root":
		r.addSemantic(&r.roots, r.seenRoots, line.Key, line.Display, r.maxLines)
	case "stack":
		r.addSemantic(&r.stacks, r.seenStacks, line.Key, line.Display, maxFailureInt(1, r.maxLines/2))
	case "hint":
		r.addSemantic(&r.hints, r.seenHints, line.Key, line.Display, maxFailureInt(1, r.maxLines/2))
	case "warning":
		r.addSemantic(&r.warnings, r.seenWarnings, line.Key, line.Display, maxFailureInt(1, r.maxLines/2))
	}
}

func (r *GenericFailureReducer) addSemantic(target *[]failureItem, seen map[string]int, key string, line string, limit int) {
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(line))
	}
	if idx, ok := seen[key]; ok {
		(*target)[idx].Similar++
		return
	}
	if len(*target) >= limit {
		r.extra++
		return
	}
	seen[key] = len(*target)
	*target = append(*target, failureItem{Text: line})
}

func (r *GenericFailureReducer) bumpSimilar(kind string, key string, similar int) {
	if similar <= 0 || key == "" {
		return
	}
	switch kind {
	case "root":
		if idx, ok := r.seenRoots[key]; ok {
			r.roots[idx].Similar += similar
		}
	case "stack":
		if idx, ok := r.seenStacks[key]; ok {
			r.stacks[idx].Similar += similar
		}
	case "hint":
		if idx, ok := r.seenHints[key]; ok {
			r.hints[idx].Similar += similar
		}
	case "warning":
		if idx, ok := r.seenWarnings[key]; ok {
			r.warnings[idx].Similar += similar
		}
	}
}

func (r *GenericFailureReducer) hasSignal() bool {
	return len(r.roots) > 0 || len(r.stacks) > 0 || len(r.hints) > 0 || len(r.warnings) > 0
}

func (r *GenericFailureReducer) compose() []string {
	out := []string{}
	appendLimited := func(lines []failureItem, limit int, label string) {
		for _, line := range lines {
			if len(out) >= r.maxLines || limit <= 0 {
				return
			}
			out = append(out, renderFailureItem(line, label))
			limit--
		}
	}

	rootLimit := minInt(r.maxLines, maxFailureInt(maxFailureInt(1, r.maxLines/2+1), r.minFailures))
	stackLimit := maxFailureInt(maxFailureInt(1, r.maxLines/3), r.minAnchors)
	hintLimit := maxFailureInt(maxFailureInt(1, r.maxLines/3), r.minHints)

	appendLimited(r.roots, rootLimit, "line")
	appendLimited(r.stacks, minInt(stackLimit, r.maxLines-len(out)), "frame")
	appendLimited(r.hints, minInt(hintLimit, r.maxLines-len(out)), "hint")
	appendLimited(r.warnings, r.maxLines-len(out), "warning")
	if summary := r.omissionSummary(len(out)); summary != "" && len(out) < r.maxLines {
		out = append(out, summary)
	}
	return out
}

func (r *GenericFailureReducer) omissionSummary(used int) string {
	parts := []string{}
	if r.extra > 0 {
		parts = append(parts, fmt.Sprintf("%d more lines", r.extra))
	}
	if count := r.droppedNoise["progress"]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d progress lines", count))
	}
	if count := r.droppedNoise["install"]; count > 0 {
		parts = append(parts, fmt.Sprintf("%d install lines", count))
	}
	if len(parts) == 0 || used >= r.maxLines {
		return ""
	}
	return "... omitted " + strings.Join(parts, ", ")
}

func renderFailureItem(item failureItem, label string) string {
	if item.Similar <= 0 {
		return item.Text
	}
	suffix := "lines"
	switch label {
	case "frame":
		suffix = "frames"
	case "hint":
		suffix = "hints"
	case "warning":
		suffix = "warnings"
	}
	return fmt.Sprintf("%s (+%d similar %s)", item.Text, item.Similar, suffix)
}

func analyzeFailureLine(raw string, noisePrefiltering bool, semanticCompaction bool) failureLine {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return failureLine{DropClass: "blank"}
	}
	if noisePrefiltering {
		if dropClass := failureNoiseClass(trimmed); dropClass != "" {
			return failureLine{DropClass: dropClass}
		}
	}
	display := trimmed
	if semanticCompaction {
		display = compactFailureDisplay(trimmed)
	}
	kind := classifyFailureLine(trimmed)
	key := strings.ToLower(strings.TrimSpace(display))
	if semanticCompaction {
		key = failureSemanticKey(trimmed, display, kind)
	}
	return failureLine{
		Kind:    kind,
		Display: clip(display, 160),
		Key:     key,
	}
}

func failureSemanticKey(raw string, display string, kind string) string {
	switch kind {
	case "stack":
		if key := stackKey(raw); key != "" {
			return strings.ToLower(key)
		}
	case "root", "warning":
		if anchor := failureAnchor(raw); anchor != "" {
			return strings.ToLower(shortenFailurePath(anchor))
		}
	}
	return strings.ToLower(strings.TrimSpace(display))
}

func failureNoiseClass(line string) string {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case looksLikeProgressLine(lower):
		return "progress"
	case looksLikeInstallNoise(lower):
		return "install"
	default:
		return ""
	}
}

func looksLikeProgressLine(lower string) bool {
	switch {
	case strings.HasPrefix(lower, "progress:"),
		strings.HasPrefix(lower, "resolving:"),
		strings.HasPrefix(lower, "downloading "),
		strings.HasPrefix(lower, "downloading:"),
		strings.HasPrefix(lower, "fetching "),
		strings.HasPrefix(lower, "fetching:"),
		strings.HasPrefix(lower, "extracting "),
		strings.HasPrefix(lower, "linking dependencies"),
		strings.HasPrefix(lower, "transforming ("),
		strings.HasPrefix(lower, "bundling "),
		strings.HasPrefix(lower, "building modules"),
		strings.HasPrefix(lower, "copying files"):
		return true
	case strings.HasPrefix(lower, "[") && strings.Contains(lower, "/") && strings.Contains(lower, "]") &&
		(strings.Contains(lower, "build") || strings.Contains(lower, "download") || strings.Contains(lower, "fetch") || strings.Contains(lower, "link")):
		return true
	case strings.Contains(lower, "%") &&
		(strings.Contains(lower, "download") || strings.Contains(lower, "fetch") || strings.Contains(lower, "install") || strings.Contains(lower, "progress")):
		return true
	default:
		return false
	}
}

func looksLikeInstallNoise(lower string) bool {
	switch {
	case strings.HasPrefix(lower, "added ") && strings.Contains(lower, " package"),
		strings.HasPrefix(lower, "removed ") && strings.Contains(lower, " package"),
		strings.HasPrefix(lower, "changed ") && strings.Contains(lower, " package"),
		strings.HasPrefix(lower, "audited ") && strings.Contains(lower, " package"),
		strings.HasPrefix(lower, "up to date"),
		strings.HasPrefix(lower, "lockfile is up to date"),
		strings.HasPrefix(lower, "already up to date"),
		strings.HasPrefix(lower, "npm notice"),
		strings.HasPrefix(lower, "npm fund"),
		strings.HasPrefix(lower, "done in "),
		strings.Contains(lower, "packages are looking for funding"),
		strings.Contains(lower, "found 0 vulnerabilities"):
		return true
	default:
		return false
	}
}

func compactFailureDisplay(line string) string {
	if anchor := failureAnchor(line); anchor != "" {
		return strings.Replace(line, anchor, shortenFailurePath(anchor), 1)
	}
	return line
}

func shortenFailurePath(anchor string) string {
	idx := strings.Index(anchor, ":")
	pathPart := anchor
	suffix := ""
	if idx >= 0 {
		pathPart = anchor[:idx]
		suffix = anchor[idx:]
	}
	segments := strings.FieldsFunc(pathPart, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(segments) <= 3 {
		return pathPart + suffix
	}
	return ".../" + strings.Join(segments[len(segments)-3:], "/") + suffix
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
	for _, ext := range []string{".go:", ".py:", ".rs:", ".ts:", ".tsx:", ".mts:", ".cts:", ".js:", ".jsx:", ".mjs:", ".cjs:", ".php:", ".phtml:", ".java:", ".c:", ".cc:", ".cpp:", ".h:", ".hpp:"} {
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

type goTestEvent struct {
	Time    string `json:"Time"`
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

type goTestPackageState struct {
	Passed bool
	Failed bool
}

func SummarizeGoTestJSON(input string) string {
	failures := map[string][]string{}
	packages := map[string]*goTestPackageState{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		var ev goTestEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		applyGoTestEvent(packages, failures, ev)
	}

	if len(packages) == 0 {
		return CompactLines(input, 12)
	}

	passed, failed := countGoTestPackages(packages)
	return renderGoTestSummary(passed, failed, failures)
}

func applyGoTestEvent(packages map[string]*goTestPackageState, failures map[string][]string, ev goTestEvent) {
	pkg := ensureGoTestPackageState(packages, ev.Package)
	switch ev.Action {
	case "fail":
		if ev.Test != "" {
			failures[ev.Package] = append(failures[ev.Package], ev.Test)
		} else if pkg != nil {
			pkg.Failed = true
		}
	case "pass":
		if ev.Test == "" && pkg != nil {
			pkg.Passed = true
		}
	case "output":
		if ev.Test != "" && strings.Contains(strings.ToLower(ev.Output), "panic") {
			failures[ev.Package] = append(failures[ev.Package], clip(strings.TrimSpace(ev.Output), 160))
		}
	}
}

func ensureGoTestPackageState(packages map[string]*goTestPackageState, pkg string) *goTestPackageState {
	if pkg == "" {
		return nil
	}
	if packages[pkg] == nil {
		packages[pkg] = &goTestPackageState{}
	}
	return packages[pkg]
}

func countGoTestPackages(packages map[string]*goTestPackageState) (int, int) {
	passed := 0
	failed := 0
	for _, pkg := range packages {
		if pkg.Failed {
			failed++
		} else if pkg.Passed {
			passed++
		}
	}
	return passed, failed
}

func renderGoTestSummary(passed, failed int, failures map[string][]string) string {
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
