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
	case "doctl":
		return doctlInventoryVerb(args)
	case "oci":
		return ociInventoryVerb(args)
	case "openstack":
		return openstackInventoryVerb(args)
	case "vercel":
		return vercelInventoryVerb(args)
	case "supabase":
		return supabaseInventoryVerb(args)
	case "heroku":
		return herokuInventoryVerb(args)
	default:
		return false
	}
}

func isVercelDeploymentCommand(args []string) bool {
	positional := positionalArgs(args[1:], vercelValueFlags)
	if len(positional) == 0 {
		return false
	}
	return positional[0] == "list" || positional[0] == "ls"
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

func doctlInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], doctlValueFlags)
	for _, arg := range positional {
		if arg == "list" || arg == "get" {
			return true
		}
	}
	return false
}

func ociInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], ociValueFlags)
	for _, arg := range positional {
		if arg == "list" || arg == "get" {
			return true
		}
	}
	return false
}

func openstackInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], openstackValueFlags)
	for _, arg := range positional {
		if arg == "list" || arg == "show" {
			return true
		}
	}
	return false
}

func vercelInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], vercelValueFlags)
	if len(positional) == 0 {
		return false
	}
	if positional[0] == "list" || positional[0] == "ls" {
		return true
	}
	for i := 0; i+1 < len(positional); i++ {
		if positional[i] == "projects" || positional[i] == "domains" || positional[i] == "teams" {
			if positional[i+1] == "list" || positional[i+1] == "ls" {
				return true
			}
		}
	}
	return false
}

func supabaseInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], supabaseValueFlags)
	for i := 0; i+1 < len(positional); i++ {
		switch positional[i] {
		case "projects", "functions", "branches":
			if positional[i+1] == "list" {
				return true
			}
		}
	}
	return false
}

func herokuInventoryVerb(args []string) bool {
	positional := positionalArgs(args[1:], herokuValueFlags)
	if len(positional) == 0 {
		return false
	}
	switch positional[0] {
	case "apps", "addons", "domains", "pipelines", "spaces":
		return true
	default:
		return false
	}
}

func prepareCloudListCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	prepare, ok := cloudListPreparers[command[0]]
	if !ok {
		return command
	}
	return prepare(command)
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

func prepareVercelDeploymentCommand(command []string) []string {
	out := prepareCloudListCommand(command)
	if containsAny(out[1:], "--meta") || containsPrefix(out[1:], "--meta=") {
		return out
	}
	return append(out, "--meta")
}

func containsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

var cloudListPreparers = map[string]func([]string) []string{
	"aws": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
	},
	"gcloud": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--format"}, []string{"--format="}, "--format=json")
	},
	"az": func(command []string) []string {
		return appendInventoryFlag(command, []string{"-o", "--output"}, []string{"--output="}, "-o", "json")
	},
	"doctl": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--format", "--output"}, []string{"--format=", "--output="}, "--output", "json")
	},
	"oci": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
	},
	"openstack": func(command []string) []string {
		return appendInventoryFlag(command, []string{"-f", "--format"}, []string{"--format="}, "-f", "json")
	},
	"vercel": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--json"}, []string{"--json="}, "--json")
	},
	"supabase": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
	},
	"heroku": func(command []string) []string {
		return appendInventoryFlag(command, []string{"--json"}, []string{"--json="}, "--json")
	},
}

func appendInventoryFlag(command []string, exactFlags, prefixFlags []string, additions ...string) []string {
	args := command[1:]
	if containsAny(args, exactFlags...) || containsAnyPrefix(args, prefixFlags...) {
		return command
	}
	return append(command, additions...)
}

func containsAnyPrefix(args []string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if containsPrefix(args, prefix) {
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

var doctlValueFlags = map[string]struct{}{
	"--context": {},
	"--token":   {},
	"--output":  {},
	"--format":  {},
}

var ociValueFlags = map[string]struct{}{
	"--profile":        {},
	"--region":         {},
	"--compartment-id": {},
	"--output":         {},
	"--query":          {},
}

var openstackValueFlags = map[string]struct{}{
	"--os-cloud": {},
	"--format":   {},
	"-f":         {},
	"--column":   {},
	"-c":         {},
	"--project":  {},
}

var vercelValueFlags = map[string]struct{}{
	"--scope": {},
	"--team":  {},
	"--token": {},
}

var supabaseValueFlags = map[string]struct{}{
	"--project-ref": {},
	"--linked":      {},
	"--output":      {},
}

var herokuValueFlags = map[string]struct{}{
	"--app":   {},
	"--team":  {},
	"--space": {},
	"--json":  {},
	"--org":   {},
}
