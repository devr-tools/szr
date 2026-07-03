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

// RunGrep executes the builtin grep-to-ripgrep rewrite for argv shapes the
// builtin supports and reports an error otherwise.
//
// Deprecated: use TryRunGrep, which reports whether the builtin handled the
// command so the caller can delegate unsupported argv to the native binary.
func RunGrep(rt Runtime, cfg config.Config, args []string) int {
	code, handled := TryRunGrep(rt, cfg, args)
	if !handled {
		fmt.Fprintln(rt.Stderr, "szr: grep: unsupported arguments for builtin execution")
		return 2
	}
	return code
}

// TryRunGrep executes the builtin grep-to-ripgrep rewrite. The second return
// value reports whether the builtin handled the command; when it is false the
// caller must delegate the original argv to the native binary via the engine
// so raw semantics and exit codes are preserved.
func TryRunGrep(rt Runtime, cfg config.Config, args []string) (int, bool) {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: grep requires a pattern")
		return 2, true
	}
	if !grepBuiltinSupports(args) {
		return 0, false
	}

	output, exitCode, executed := runBuiltinRipgrep(args)
	if !executed {
		// rg vanished between LookPath and exec; fall back to grep.
		return 0, false
	}
	if exitCode > 1 {
		fmt.Fprintln(rt.Stderr, output)
		return exitCode, true
	}

	fmt.Fprintln(rt.Stdout, filters.GroupRipgrep(output, adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxMatchGroups)))
	return exitCode, true
}

// grepBuiltinSupports reports whether the builtin can handle the argv:
// the `PATTERN [PATH] [rg flags]` shape with ripgrep available on PATH.
// grep-style flag-first argv (e.g. `grep -rn PATTERN PATH`) and a missing
// ripgrep both mean the grep the user actually typed must run instead.
func grepBuiltinSupports(args []string) bool {
	if strings.HasPrefix(args[0], "-") {
		return false
	}
	_, err := exec.LookPath("rg")
	return err == nil
}

func runBuiltinRipgrep(args []string) (string, int, bool) {
	cmd := exec.Command("rg", buildBuiltinRipgrepArgs(args)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return "", 0, false
		}
		return string(output), exitErr.ExitCode(), true
	}
	return string(output), 0, true
}

func buildBuiltinRipgrepArgs(args []string) []string {
	pattern, searchPath, extra := splitGrepArgs(args)
	rgArgs := []string{"-n", "--no-heading"}
	if shouldInjectBuiltinRipgrepExcludes(searchPath, extra) {
		for _, glob := range filters.DefaultRipgrepExcludeGlobs() {
			rgArgs = append(rgArgs, "-g", glob)
		}
	}
	rgArgs = append(rgArgs, pattern, searchPath)
	return append(rgArgs, extra...)
}

// splitGrepArgs interprets the builtin shape `PATTERN [PATH] [rg flags]`.
// A flag in the path position is treated as the start of the rg flags.
func splitGrepArgs(args []string) (string, string, []string) {
	pattern := args[0]
	searchPath := "."
	extra := []string{}
	if len(args) > 1 {
		if strings.HasPrefix(args[1], "-") {
			extra = args[1:]
		} else {
			searchPath = args[1]
			extra = args[2:]
		}
	}
	return pattern, searchPath, extra
}

// RunFind executes the builtin find emulation for argv shapes the builtin
// supports and reports an error otherwise.
//
// Deprecated: use TryRunFind, which reports whether the builtin handled the
// command so the caller can delegate unsupported argv to the native binary.
func RunFind(rt Runtime, cfg config.Config, args []string) int {
	code, handled := TryRunFind(rt, cfg, args)
	if !handled {
		fmt.Fprintln(rt.Stderr, "szr: find: unsupported arguments for builtin execution")
		return 2
	}
	return code
}

// TryRunFind executes the builtin find emulation. The second return value
// reports whether the builtin handled the command; when it is false the
// caller must delegate the original argv to the native find binary.
func TryRunFind(rt Runtime, cfg config.Config, args []string) (int, bool) {
	opts, status := parseFindOptions(rt, args)
	if status == findDelegate {
		return 0, false
	}
	if status == findParseError {
		return 2, true
	}

	root := filepath.Clean(opts.root)
	limit := adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxPreviewLines)
	matches, err := collectFindMatches(root, opts)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1, true
	}

	if opts.grouped {
		fmt.Fprintln(rt.Stdout, filters.SummarizeFindPathsGrouped(matches, limit))
		return 0, true
	}
	fmt.Fprintln(rt.Stdout, filters.SummarizeFindPaths(matches, limit))
	return 0, true
}

func collectFindMatches(root string, opts findOptions) ([]string, error) {
	collector := &findCollector{root: root, rootDepth: pathDepth(root), opts: opts, matches: []string{}}
	err := filepathWalk(root, collector.visit)
	return collector.matches, err
}

type findCollector struct {
	root      string
	rootDepth int
	opts      findOptions
	matches   []string
}

func (c *findCollector) visit(current string, info os.FileInfo, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if current == c.root {
		return nil
	}
	normalized := filepath.ToSlash(current)
	relative := findRelativePath(current, c.root)
	decision, err := c.opts.match(current, normalized, relative, info, c.rootDepth)
	if err != nil || decision.skipDir || !decision.include {
		return findVisitOutcome(err, decision)
	}
	c.matches = append(c.matches, filepath.ToSlash(findDisplayPath(normalized, relative)))
	return nil
}

func findVisitOutcome(err error, decision findMatchDecision) error {
	if err != nil {
		return err
	}
	if decision.skipDir {
		return skipDir()
	}
	return nil
}

func findRelativePath(current, root string) string {
	relative := filepath.ToSlash(strings.TrimPrefix(current, root))
	return strings.TrimPrefix(relative, "/")
}

func findDisplayPath(normalized, relative string) string {
	if relative != "" {
		return relative
	}
	if base := filepath.Base(normalized); base != "" {
		return base
	}
	return normalized
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
	grouped     bool
}

type findMatchDecision struct {
	include bool
	skipDir bool
}

type findParseStatus int

const (
	findParsed findParseStatus = iota
	findParseError
	findDelegate
)

func parseFindOptions(rt Runtime, args []string) (findOptions, findParseStatus) {
	opts := findOptions{
		root:     ".",
		maxDepth: -1,
		excludes: defaultFindExcludes(),
	}
	rootSet := false

	for i := 0; i < len(args); i++ {
		if isSupportedFindFlag(args[i]) {
			if !applyFindFlag(rt, &opts, args, &i) {
				return findOptions{}, findParseError
			}
			continue
		}
		if !acceptFindRoot(&opts, args[i], &rootSet) {
			return findOptions{}, findDelegate
		}
	}

	return opts, findParsed
}

// acceptFindRoot records the positional root argument. Native find
// predicates (`-name`, `-type`, `-not`, ...) and additional roots are not
// implemented by the builtin, so it reports false and the caller delegates
// the whole command to the real find binary.
func acceptFindRoot(opts *findOptions, arg string, rootSet *bool) bool {
	if strings.HasPrefix(arg, "-") || *rootSet {
		return false
	}
	opts.root = arg
	*rootSet = true
	return true
}

func isSupportedFindFlag(arg string) bool {
	switch arg {
	case "--name", "--path", "--exclude", "--type", "--max-depth", "--grouped":
		return true
	default:
		return false
	}
}

func applyFindFlag(rt Runtime, opts *findOptions, args []string, index *int) bool {
	flag := args[*index]
	value, ok := findFlagValue(rt, args, index, flag)
	if !ok {
		return false
	}

	switch flag {
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
	case "--grouped":
		opts.grouped = true
	default:
		fmt.Fprintf(rt.Stderr, "szr: unexpected find argument %s\n", flag)
		return false
	}

	return true
}

func findFlagValue(rt Runtime, args []string, index *int, flag string) (string, bool) {
	if flag == "--grouped" {
		return "true", true
	}
	*index = *index + 1
	if *index >= len(args) {
		fmt.Fprintf(rt.Stderr, "szr: find requires a value for %s\n", flag)
		return "", false
	}
	return args[*index], true
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
