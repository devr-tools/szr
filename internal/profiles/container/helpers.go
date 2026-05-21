package container

import "strings"

func isDockerComposeCommand(args []string, sub string) bool {
	return len(args) >= 3 && args[0] == "docker" && args[1] == "compose" && args[2] == sub
}

func insertAfterDockerSubcommand(command []string, extra ...string) []string {
	if len(command) == 0 || len(extra) == 0 {
		return command
	}

	insertAt := len(command)
	start := 2
	if isDockerComposeCommand(command, "logs") || isDockerComposeCommand(command, "ps") {
		start = 3
	}
	for i := start; i < len(command); i++ {
		arg := command[i]
		if !strings.HasPrefix(arg, "-") {
			insertAt = i
			break
		}
	}
	out := append([]string{}, command[:insertAt]...)
	out = append(out, extra...)
	return append(out, command[insertAt:]...)
}
