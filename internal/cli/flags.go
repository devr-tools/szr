package cli

import "strings"

type globalFlags struct {
	verbose int
	ultra   bool
}

func parseGlobalFlags(args []string) (globalFlags, []string) {
	var flags globalFlags
	for len(args) > 0 {
		switch args[0] {
		case "-u", "--ultra-compact":
			flags.ultra = true
			args = args[1:]
		case "-v", "--verbose":
			flags.verbose++
			args = args[1:]
		case "-vv":
			flags.verbose += 2
			args = args[1:]
		case "-vvv":
			flags.verbose += 3
			args = args[1:]
		default:
			if strings.HasPrefix(args[0], "-") && strings.Trim(args[0], "v") == "-" {
				flags.verbose += strings.Count(args[0], "v")
				args = args[1:]
				continue
			}
			return flags, args
		}
	}
	return flags, args
}
