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
	reducer := NewRipgrepReducer(maxGroups, maxGroups*4)
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

type ripgrepGroup struct {
	count    int
	previews []string
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
	if r.extraFiles == 0 && suppressedBucketTotal(r.suppressed) == 0 {
		return false
	}
	totalPreviews := 0
	for _, file := range r.order {
		group := r.groups[file]
		if group == nil || len(group.previews) == 0 {
			return false
		}
		totalPreviews += len(group.previews)
	}
	return totalPreviews >= minInt(r.maxLines-len(r.order), r.maxGroups+1)
}

func (r *RipgrepReducer) Preview() string {
	return r.render(true)
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
		group = &ripgrepGroup{previews: make([]string, 0, 3)}
		r.groups[file] = group
		r.order = append(r.order, file)
	}
	group.count++
	if len(group.previews) < 3 {
		group.previews = append(group.previews, fmt.Sprintf("  %d: %s", lineNo, clip(text, 120)))
	}
}

func (r *RipgrepReducer) render(preview bool) string {
	if len(r.order) == 0 {
		if stderr := strings.TrimSpace(r.stderrReducer.Preview()); stderr != "" {
			return stderr
		}
		return "no matches"
	}
	out := make([]string, 0, r.maxLines+1)
	footerLines := 0
	if r.extraFiles > 0 {
		footerLines++
	}
	if summarizeSuppressedSearchBuckets(r.suppressed) != "" {
		footerLines++
	}
	for _, file := range r.order {
		group := r.groups[file]
		if group == nil {
			continue
		}
		reserve := footerLines
		if extra := group.count - len(group.previews); extra > 0 {
			reserve++
		}
		if len(out)+1+reserve > r.maxLines {
			break
		}
		out = append(out, fmt.Sprintf("%s (%d matches)", file, group.count))
		availablePreviews := maxInt(0, r.maxLines-len(out)-reserve)
		if availablePreviews > len(group.previews) {
			availablePreviews = len(group.previews)
		}
		out = append(out, group.previews[:availablePreviews]...)
		if extra := group.count - availablePreviews; extra > 0 && len(out) < r.maxLines-reserve+1 {
			out = append(out, fmt.Sprintf("  ... +%d more", extra))
		}
	}
	if r.extraFiles > 0 && len(out) < r.maxLines {
		out = append(out, fmt.Sprintf("... +%d more files", r.extraFiles))
	}
	if line := summarizeSuppressedSearchBuckets(r.suppressed); line != "" && len(out) < r.maxLines+1 {
		out = append(out, line)
	}
	if preview && len(out) > r.maxLines {
		out = out[:r.maxLines]
	}
	return strings.Join(out, "\n")
}

func suppressedBucketTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
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
