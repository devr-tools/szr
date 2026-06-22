package git

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeGitAdd(inv engine.Invocation, input string) string {
	summary, ok, _ := summarizeGitAdd(inv, input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitCommit(inv engine.Invocation, input string) string {
	summary, ok, _ := summarizeGitCommit(inv, input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitPush(input string) string {
	summary, ok, _ := summarizeGitPush(input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

func SummarizeGitPull(input string) string {
	summary, ok, _ := summarizeGitPull(input)
	if !ok {
		return shared.CompactLines(input, 6)
	}
	return summary
}

type GitSuccessPathReducer struct {
	mode        string
	inv         engine.Invocation
	maxLines    int
	bytesParsed int
	stdoutScan  shared.LineScanner
	stderrScan  shared.LineScanner
	stdoutLines []string
	stderrLines []string
	finalized   bool
	summary     string
	matched     bool
	omitted     bool
}

func NewGitSuccessPathReducer(mode string, inv engine.Invocation, maxLines, _ int) *GitSuccessPathReducer {
	if maxLines <= 0 {
		maxLines = 6
	}
	return &GitSuccessPathReducer{
		mode:        mode,
		inv:         inv,
		maxLines:    maxLines,
		stdoutLines: make([]string, 0, maxLines),
		stderrLines: make([]string, 0, maxLines),
	}
}

func (r *GitSuccessPathReducer) ConsumeStdout(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stdoutScan.Consume(chunk, func(line string) {
		r.stdoutLines = append(r.stdoutLines, line)
	})
}

func (r *GitSuccessPathReducer) ConsumeStderr(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.stderrScan.Consume(chunk, func(line string) {
		r.stderrLines = append(r.stderrLines, line)
	})
}

func (r *GitSuccessPathReducer) Result() string {
	r.finish()
	if r.matched {
		return r.summary
	}
	return shared.CompactLines(r.input(), r.maxLines)
}

func (r *GitSuccessPathReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *GitSuccessPathReducer) FallbackUsed() bool {
	r.finish()
	return !r.matched
}

func (r *GitSuccessPathReducer) Preview() string {
	r.finish()
	if r.matched {
		return r.summary
	}
	return strings.TrimSpace(shared.JoinLimitedLines(r.lines(), r.maxLines))
}

func (r *GitSuccessPathReducer) RecoveryInfo() (string, string, bool) {
	r.finish()
	if !r.matched || !r.omitted {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted git %s success details", r.mode))
}

func (r *GitSuccessPathReducer) finish() {
	if r.finalized {
		return
	}
	r.stdoutScan.Finish(func(line string) {
		r.stdoutLines = append(r.stdoutLines, line)
	})
	r.stderrScan.Finish(func(line string) {
		r.stderrLines = append(r.stderrLines, line)
	})
	r.summary, r.matched, r.omitted = summarizeGitSuccess(r.mode, r.inv, r.input())
	r.finalized = true
}
