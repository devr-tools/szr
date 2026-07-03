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

// NewBufferedRawTextReducer buffers the streams without stripping ANSI, for
// render functions that strip escapes themselves and self-cap to the engine
// compression contract's predicted allowance (see PredictedTokenAllowance):
// the contract budgets against the raw stream, so a pre-stripped buffer
// would make ANSI-heavy output look several times cheaper than it is and
// disarm the self-cap exactly when it matters.
func NewBufferedRawTextReducer(
	stdoutEnabled, stderrEnabled bool,
	render func(string) string,
	recovery func(string) (string, string, bool),
) *BufferedTextReducer {
	reducer := NewBufferedTextReducerWithRecovery(stdoutEnabled, stderrEnabled, render, recovery)
	reducer.stdout.keepANSI = true
	reducer.stderr.keepANSI = true
	return reducer
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
	stdout := r.stdout.String()
	stderr := r.stderr.String()
	return strings.TrimSpace(stdout + joinReducerStreams(stdout, stderr))
}

func joinReducerStreams(stdout, stderr string) string {
	switch {
	case stdout == "" || stderr == "":
		return stderr
	default:
		return "\n" + stderr
	}
}
