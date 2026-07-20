package engine

// executionBaselineCommand is the argv the user's command would have
// executed without any profile Prepare rewrite: the original wrapper argv
// for unwrapped shell invocations, the prepared command otherwise.
func executionBaselineCommand(inv Invocation) []string {
	if inv.ShellWrap != nil {
		return inv.ShellWrap.Original
	}
	return inv.Command
}

func commandWasRewritten(original []string, prepared []string) bool {
	if len(original) != len(prepared) {
		return true
	}
	for i := range original {
		if original[i] != prepared[i] {
			return true
		}
	}
	return false
}
