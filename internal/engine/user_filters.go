package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/declarative"
	"github.com/devr-tools/szr/internal/rules"
)

// UserFilterSources locates user-defined declarative filter spec dirs.
type UserFilterSources struct {
	GlobalDir      string
	ProjectDir     string
	ProjectEnabled bool
	Warn           io.Writer
}

func defaultUserFilterSources(cfg config.Config, paths config.Paths) UserFilterSources {
	sources := UserFilterSources{
		ProjectEnabled: cfg.Advanced.ProjectFilters,
		Warn:           os.Stderr,
	}
	if paths.ConfigDir != "" {
		sources.GlobalDir = filepath.Join(paths.ConfigDir, "filters")
	}
	if cwd, err := os.Getwd(); err == nil {
		sources.ProjectDir = filepath.Join(cwd, ".szr", "filters")
	}
	return sources
}

// LoadUserFilterProfiles compiles user (global) then project declarative
// filter specs into profiles. Project specs load only when enabled; reserved
// holds profile names a user spec may not shadow, and accepted names are
// added to it.
func LoadUserFilterProfiles(sources UserFilterSources, maxLines int, reserved map[string]struct{}) []Profile {
	loaded := loadUserFilterDir(sources.GlobalDir, SourceUserFilter, maxLines, reserved, sources.Warn)
	switch {
	case sources.ProjectDir == "":
		return loaded
	case !sources.ProjectEnabled:
		if declarative.HasUserSpecFiles(sources.ProjectDir) {
			warnUserFilter(sources.Warn, "ignoring project filters in %s (enable advanced.project_filters via szr settings)", sources.ProjectDir)
		}
		return loaded
	default:
		return append(loaded, loadUserFilterDir(sources.ProjectDir, SourceProjectFilter, maxLines, reserved, sources.Warn)...)
	}
}

func loadUserFilterDir(dir, source string, maxLines int, reserved map[string]struct{}, warn io.Writer) []Profile {
	if dir == "" {
		return nil
	}
	specs, warnings := declarative.LoadUserSpecs(dir)
	for _, warning := range warnings {
		warnUserFilter(warn, "skipping user filter: %s", warning)
	}
	loaded := make([]Profile, 0, len(specs))
	for _, spec := range specs {
		if _, exists := reserved[spec.Name]; exists {
			warnUserFilter(warn, "skipping %s filter %s (%s): name collides with an existing profile", source, spec.Name, spec.Path)
			continue
		}
		reserved[spec.Name] = struct{}{}
		loaded = append(loaded, userFilterProfile(spec, source, maxLines))
	}
	return loaded
}

func warnUserFilter(warn io.Writer, format string, args ...any) {
	if warn == nil {
		return
	}
	fmt.Fprintf(warn, "szr: "+format+"\n", args...)
}

func profileNameSet(lists ...[]Profile) map[string]struct{} {
	names := make(map[string]struct{})
	for _, list := range lists {
		for _, profile := range list {
			names[profile.Name] = struct{}{}
		}
	}
	return names
}

func userFilterProfile(spec declarative.UserSpec, source string, maxLines int) Profile {
	budgetLines := userFilterBudgetLines(spec.Spec, maxLines)
	return Profile{
		Name:             spec.Name,
		Description:      userFilterDescription(spec),
		Source:           source,
		Confidence:       ConfidenceMedium,
		StreamPreference: StreamStdoutFirst,
		Budget:           OutputBudget{MaxLines: budgetLines, MaxBytes: budgetLines * 160, MaxTokens: budgetLines * 32},
		Match:            matchUserFilter(spec.Match),
		Render:           renderUserFilter(spec.Spec),
		ParseBytes:       parseUserFilter,
		Explain:          explainUserFilter(spec, source),
	}
}

func userFilterBudgetLines(spec declarative.Spec, maxLines int) int {
	lines := spec.Head + spec.Tail
	if lines <= 0 {
		lines = maxLines
	}
	if lines <= 0 {
		lines = 12
	}
	return lines
}

func userFilterDescription(spec declarative.UserSpec) string {
	if spec.Description != "" {
		return spec.Description
	}
	return "User-defined declarative filter."
}

func matchUserFilter(match declarative.UserMatch) func(Invocation) bool {
	rule := rules.Match{
		CommandPrefix: match.CommandPrefix,
		DisplayPrefix: match.DisplayPrefix,
		AnyArgs:       match.AnyArgs,
		ExcludeArgs:   match.ExcludeArgs,
	}
	return func(inv Invocation) bool {
		return matchRule(rule, inv)
	}
}

func renderUserFilter(spec declarative.Spec) func(Invocation, Execution) string {
	return func(_ Invocation, exec Execution) string {
		input := filters.StripANSI(combineStreams(exec.Stdout, exec.Stderr))
		result, err := declarative.Apply(spec, input, declarative.Options{})
		if err != nil {
			return ""
		}
		return result.Text
	}
}

func parseUserFilter(exec Execution) int {
	return len(exec.Stdout) + len(exec.Stderr)
}

func explainUserFilter(spec declarative.UserSpec, source string) []string {
	lines := []string{
		"User-defined declarative filter (" + source + ").",
		"Loaded from " + spec.Path + ".",
	}
	if len(spec.Match.CommandPrefix) > 0 {
		lines = append(lines, "Matches command prefix `"+stringsJoin(spec.Match.CommandPrefix)+"`.")
	}
	if len(spec.Match.DisplayPrefix) > 0 {
		lines = append(lines, "Matches display prefix `"+stringsJoin(spec.Match.DisplayPrefix)+"`.")
	}
	return lines
}
