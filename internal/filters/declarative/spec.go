package declarative

import "strconv"

type Spec struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	KeepPatterns  []string `json:"keep_patterns,omitempty"`
	StripPatterns []string `json:"strip_patterns,omitempty"`
	Head          int      `json:"head,omitempty"`
	Tail          int      `json:"tail,omitempty"`
	MaxLineWidth  int      `json:"max_line_width,omitempty"`
	DropEmpty     bool     `json:"drop_empty,omitempty"`
	EmptyMessage  string   `json:"empty_message,omitempty"`
}

type Options struct {
	LineLimit int
}

type Result struct {
	Text          string
	TotalLines    int
	VisibleLines  int
	OmittedBefore int
	OmittedAfter  int
	Empty         bool
}

func (r Result) OmittedCount() int {
	return r.OmittedBefore + r.OmittedAfter
}

func (r Result) RecoverySummary(noun string) string {
	omitted := r.OmittedCount()
	if omitted <= 0 {
		return ""
	}
	if noun == "" {
		noun = "lines"
	}
	if omitted == 1 {
		noun = singularize(noun)
	}
	return "omitted " + strconv.Itoa(omitted) + " additional " + noun
}

func singularize(noun string) string {
	if len(noun) > 1 && noun[len(noun)-1] == 's' {
		return noun[:len(noun)-1]
	}
	return noun
}
