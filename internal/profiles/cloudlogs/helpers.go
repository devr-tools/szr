package cloudlogs

import "strings"

func isCloudLogsCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "aws":
		return len(args) >= 3 && args[1] == "logs" && (args[2] == "tail" || args[2] == "filter-log-events" || args[2] == "get-log-events")
	case "gcloud":
		return len(args) >= 3 && args[1] == "logging" && args[2] == "read"
	case "az":
		return len(args) >= 3 && args[1] == "monitor" && (args[2] == "activity-log" || args[2] == "log-analytics")
	default:
		return false
	}
}

func prepareCloudLogsCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	switch command[0] {
	case "gcloud":
		if hasAny(command[1:], "--format") || hasPrefix(command[1:], "--format=") {
			return command
		}
		return append(command, "--format=json")
	case "az":
		if hasAny(command[1:], "-o", "--output") || hasPrefix(command[1:], "--output=") {
			return command
		}
		return append(command, "-o", "json")
	default:
		return command
	}
}

func hasAny(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func hasPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
