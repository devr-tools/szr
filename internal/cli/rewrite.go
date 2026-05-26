package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devr-tools/szr/internal/rewrite"
)

func (a *App) runRewrite(args []string) int {
	opts, exitCode := parseRewriteOptions(args)
	if exitCode != 0 {
		return exitCode
	}

	if opts.hook != "" {
		return a.runRewriteHook(opts.hook, opts.binary)
	}
	command := opts.commandString()
	if opts.readStdin && command == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to read rewrite stdin: %v\n", err)
			return 1
		}
		command = strings.TrimSpace(string(data))
	}
	if strings.TrimSpace(command) == "" {
		fmt.Fprintln(os.Stderr, "szr: rewrite requires a command")
		return 2
	}

	decision := rewrite.Analyze(command, opts.binary)
	switch opts.format {
	case "command":
		if decision.Rewrite != "" {
			fmt.Println(decision.Rewrite)
		}
	case "hint":
		if decision.Hint != "" {
			fmt.Println(decision.Hint)
		}
	case "json":
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(decision)
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown rewrite format %s\n", opts.format)
		return 2
	}
	return 0
}

type rewriteOptions struct {
	format       string
	hook         string
	binary       string
	readStdin    bool
	commandParts []string
}

func parseRewriteOptions(args []string) (rewriteOptions, int) {
	opts := rewriteOptions{
		format:       "command",
		binary:       "szr",
		commandParts: []string{},
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			value, ok := rewriteFlagValue(args, &i, "--format")
			if !ok {
				return rewriteOptions{}, 2
			}
			opts.format = value
		case "--hook":
			value, ok := rewriteFlagValue(args, &i, "--hook")
			if !ok {
				return rewriteOptions{}, 2
			}
			opts.hook = value
		case "--command":
			value, ok := rewriteFlagValue(args, &i, "--command")
			if !ok {
				return rewriteOptions{}, 2
			}
			opts.commandParts = []string{value}
		case "--binary":
			value, ok := rewriteFlagValue(args, &i, "--binary")
			if !ok {
				return rewriteOptions{}, 2
			}
			opts.binary = value
		case "--stdin":
			opts.readStdin = true
		default:
			opts.commandParts = append(opts.commandParts, args[i])
		}
	}

	return opts, 0
}

func rewriteFlagValue(args []string, index *int, flag string) (string, bool) {
	*index++
	if *index >= len(args) {
		fmt.Fprintf(os.Stderr, "szr: rewrite requires a value for %s\n", flag)
		return "", false
	}
	return args[*index], true
}

func (o rewriteOptions) commandString() string {
	return strings.Join(o.commandParts, " ")
}

func (a *App) runRewriteHook(hook, binary string) int {
	var payload struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return printRewriteHookFallback(hook)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return printRewriteHookFallback(hook)
	}
	command, _ := payload.ToolInput["command"].(string)
	decision := rewrite.Analyze(command, binary)
	if !decision.AutoRewrite || decision.Rewrite == "" {
		return printRewriteHookFallback(hook)
	}
	switch hook {
	case "claude":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "allow",
				"permissionDecisionReason": "szr auto-rewrite",
				"updatedInput": map[string]any{
					"command": decision.Rewrite,
				},
			},
		})
	case "cursor":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"permission": "allow",
			"updated_input": map[string]any{
				"command": decision.Rewrite,
			},
		})
	case "gemini":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"decision": "allow",
			"hookSpecificOutput": map[string]any{
				"tool_input": map[string]any{
					"command": decision.Rewrite,
				},
			},
		})
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown rewrite hook %s\n", hook)
		return 2
	}
	return 0
}

func printRewriteHookFallback(hook string) int {
	switch hook {
	case "claude":
		return 0
	case "cursor":
		fmt.Print("{}")
		return 0
	case "gemini":
		fmt.Print("{\"decision\":\"allow\"}")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown rewrite hook %s\n", hook)
		return 2
	}
}
