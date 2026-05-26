package cloudlogs

import "strings"

func isCloudLogsCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	match, ok := cloudLogsMatchers[args[0]]
	return ok && match(args)
}

func prepareCloudLogsCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	prepare, ok := cloudLogsPreparers[command[0]]
	if !ok {
		return command
	}
	return prepare(command)
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

func hasFlagValue(args []string, flag, want string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == flag && i+1 < len(args) {
			if want == "" || args[i+1] == want {
				return true
			}
		}
		if strings.HasPrefix(arg, flag+"=") {
			value := strings.TrimPrefix(arg, flag+"=")
			if want == "" || value == want {
				return true
			}
		}
	}
	return false
}

var cloudLogsMatchers = map[string]func([]string) bool{
	"aws": func(args []string) bool {
		return matchTokens(args, "logs", "tail") ||
			matchTokens(args, "logs", "filter-log-events") ||
			matchTokens(args, "logs", "get-log-events")
	},
	"gcloud": func(args []string) bool {
		return matchTokens(args, "logging", "read")
	},
	"az": func(args []string) bool {
		return matchTokens(args, "monitor", "activity-log") ||
			matchTokens(args, "monitor", "log-analytics")
	},
	"oci": func(args []string) bool {
		return matchTokens(args, "logging-search", "search-logs")
	},
	"doctl": func(args []string) bool {
		return matchTokens(args, "apps", "logs")
	},
	"openstack": func(args []string) bool {
		return matchTokens(args, "console", "log", "show")
	},
	"vercel": func(args []string) bool {
		return matchTokens(args, "logs")
	},
	"supabase": func(args []string) bool {
		return matchTokens(args, "logs") || matchTokens(args, "functions", "logs")
	},
	"heroku": func(args []string) bool {
		return matchTokens(args, "logs") &&
			!hasFlagValue(args[2:], "--source", "app") &&
			!hasFlagValue(args[2:], "--dyno", "")
	},
}

var cloudLogsPreparers = map[string]func([]string) []string{
	"gcloud": func(command []string) []string {
		return appendOutputFlag(command, []string{"--format"}, []string{"--format="}, "--format=json")
	},
	"az": func(command []string) []string {
		return appendOutputFlag(command, []string{"-o", "--output"}, []string{"--output="}, "-o", "json")
	},
	"oci": func(command []string) []string {
		return appendOutputFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
	},
	"supabase": func(command []string) []string {
		return appendOutputFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
	},
	"heroku": func(command []string) []string {
		if hasFlagValue(command[2:], "--source", "heroku") || hasAny(command[2:], "--source") {
			return command
		}
		return append(command, "--source", "heroku")
	},
}

func matchTokens(args []string, tokens ...string) bool {
	if len(args) < len(tokens)+1 {
		return false
	}
	for idx, token := range tokens {
		if args[idx+1] != token {
			return false
		}
	}
	return true
}

func appendOutputFlag(command []string, exactFlags, prefixFlags []string, additions ...string) []string {
	args := command[1:]
	if hasAny(args, exactFlags...) || hasAnyPrefix(args, prefixFlags...) {
		return command
	}
	return append(command, additions...)
}

func hasAnyPrefix(args []string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if hasPrefix(args, prefix) {
			return true
		}
	}
	return false
}

func isSupabaseFunctionLogsCommand(args []string) bool {
	return len(args) >= 3 && args[0] == "supabase" && args[1] == "functions" && args[2] == "logs"
}

func prepareSupabaseFunctionLogsCommand(command []string) []string {
	return appendOutputFlag(command, []string{"--output"}, []string{"--output="}, "--output", "json")
}

func isHerokuRouterLogsCommand(args []string) bool {
	return len(args) >= 2 && args[0] == "heroku" && args[1] == "logs" &&
		!hasFlagValue(args[2:], "--source", "app") &&
		!hasFlagValue(args[2:], "--dyno", "")
}

func prepareHerokuRouterLogsCommand(command []string) []string {
	if hasFlagValue(command[2:], "--source", "heroku") || hasAny(command[2:], "--source") {
		return command
	}
	return append(command, "--source", "heroku")
}
