package kubernetes

import "strings"

func kubectlVerb(args []string) (int, string, bool) {
	if len(args) == 0 || args[0] != "kubectl" {
		return -1, "", false
	}
	for i := 1; i < len(args); {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			return i, arg, true
		}
		i = skipOption(args, i, kubectlOptionValueFlags)
	}
	return -1, "", false
}

func isKubectlCommand(args []string, verb string) bool {
	_, got, ok := kubectlVerb(args)
	return ok && got == verb
}

func hasKubectlWideOutput(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" || arg == "--output" {
			if i+1 < len(args) && args[i+1] == "wide" {
				return true
			}
			continue
		}
		if arg == "-owide" || strings.HasPrefix(arg, "-o=") && strings.TrimPrefix(arg, "-o=") == "wide" {
			return true
		}
		if strings.HasPrefix(arg, "--output=") && strings.TrimPrefix(arg, "--output=") == "wide" {
			return true
		}
	}
	return false
}

func insertKubectlVerbArgs(command []string, extra ...string) []string {
	if len(command) == 0 || len(extra) == 0 {
		return command
	}
	verbIdx, verb, ok := kubectlVerb(command)
	if !ok {
		return command
	}

	insertAt := len(command)
	valueFlags := kubectlSubcommandValueFlags[verb]
	for i := verbIdx + 1; i < len(command); {
		arg := command[i]
		if !strings.HasPrefix(arg, "-") {
			insertAt = i
			break
		}
		i = skipOption(command, i, valueFlags)
	}
	out := append([]string{}, command[:insertAt]...)
	out = append(out, extra...)
	return append(out, command[insertAt:]...)
}

func containsAny(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func containsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func skipOption(args []string, i int, valueFlags map[string]struct{}) int {
	if i >= len(args) {
		return i + 1
	}
	arg := args[i]
	if strings.Contains(arg, "=") {
		return i + 1
	}
	if _, ok := valueFlags[arg]; ok && i+1 < len(args) {
		return i + 2
	}
	return i + 1
}

var kubectlOptionValueFlags = map[string]struct{}{
	"-n":                {},
	"--namespace":       {},
	"--context":         {},
	"--cluster":         {},
	"--user":            {},
	"--kubeconfig":      {},
	"--request-timeout": {},
}

var kubectlSubcommandValueFlags = map[string]map[string]struct{}{
	"get": {
		"-n":               {},
		"--namespace":      {},
		"-o":               {},
		"--output":         {},
		"-l":               {},
		"--selector":       {},
		"--field-selector": {},
	},
	"describe": {
		"-n":               {},
		"--namespace":      {},
		"-l":               {},
		"--selector":       {},
		"--field-selector": {},
	},
	"logs": {
		"-n":           {},
		"--namespace":  {},
		"-c":           {},
		"--container":  {},
		"--since":      {},
		"--since-time": {},
		"--tail":       {},
		"-l":           {},
		"--selector":   {},
	},
}
