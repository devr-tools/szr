package filters

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

func SummarizeRipgrep(input string, maxGroups, maxLines int) string {
	grouped := GroupRipgrep(input, maxGroups)
	if grouped != "no matches" {
		return grouped
	}
	return SummarizeGenericFailure(input, maxLines)
}

func GroupRipgrep(input string, maxGroups int) string {
	reducer := NewRipgrepReducer(maxGroups, maxGroups*2)
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		reducer.ingestLine(scanner.Text())
	}
	return reducer.render(false)
}

type RipgrepReducer struct {
	stdoutScanner lineScanner
	stderrReducer *CompactLineReducer
	maxGroups     int
	maxLines      int
	bytesParsed   int
	groups        map[string]*ripgrepGroup
	order         []string
	totalMatches  int
	extraFiles    int
	suppressed    map[string]int
}

// ripgrepMatchTextLimit clips the representative match text kept per file.
const ripgrepMatchTextLimit = 120

type ripgrepGroup struct {
	count       int
	firstLineNo int
	firstText   string
}

func NewRipgrepReducer(maxGroups, maxLines int) *RipgrepReducer {
	if maxGroups <= 0 {
		maxGroups = 4
	}
	if maxLines <= 0 {
		maxLines = maxGroups * 4
	}
	return &RipgrepReducer{
		stderrReducer: NewCompactLineReducer(maxLines, 0),
		maxGroups:     maxGroups,
		maxLines:      maxLines,
		groups:        map[string]*ripgrepGroup{},
		order:         make([]string, 0, maxGroups),
		suppressed:    map[string]int{},
	}
}

func (r *RipgrepReducer) ConsumeStdout(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stdoutScanner.Consume(chunk, r.ingestLine)
}

func (r *RipgrepReducer) ConsumeStderr(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stderrReducer.ConsumeStderr(chunk)
}

func (r *RipgrepReducer) Result() string {
	r.stdoutScanner.Finish(r.ingestLine)
	return r.render(false)
}

func (r *RipgrepReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *RipgrepReducer) FallbackUsed() bool {
	return false
}

func (r *RipgrepReducer) Done() bool {
	if strings.TrimSpace(r.stderrReducer.Preview()) != "" || len(r.order) < r.maxGroups {
		return false
	}
	return r.extraFiles > 0 || suppressedBucketTotal(r.suppressed) > 0
}

func (r *RipgrepReducer) Preview() string {
	return r.render(true)
}

func (r *RipgrepReducer) RecoveryInfo() (string, string, bool) {
	r.stdoutScanner.Finish(r.ingestLine)
	omittedMatches, omittedFiles := r.omittedCounts()
	parts := make([]string, 0, 2)
	if omittedMatches > 0 {
		parts = append(parts, fmt.Sprintf("%d additional matches", omittedMatches))
	}
	if omittedFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d additional files", omittedFiles))
	}
	if len(parts) == 0 {
		return NoRecovery()
	}
	return FullOutputRecovery("omitted " + strings.Join(parts, ", "))
}

func (r *RipgrepReducer) ingestLine(line string) {
	file, lineNo, text, ok := parseRipgrepMatch(line)
	if !ok {
		return
	}
	if bucket := searchReducerNoiseBucket(file); bucket != "" {
		r.suppressed[bucket]++
		return
	}
	r.totalMatches++
	group := r.groups[file]
	if group == nil {
		if len(r.order) >= r.maxGroups {
			r.extraFiles++
			return
		}
		group = &ripgrepGroup{firstLineNo: lineNo, firstText: Clip(text, ripgrepMatchTextLimit)}
		r.groups[file] = group
		r.order = append(r.order, file)
	}
	group.count++
}

func (r *RipgrepReducer) render(preview bool) string {
	if len(r.order) == 0 {
		if stderr := strings.TrimSpace(r.stderrReducer.Preview()); stderr != "" {
			return stderr
		}
		return "no matches"
	}
	out := make([]string, 0, r.maxLines+1)
	out = append(out, fmt.Sprintf("%d matches across %d files", r.totalMatches, len(r.order)+r.extraFiles))
	for _, file := range r.order {
		group := r.groups[file]
		if group == nil {
			continue
		}
		if len(out)+1+r.footerLines() > r.maxLines {
			break
		}
		out = append(out, formatRipgrepGroup(file, group))
	}
	out = r.appendFooter(out)
	if preview && len(out) > r.maxLines {
		out = out[:r.maxLines]
	}
	return strings.Join(out, "\n")
}

func formatRipgrepGroup(file string, group *ripgrepGroup) string {
	if group.firstText == "" {
		return fmt.Sprintf("%s:%d (%d matches)", file, group.firstLineNo, group.count)
	}
	return fmt.Sprintf("%s:%d: %s (%d matches)", file, group.firstLineNo, group.firstText, group.count)
}

func (r *RipgrepReducer) footerLines() int {
	lines := 0
	if r.extraFiles > 0 {
		lines++
	}
	if r.suppressedSummary() != "" {
		lines++
	}
	return lines
}

func (r *RipgrepReducer) suppressedSummary() string {
	return summarizeSuppressedSearchBuckets(r.suppressed)
}

func (r *RipgrepReducer) appendFooter(out []string) []string {
	if r.extraFiles > 0 && len(out) < r.maxLines {
		out = append(out, fmt.Sprintf("... +%d more files", r.extraFiles))
	}
	if summary := r.suppressedSummary(); summary != "" && len(out) < r.maxLines+1 {
		out = append(out, summary)
	}
	return out
}

func suppressedBucketTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func (r *RipgrepReducer) omittedCounts() (matches int, files int) {
	files = r.extraFiles
	return matches, files
}

func parseRipgrepMatch(line string) (string, int, string, bool) {
	for idx := 0; idx < len(line)-2; idx++ {
		if line[idx] != ':' {
			continue
		}
		next := idx + 1
		end := next
		for end < len(line) && line[end] >= '0' && line[end] <= '9' {
			end++
		}
		if end == next || end >= len(line) || line[end] != ':' {
			continue
		}
		lineNo, err := strconv.Atoi(line[next:end])
		if err != nil {
			continue
		}
		file := strings.TrimSpace(line[:idx])
		text := strings.TrimSpace(line[end+1:])
		if file == "" {
			return "", 0, "", false
		}
		return file, lineNo, text, true
	}
	return "", 0, "", false
}
