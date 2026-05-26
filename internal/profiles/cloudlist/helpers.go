package cloudlist

import "strings"

func isCloudListCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "aws":
		return awsInventoryVerb(args)
	case "gcloud":
		return gcloudInventoryVerb(args)
	case "az":
		return azInventoryVerb(args)
	default:
		return false
	}
}

func awsInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], awsValueFlags)
	if len(positional) < 2 {
		return false
	}
	verb := positional[1]
	return strings.HasPrefix(verb, "list-") || strings.HasPrefix(verb, "describe-") || strings.HasPrefix(verb, "get-")
}

func gcloudInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], gcloudValueFlags)
	for idx, arg := range positional {
		if idx == 0 {
			continue
		}
		if arg == "list" || arg == "describe" || arg == "get" || strings.HasPrefix(arg, "get-") {
			return true
		}
	}
	return false
}

func azInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], azValueFlags)
	for idx, arg := range positional {
		if idx == 0 {
			continue
		}
		if arg == "list" || arg == "show" || arg == "get" || strings.HasPrefix(arg, "list-") || strings.HasPrefix(arg, "show-") || strings.HasPrefix(arg, "get-") {
			return true
		}
	}
	return false
}

func prepareCloudListCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	switch command[0] {
	case "aws":
		if containsAny(command[1:], "--output") || containsPrefix(command[1:], "--output=") {
			return command
		}
		return append(command, "--output", "json")
	case "gcloud":
		if containsAny(command[1:], "--format") || containsPrefix(command[1:], "--format=") {
			return command
		}
		return append(command, "--format=json")
	case "az":
		if containsAny(command[1:], "-o", "--output") || containsPrefix(command[1:], "--output=") {
			return command
		}
		return append(command, "-o", "json")
	default:
		return command
	}
}

func positionalArgs(args []string, valueFlags map[string]struct{}) []string {
	out := []string{}
	for i := 0; i < len(args); {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			i = skipOption(args, i, valueFlags)
			continue
		}
		out = append(out, arg)
		i++
	}
	return out
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

var awsValueFlags = map[string]struct{}{
	"--profile":        {},
	"--region":         {},
	"--output":         {},
	"--query":          {},
	"--endpoint-url":   {},
	"--cli-input-json": {},
	"--cli-input-yaml": {},
}

var gcloudValueFlags = map[string]struct{}{
	"--project":                     {},
	"--configuration":               {},
	"--account":                     {},
	"--impersonate-service-account": {},
	"--filter":                      {},
	"--format":                      {},
	"--limit":                       {},
	"--page-size":                   {},
	"--sort-by":                     {},
	"--uri":                         {},
}

var azValueFlags = map[string]struct{}{
	"-g":               {},
	"--resource-group": {},
	"-n":               {},
	"--name":           {},
	"--subscription":   {},
	"--query":          {},
	"-o":               {},
	"--output":         {},
}
