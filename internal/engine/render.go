package engine

import "strings"

type renderResult struct {
	text         string
	bytesParsed  int
	fallbackUsed bool
}

func renderProfile(profile Profile, inv Invocation, exec Execution, fallbackLines int, passthrough bool) renderResult {
	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	if passthrough {
		return renderResult{
			text:        rawCombined,
			bytesParsed: len(rawCombined),
		}
	}

	if profile.StreamRender != nil {
		budget := ResolveBudget(profile, inv, fallbackLines)
		reducer := profile.StreamRender(inv, budget)
		if reducer != nil {
			feedReducer(profile.StreamPreference, reducer, exec)
			text := reducer.Result()
			if strings.TrimSpace(text) == "" {
				text = rawCombined
			}
			return renderResult{
				text:         text,
				bytesParsed:  maxInt(reducer.BytesParsed(), 0),
				fallbackUsed: reducer.FallbackUsed() || profile.Name == "passthrough",
			}
		}
	}

	rendered := rawCombined
	if profile.Render != nil {
		rendered = profile.Render(inv, exec)
	}
	fallbackUsed := false
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
		fallbackUsed = true
	}
	if profile.Name == "passthrough" {
		fallbackUsed = true
	}
	bytesParsed := len(rawCombined)
	if profile.ParseBytes != nil {
		bytesParsed = maxInt(profile.ParseBytes(exec), 0)
	}
	return renderResult{
		text:         rendered,
		bytesParsed:  bytesParsed,
		fallbackUsed: fallbackUsed,
	}
}

func feedReducer(preference string, reducer StreamReducer, exec Execution) {
	switch preference {
	case StreamStdoutOnly:
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
	case StreamStderrOnly:
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
	case StreamStderrFirst:
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
	default:
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
