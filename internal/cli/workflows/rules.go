package workflows

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/internal/rules"
)

func RunRules(rt Runtime, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: rules requires a subcommand")
		return 2
	}
	switch args[0] {
	case "check":
		return RunRulesCheck(rt, args[1:])
	case "test":
		return RunRulesTest(rt, args[1:])
	default:
		fmt.Fprintf(rt.Stderr, "szr: unknown rules subcommand %s\n", args[0])
		return 2
	}
}

func RunRulesCheck(rt Runtime, args []string) int {
	asJSON := false
	path := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(rt.Stderr, "szr: unknown rules check flag %s\n", arg)
				return 2
			}
			if path != "" {
				fmt.Fprintln(rt.Stderr, "szr: rules check accepts at most one path")
				return 2
			}
			path = arg
		}
	}

	resolved, file, err := loadRulesFile(path)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	if asJSON {
		payload := map[string]any{
			"path":        resolved,
			"profiles":    len(file.Profiles),
			"preferences": len(file.Preferences),
			"version":     file.Version,
		}
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

	fmt.Fprintf(rt.Stdout, "rules: %s\n", resolved)
	fmt.Fprintf(rt.Stdout, "version: %d\n", file.Version)
	fmt.Fprintf(rt.Stdout, "profiles: %d\n", len(file.Profiles))
	fmt.Fprintf(rt.Stdout, "preferences: %d\n", len(file.Preferences))
	fmt.Fprintln(rt.Stdout, "status: valid")
	return 0
}

func RunRulesTest(rt Runtime, args []string) int {
	opts, err := parseRulesTestArgs(args)
	if err != nil {
		fmt.Fprintln(rt.Stderr, "szr:", err)
		return 2
	}
	if len(opts.command) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: rules test requires a command")
		return 2
	}

	resolved, file, err := loadRulesFile(opts.path)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	cfg := rt.Config
	cfg.ProjectRules = file
	paths := rt.Paths
	paths.ProjectRuleFile = resolved
	paths.ProjectDir = filepath.Dir(resolved)
	eng := engine.New(cfg, paths, rt.History, profiles.Builtins(cfg.MaxPreviewLines))

	cwd, _ := os.Getwd()
	inv := engine.Invocation{
		Command:             append([]string(nil), opts.command...),
		Display:             append([]string(nil), opts.command...),
		Cwd:                 cwd,
		Verbose:             rt.Verbose,
		UltraCompact:        rt.UltraCompact,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
	}
	effectiveInv, preferences := eng.ExplainPreferences(inv)
	profile := eng.Explain(inv)
	decisions := eng.ExplainDecisions(inv)
	describeSource := rt.DescribeProfileSource
	if describeSource == nil {
		describeSource = func(source string, _ string) string { return source }
	}

	if opts.asJSON {
		payload := map[string]any{
			"rules":             resolved,
			"command":           strings.Join(opts.command, " "),
			"effective_command": strings.Join(effectiveInv.Command, " "),
			"profile":           profile.Name,
			"source":            describeSource(profile.Source, resolved),
			"preferences":       preferences,
			"decisions":         decisions,
		}
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

	fmt.Fprintf(rt.Stdout, "rules: %s\n", resolved)
	fmt.Fprintf(rt.Stdout, "command: %s\n", strings.Join(opts.command, " "))
	if len(effectiveInv.Command) > 0 {
		fmt.Fprintf(rt.Stdout, "effective command: %s\n", strings.Join(effectiveInv.Command, " "))
	}
	fmt.Fprintf(rt.Stdout, "selected profile: %s\n", profile.Name)
	fmt.Fprintf(rt.Stdout, "source: %s\n", describeSource(profile.Source, resolved))
	if len(preferences) > 0 {
		fmt.Fprintln(rt.Stdout, "preferences:")
		for _, preference := range preferences {
			label := "satisfied"
			if preference.Applied {
				label = "applied"
			}
			fmt.Fprintf(rt.Stdout, "  - %s %s\n", label, preference.Name)
		}
	}
	if len(decisions) > 0 {
		fmt.Fprintln(rt.Stdout, "matches:")
		for _, decision := range decisions {
			label := "also matches"
			if decision.Selected {
				label = "selected"
			}
			fmt.Fprintf(rt.Stdout, "  - %s %s (%s)\n", label, decision.Name, describeSource(decision.Source, resolved))
		}
	}
	return 0
}

type rulesTestOptions struct {
	asJSON  bool
	path    string
	command []string
}

func parseRulesTestArgs(args []string) (rulesTestOptions, error) {
	var opts rulesTestOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.asJSON = true
		case "--file":
			if i+1 >= len(args) {
				return rulesTestOptions{}, fmt.Errorf("rules test requires a value after --file")
			}
			i++
			opts.path = args[i]
		default:
			opts.command = append(opts.command, args[i])
		}
	}
	return opts, nil
}

func loadRulesFile(path string) (string, rules.File, error) {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", rules.File{}, err
		}
		discovered, _, err := rules.Discover(cwd)
		if err != nil {
			return "", rules.File{}, err
		}
		if discovered == "" {
			return "", rules.File{}, fmt.Errorf("no .szr.json/.szr.yaml/.szr.yml file found from %s upward", cwd)
		}
		resolved = discovered
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", rules.File{}, err
	}
	file, err := rules.ParseFile(resolved, data)
	if err != nil {
		return "", rules.File{}, err
	}
	return resolved, file, nil
}
