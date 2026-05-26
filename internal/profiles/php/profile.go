package php

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	phpfilter "github.com/devr-tools/szr/internal/filters/php"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "php-tooling",
			Description:      "Summarizes Composer, PHPUnit/Pest, and common PHP static-analysis output around actionable failures.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isPHPToolingCommand(inv.Display) || isPHPToolingCommand(inv.Command)
			},
			Prepare: func(inv engine.Invocation) []string {
				if !inv.Advanced.AggressivePrepareRewrites {
					return inv.Command
				}
				return preparePHPToolingCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return phpfilter.SummarizePHP(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return phpfilter.SummarizePHP(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches Composer, PHPUnit, Pest, PHPStan, Psalm, PHPCS, PHP CS Fixer, and `php artisan` workflows.",
				"Keeps failing assertions, dependency-resolution errors, static-analysis diagnostics, and PHP file anchors while trimming install and runner chatter.",
			},
		},
	}
}

func isPHPToolingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch commandBase(args[0]) {
	case "composer", "phpunit", "pest", "phpstan", "psalm", "phpcs", "phpcbf", "php-cs-fixer", "artisan":
		return true
	case "php":
		if len(args) < 2 {
			return false
		}
		switch commandBase(args[1]) {
		case "artisan", "phpunit", "pest", "phpstan", "psalm", "phpcs", "phpcbf", "php-cs-fixer":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func commandBase(arg string) string {
	if arg == "" {
		return ""
	}
	return filepath.Base(arg)
}

func preparePHPToolingCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	switch detectPHPTool(command) {
	case "composer":
		out = prepareComposerCommand(out, command[1:])
	case "phpunit", "pest":
		out = preparePHPTestCommand(out, command[1:])
	case "artisan":
		out = prepareArtisanCommand(out, command)
	case "phpstan":
		out = appendPHPArgIfMissing(out, command[1:], "--no-progress")
	case "psalm":
		out = preparePsalmCommand(out, command[1:])
	case "phpcs":
		out = preparePHPCSCommand(out, command[1:])
	}
	return out
}

func prepareComposerCommand(out, args []string) []string {
	out = appendPHPArgIfMissing(out, args, "--no-progress")
	return appendPHPArgIfMissing(out, args, "--no-ansi")
}

func preparePHPTestCommand(out, args []string) []string {
	if !containsPHPArg(args, "--colors=never", "--colors", "never") && !containsPHPPrefix(args, "--colors=") {
		out = append(out, "--colors=never")
	}
	return appendPHPArgIfMissing(out, args, "--no-progress")
}

func prepareArtisanCommand(out, command []string) []string {
	if !isPHPArtisanTestCommand(command) {
		return out
	}
	args := phpArtisanArgs(command)
	out = appendPHPArgIfMissing(out, args, "--without-tty")
	return appendPHPArgIfMissing(out, args, "--compact")
}

func phpArtisanArgs(command []string) []string {
	if commandBase(command[0]) == "php" && len(command) > 2 {
		return command[2:]
	}
	return command[1:]
}

func preparePsalmCommand(out, args []string) []string {
	if !containsPHPPrefix(args, "--output-format=") && !containsPHPArg(args, "--output-format") {
		out = append(out, "--output-format=console")
	}
	return out
}

func preparePHPCSCommand(out, args []string) []string {
	if !containsPHPArg(args, "-q", "--quiet") {
		out = append(out, "-q")
	}
	return out
}

func appendPHPArgIfMissing(out, args []string, needle string) []string {
	if !containsPHPArg(args, needle) {
		out = append(out, needle)
	}
	return out
}

func detectPHPTool(command []string) string {
	if len(command) == 0 {
		return ""
	}
	if commandBase(command[0]) != "php" {
		return commandBase(command[0])
	}
	for i := 1; i < len(command); i++ {
		if strings.HasPrefix(command[i], "-") {
			continue
		}
		return commandBase(command[i])
	}
	return "php"
}

func containsPHPArg(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func containsPHPPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func isPHPArtisanTestCommand(command []string) bool {
	if len(command) >= 2 && commandBase(command[0]) == "artisan" && command[1] == "test" {
		return true
	}
	return len(command) >= 3 && commandBase(command[0]) == "php" && commandBase(command[1]) == "artisan" && command[2] == "test"
}
