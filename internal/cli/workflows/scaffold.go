package workflows

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RunScaffold(rt Runtime, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: scaffold requires a subcommand")
		return 2
	}
	switch args[0] {
	case "profile":
		return RunScaffoldProfile(rt, args[1:])
	default:
		fmt.Fprintf(rt.Stderr, "szr: unknown scaffold subcommand %s\n", args[0])
		return 2
	}
}

func RunScaffoldProfile(rt Runtime, args []string) int {
	printOnly := false
	builtin := false
	name := ""
	for _, arg := range args {
		switch arg {
		case "--print":
			printOnly = true
		case "--builtin":
			builtin = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(rt.Stderr, "szr: unknown scaffold profile flag %s\n", arg)
				return 2
			}
			if name != "" {
				fmt.Fprintln(rt.Stderr, "szr: scaffold profile accepts exactly one profile name")
				return 2
			}
			name = arg
		}
	}
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(rt.Stderr, "szr: scaffold profile requires a name")
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	files := scaffoldProfileFiles(cwd, name, builtin)
	relative := rt.RelativeToRepo
	if relative == nil {
		relative = func(_ string, path string) string { return path }
	}
	if printOnly {
		fmt.Fprintf(rt.Stdout, "plan: scaffold profile %s\n", name)
		for path, content := range files {
			fmt.Fprintf(rt.Stdout, "  %s (%d bytes)\n", relative(cwd, path), len(content))
		}
		return 0
	}
	for path, content := range files {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(rt.Stderr, "szr: scaffold target already exists: %s\n", path)
			return 1
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(rt.Stderr, "szr: failed to create %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(rt.Stderr, "szr: failed to write %s: %v\n", path, err)
			return 1
		}
	}
	fmt.Fprintf(rt.Stdout, "scaffolded profile %s\n", name)
	for path := range files {
		fmt.Fprintf(rt.Stdout, "  %s\n", relative(cwd, path))
	}
	return 0
}

func scaffoldProfileFiles(root, name string, builtin bool) map[string]string {
	slug := sanitizeSlug(name)
	files := map[string]string{}
	if builtin {
		files[filepath.Join(root, "internal", "profiles", slug, "profile.go")] = builtinProfileStub(slug)
		files[filepath.Join(root, "test", "profiles", slug, "render_test.go")] = builtinProfileTestStub(slug)
		return files
	}
	base := filepath.Join(root, ".szr", "scaffold", slug)
	files[filepath.Join(base, "profile.yaml")] = customProfileStub(slug)
	files[filepath.Join(base, "raw.stdout.txt")] = "replace with representative raw stdout for this command family\n"
	files[filepath.Join(base, "raw.stderr.txt")] = "replace with representative raw stderr for this command family\n"
	files[filepath.Join(base, "expected.txt")] = "replace with the reducer output you want to preserve\n"
	files[filepath.Join(base, "README.md")] = scaffoldReadme(slug)
	return files
}

func customProfileStub(name string) string {
	return fmt.Sprintf(`version: 1
profiles:
  - name: %s
    description: Summarize the %s command family for agent-friendly review.
    explain:
      - Matches the target command family.
      - Rewrites the command into a more structured form before rendering compact output.
    match:
      command_prefix:
        - your-cli
        - subcommand
    rewrite:
      mode: append
      args:
        - --json
    render:
      mode: failure
      max_lines: 12
`, name, name)
}

func builtinProfileStub(name string) string {
	return fmt.Sprintf(`package %s

import "github.com/devr-tools/szr/internal/engine"

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{{
		Name:        %q,
		Description: "Summarizes %s output into an agent-friendly preview.",
		Confidence:  engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Command) >= 2 && inv.Command[0] == "your-cli" && inv.Command[1] == "subcommand"
		},
		Prepare: func(inv engine.Invocation) []string {
			return append([]string(nil), inv.Command...)
		},
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return exec.Stdout
		},
		Explain: []string{
			"Matches the intended command family explicitly.",
			"Preserves the minimum set of identifiers and failure details needed for follow-up actions.",
		},
	}}
}
`, name, name, name)
}

func builtinProfileTestStub(name string) string {
	return fmt.Sprintf(`package %s_test

import (
	"testing"

	"github.com/devr-tools/szr/test/testutil"
)

func Test%sStub(t *testing.T) {
	_ = testutil.NewTestApp(t)
	t.Skip("replace scaffolded stub with reducer coverage")
}
`, name, toTitle(name))
}

func scaffoldReadme(name string) string {
	return fmt.Sprintf(`# %s scaffold

1. Capture representative command output into `+"`raw.stdout.txt`"+` and `+"`raw.stderr.txt`"+`.
2. Update `+"`profile.yaml`"+` or move the logic into a builtin profile.
3. Replace `+"`expected.txt`"+` with the reduced output contract you want tests to enforce.
`, name)
}

func sanitizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	if value == "" {
		return "profile"
	}
	return value
}

func toTitle(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
