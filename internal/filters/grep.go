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
	totalPreviews := 0
	for _, file := range r.order {
		group := r.groups[file]
		if group == nil || len(group.previews) == 0 {
			return false
		}
		totalPreviews += len(group.previews)
	}
	return totalPreviews >= r.maxGroups+1
}

func (r *RipgrepReducer) Preview() string {
	return r.render(true)
}

func (r *RipgrepReducer) ingestLine(line string) {
	file, lineNo, text, ok := parseRipgrepMatch(line)
	if !ok {
		return
	}
	if bucket := SearchNoiseBucket(file); bucket != "" {
		r.suppressed[bucket]++
		return
	}
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
	out := make([]string, 0, r.maxLines)
	for _, file := range r.order {
		group := r.groups[file]
		if group == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%d matches)", file, group.count))
		out = append(out, group.previews...)
		if extra := group.count - len(group.previews); extra > 0 {
			out = append(out, fmt.Sprintf("  ... +%d more", extra))
		}
		if len(out) >= r.maxLines {
			break
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
