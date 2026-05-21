package engine

func (e *Engine) match(inv Invocation) Profile {
	for _, profile := range e.profiles {
		if profile.Match != nil && profile.Match(inv) {
			return profile
		}
	}
	return fallbackProfile()
}

func fallbackProfile() Profile {
	return Profile{
		Name:             "passthrough",
		Description:      "Raw command passthrough with trimming.",
		Source:           SourceFallback,
		Confidence:       ConfidenceLow,
		StreamPreference: StreamStdoutFirst,
		Budget:           OutputBudget{MaxLines: 12, MaxBytes: 12 * 160, MaxTokens: 12 * 32},
		Render: func(_ Invocation, exec Execution) string {
			return combineStreams(exec.Stdout, exec.Stderr)
		},
		ParseBytes: func(exec Execution) int {
			return len(combineStreams(exec.Stdout, exec.Stderr))
		},
		Explain: []string{
			"No specialized profile matched.",
			"Raw stdout and stderr are combined with minimal trimming.",
		},
	}
}
