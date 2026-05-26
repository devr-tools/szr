package localcmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/filters"
)

func RunGrep(rt Runtime, cfg config.Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: grep requires a pattern")
		return 2
	}

	pattern := args[0]
	searchPath := "."
	extra := []string{}
	if len(args) > 1 {
		searchPath = args[1]
		if len(args) > 2 {
			extra = args[2:]
		}
	}

	rgArgs := []string{"-n", "--no-heading"}
	if shouldInjectBuiltinRipgrepExcludes(searchPath, extra) {
		for _, glob := range filters.DefaultRipgrepExcludeGlobs() {
			rgArgs = append(rgArgs, "-g", glob)
		}
	}
	rgArgs = append(rgArgs, pattern, searchPath)
	rgArgs = append(rgArgs, extra...)
	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
			return 1
		}
		if exitErr.ExitCode() > 1 {
			fmt.Fprintln(rt.Stderr, string(output))
			return exitErr.ExitCode()
		}
	}

	fmt.Fprintln(rt.Stdout, filters.GroupRipgrep(string(output), adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxMatchGroups)))
	return 0
}

func RunFind(rt Runtime, cfg config.Config, args []string) int {
	opts, exitCode := parseFindOptions(rt, args)
	if exitCode != 0 {
		return exitCode
	}

	root := filepath.Clean(opts.root)
	limit := adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxPreviewLines)
	matches := []string{}
	rootDepth := pathDepth(root)
	err := filepathWalk(root, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		normalized := filepath.ToSlash(current)
		relative := filepath.ToSlash(strings.TrimPrefix(current, root))
		relative = strings.TrimPrefix(relative, "/")
		decision, err := opts.match(current, normalized, relative, info, rootDepth)
		if err != nil {
			return err
		}
		if decision.skipDir {
			return skipDir()
		}
		if !decision.include {
			return nil
		}
		matches = append(matches, normalized)
		return nil
	})
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	fmt.Fprintln(rt.Stdout, filters.SummarizeFindPaths(matches, limit))
	return 0
}

func RunRGExternal(rt Runtime, runExternal func([]string) int, args []string) int {
	if _, err := exec.LookPath("rg"); err != nil {
		fmt.Fprintln(rt.Stderr, "szr: `rg` is not installed or not on PATH")
		fmt.Fprintln(rt.Stderr, "szr: install ripgrep to use `szr rg ...`")
		return 1
	}
	return runExternal(args)
}

type findOptions struct {
	root        string
	namePattern string
	pathPattern string
	matchType   string
	maxDepth    int
	excludes    []string
}

type findMatchDecision struct {
	include bool
	skipDir bool
}

func parseFindOptions(rt Runtime, args []string) (findOptions, int) {
	opts := findOptions{
		root:     ".",
		maxDepth: -1,
		excludes: defaultFindExcludes(),
	}
	rootSet := false

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			if !applyFindFlag(rt, &opts, args, &i) {
				return findOptions{}, 2
			}
			continue
		}
		if !assignFindRoot(rt, &opts, args[i], &rootSet) {
			return findOptions{}, 2
		}
	}

	return opts, 0
}

func applyFindFlag(rt Runtime, opts *findOptions, args []string, index *int) bool {
	value, ok := findFlagValue(rt, args, index, args[*index])
	if !ok {
		return false
	}

	switch args[*index-1] {
	case "--name":
		opts.namePattern = value
	case "--path":
		opts.pathPattern = filepath.ToSlash(value)
	case "--exclude":
		opts.excludes = append(opts.excludes, filepath.ToSlash(value))
	case "--type":
		if value != "f" && value != "d" {
			fmt.Fprintf(rt.Stderr, "szr: unsupported find type %q\n", value)
			return false
		}
		opts.matchType = value
	case "--max-depth":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			fmt.Fprintf(rt.Stderr, "szr: invalid find max-depth %q\n", value)
			return false
		}
		opts.maxDepth = parsed
	default:
		fmt.Fprintf(rt.Stderr, "szr: unexpected find argument %s\n", args[*index-1])
		return false
	}

	return true
}

func findFlagValue(rt Runtime, args []string, index *int, flag string) (string, bool) {
	*index = *index + 1
	if *index >= len(args) {
		fmt.Fprintf(rt.Stderr, "szr: find requires a value for %s\n", flag)
		return "", false
	}
	return args[*index], true
}

func assignFindRoot(rt Runtime, opts *findOptions, value string, rootSet *bool) bool {
	if *rootSet {
		fmt.Fprintf(rt.Stderr, "szr: unexpected find argument %s\n", value)
		return false
	}
	opts.root = value
	*rootSet = true
	return true
}

func (o findOptions) match(current, normalized, relative string, info os.FileInfo, rootDepth int) (findMatchDecision, error) {
	if shouldExcludeFindPath(normalized, relative, o.excludes) {
		return findSkipDecision(info), nil
	}
	if o.maxDepth >= 0 && pathDepth(current)-rootDepth > o.maxDepth {
		return findSkipDecision(info), nil
	}
	if o.matchType == "f" && info.IsDir() {
		return findMatchDecision{}, nil
	}
	if o.matchType == "d" && !info.IsDir() {
		return findMatchDecision{}, nil
	}
	if o.namePattern != "" {
		ok, err := path.Match(o.namePattern, info.Name())
		if err != nil || !ok {
			return findMatchDecision{include: ok}, err
		}
	}
	if o.pathPattern != "" {
		ok, err := matchesFindPath(o.pathPattern, normalized, relative)
		if err != nil || !ok {
			return findMatchDecision{include: ok}, err
		}
	}
	return findMatchDecision{include: true}, nil
}

func findSkipDecision(info os.FileInfo) findMatchDecision {
	return findMatchDecision{
		include: false,
		skipDir: info.IsDir(),
	}
}

func matchesFindPath(pattern, normalized, relative string) (bool, error) {
	ok, err := path.Match(pattern, normalized)
	if err != nil || ok {
		return ok, err
	}
	if relative == "" {
		return false, nil
	}
	return path.Match(pattern, relative)
}

func adjustCountForReasoningMode(mode string, value int) int {
	if value <= 0 {
		return value
	}
	switch mode {
	case config.ReasoningBudgetAgent:
		scaled := (value*3 + 3) / 4
		if scaled < 4 {
			return 4
		}
		return scaled
	case config.ReasoningBudgetAggressive:
		scaled := (value + 1) / 2
		if scaled < 3 {
			return 3
		}
		return scaled
	default:
		return value
	}
}

func shouldExcludeFindPath(fullPath, relativePath string, excludes []string) bool {
	for _, pattern := range excludes {
		if ok, _ := path.Match(pattern, fullPath); ok {
			return true
		}
		if relativePath != "" {
			if ok, _ := path.Match(pattern, relativePath); ok {
				return true
			}
		}
	}
	return false
}

func defaultFindExcludes() []string {
	patterns := make([]string, 0, len(filters.DefaultSearchNoiseDirs()))
	for _, dir := range filters.DefaultSearchNoiseDirs() {
		patterns = append(patterns, "*/"+dir+"/*")
	}
	return patterns
}

func shouldInjectBuiltinRipgrepExcludes(searchPath string, extra []string) bool {
	if searchPath != "." && searchPath != "./" && searchPath != "" {
		return false
	}
	for _, arg := range extra {
		if arg == "--hidden" || arg == "--no-ignore" || arg == "--no-ignore-vcs" || arg == "--no-ignore-parent" || arg == "-u" || arg == "-uu" || arg == "-uuu" {
			return false
		}
		if strings.HasPrefix(arg, "-g") || strings.HasPrefix(arg, "--glob") || strings.HasPrefix(arg, "--iglob") {
			return false
		}
	}
	return true
}
