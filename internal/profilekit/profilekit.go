package profilekit

import (
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/engine"
)

func AtLeast(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}

func OutputBudget(lines int) engine.OutputBudget {
	if lines <= 0 {
		lines = 12
	}
	return engine.OutputBudget{
		MaxLines:  lines,
		MaxBytes:  lines * 160,
		MaxTokens: lines * 32,
	}
}

func LatencyBudget(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func ParseStdout(exec engine.Execution) int {
	return len(exec.Stdout)
}

func ParseCombined(exec engine.Execution) int {
	return len(exec.Stdout) + len(exec.Stderr)
}

func ParseStderrFirst(exec engine.Execution) int {
	if exec.Stderr == "" {
		return len(exec.Stdout)
	}
	return len(exec.Stderr) + len(exec.Stdout)
}

func HasCommand(args []string, head, sub string) bool {
	return len(args) >= 2 && args[0] == head && args[1] == sub
}

func ContainsAny(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func ContainsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
