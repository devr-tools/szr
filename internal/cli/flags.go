package cli

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/config"
)

type globalFlags struct {
	verbose            int
	ultra              bool
	reasoningBudget    string
	reasoningBudgetSet bool
}

func parseGlobalFlags(args []string) (globalFlags, []string, error) {
	var flags globalFlags
	for len(args) > 0 {
		switch args[0] {
		case "-u", "--ultra-compact":
			flags.ultra = true
			args = args[1:]
		case "--reasoning-budget", "--reasoning-budget-mode":
			if len(args) < 2 {
				return globalFlags{}, nil, fmt.Errorf("missing value for %s", args[0])
			}
			mode, err := config.NormalizeReasoningBudgetMode(args[1])
			if err != nil {
				return globalFlags{}, nil, err
			}
			flags.reasoningBudget = mode
			flags.reasoningBudgetSet = true
			args = args[2:]
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
			if value, ok := strings.CutPrefix(args[0], "--reasoning-budget="); ok {
				mode, err := config.NormalizeReasoningBudgetMode(value)
				if err != nil {
					return globalFlags{}, nil, err
				}
				flags.reasoningBudget = mode
				flags.reasoningBudgetSet = true
				args = args[1:]
				continue
			}
			if value, ok := strings.CutPrefix(args[0], "--reasoning-budget-mode="); ok {
				mode, err := config.NormalizeReasoningBudgetMode(value)
				if err != nil {
					return globalFlags{}, nil, err
				}
				flags.reasoningBudget = mode
				flags.reasoningBudgetSet = true
				args = args[1:]
				continue
			}
			if strings.HasPrefix(args[0], "-") && strings.Trim(args[0], "v") == "-" {
				flags.verbose += strings.Count(args[0], "v")
				args = args[1:]
				continue
			}
			return flags, args, nil
		}
	}
	return flags, args, nil
}
