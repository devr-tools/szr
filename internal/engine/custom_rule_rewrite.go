package engine

import "szr/internal/rules"

func rewriteRule(rewrite rules.Rewrite, inv Invocation) []string {
	mode := rewrite.Mode
	if mode == "" {
		mode = "append"
	}
	if len(rewrite.Args) == 0 {
		return inv.Command
	}
	if containsAnyValue(invocationArgs(inv), rewrite.SkipIfHasAny) {
		return inv.Command
	}

	switch mode {
	case "replace":
		return append([]string(nil), rewrite.Args...)
	default:
		return applyRewriteArgs(inv.Command, rewrite.Args, rewrite.Placement)
	}
}

func applyRewriteArgs(command []string, args []string, placement string) []string {
	command = append([]string(nil), command...)
	switch placement {
	case "before-terminator":
		if index := indexOf(command, "--"); index >= 0 {
			rewritten := append([]string(nil), command[:index]...)
			rewritten = append(rewritten, args...)
			rewritten = append(rewritten, command[index:]...)
			return rewritten
		}
	}
	command = append(command, args...)
	return command
}

func indexOf(values []string, needle string) int {
	for i, value := range values {
		if value == needle {
			return i
		}
	}
	return -1
}
