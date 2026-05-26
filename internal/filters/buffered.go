package filters

import "strings"

type BufferedTextReducer struct {
	stdoutEnabled bool
	stderrEnabled bool
	render        func(string) string
	recovery      func(string) (string, string, bool)
	bytesParsed   int
	stdout        textBuffer
	stderr        textBuffer
}

func NewBufferedTextReducer(stdoutEnabled, stderrEnabled bool, render func(string) string) *BufferedTextReducer {
	return NewBufferedTextReducerWithRecovery(stdoutEnabled, stderrEnabled, render, nil)
}

func NewBufferedTextReducerWithRecovery(
	stdoutEnabled, stderrEnabled bool,
	render func(string) string,
	recovery func(string) (string, string, bool),
) *BufferedTextReducer {
	return &BufferedTextReducer{
		stdoutEnabled: stdoutEnabled,
		stderrEnabled: stderrEnabled,
		render:        render,
		recovery:      recovery,
	}
}

func (r *BufferedTextReducer) ConsumeStdout(chunk []byte) {
	if !r.stdoutEnabled {
		return
	}
	r.bytesParsed += len(chunk)
	r.stdout.Consume(chunk)
}

func (r *BufferedTextReducer) ConsumeStderr(chunk []byte) {
	if !r.stderrEnabled {
		return
	}
	r.bytesParsed += len(chunk)
	r.stderr.Consume(chunk)
}

func (r *BufferedTextReducer) Result() string {
	return r.render(r.input())
}

func (r *BufferedTextReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *BufferedTextReducer) FallbackUsed() bool {
	return false
}

func (r *BufferedTextReducer) RecoveryInfo() (string, string, bool) {
	if r.recovery == nil {
		return NoRecovery()
	}
	return r.recovery(r.input())
}

func (r *BufferedTextReducer) input() string {
	return strings.TrimSpace(r.stdout.String() + joinReducerStreams(r.stdout.String(), r.stderr.String()))
}

func joinReducerStreams(stdout, stderr string) string {
	switch {
	case stdout == "" || stderr == "":
		return stderr
	default:
		return "\n" + stderr
	}
}
