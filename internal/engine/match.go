package engine

func (e *Engine) match(inv Invocation) Profile {
	for _, profile := range e.profiles {
		if profile.Match != nil && profile.Match(inv) {
			return profile
		}
	}
	return Profile{
		Name:        "passthrough",
		Description: "Raw command passthrough with trimming.",
		Render: func(_ Invocation, exec Execution) string {
			return combineStreams(exec.Stdout, exec.Stderr)
		},
		Explain: []string{
			"No specialized profile matched.",
			"Raw stdout and stderr are combined with minimal trimming.",
		},
	}
}
