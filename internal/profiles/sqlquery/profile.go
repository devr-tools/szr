package sqlquery

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	sqlfilter "github.com/devr-tools/szr/internal/filters/sqlquery"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "sql-query",
			Description:      "Summarizes common psql, sqlite3, mysql, and duckdb query commands around result rows, row counts, and SQL errors.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isSQLQueryCommand(inv.Display) || isSQLQueryCommand(inv.Command)
			},
			Prepare: func(inv engine.Invocation) []string {
				if !inv.Advanced.AggressivePrepareRewrites {
					return inv.Command
				}
				return prepareSQLQueryCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return sqlfilter.SummarizeSQLQuery(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					true,
					func(input string) string {
						return sqlfilter.SummarizeSQLQuery(input, budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return sqlfilter.SQLQueryRecoveryInfo(input, budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches non-interactive query invocations for `psql`, `sqlite3`, `mysql`, and `duckdb`.",
				"In aggressive prepare mode, prefers quieter or machine-friendlier formats when the user did not already choose one.",
				"Keeps result rows, row-count summaries, and engine-specific SQL errors while trimming CLI banners and table borders.",
			},
		},
	}
}

func isSQLQueryCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch filepath.Base(args[0]) {
	case "psql":
		return flagValue(args[1:], "-c", "--command") != ""
	case "mysql":
		return flagValue(args[1:], "-e", "--execute") != ""
	case "sqlite3":
		if looksLikeSQL(flagValue(args[1:], "-cmd")) {
			return true
		}
		return hasSQLPositional(args[1:], true)
	case "duckdb":
		if query := flagValue(args[1:], "-c", "--command"); looksLikeSQL(query) {
			return true
		}
		return hasSQLPositional(args[1:], true)
	default:
		return false
	}
}

func prepareSQLQueryCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	args := command[1:]
	out := append([]string{}, command...)
	switch filepath.Base(command[0]) {
	case "psql":
		if !hasAnyArg(args, "-q", "--quiet") {
			out = append(out, "-q")
		}
		if !hasPSQLFormatArg(args) {
			out = append(out, "--csv")
		}
	case "sqlite3":
		if !hasSQLiteFormatArg(args) {
			out = append(out, "-json")
		}
	case "mysql":
		if !hasAnyArg(args, "--batch", "-B", "--table", "-t") {
			out = append(out, "--batch")
		}
		if !hasAnyArg(args, "--raw", "-r") {
			out = append(out, "--raw")
		}
	case "duckdb":
		if !hasDuckDBFormatArg(args) {
			out = append(out, "-json")
		}
	}
	return out
}

func flagValue(args []string, names ...string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		for _, name := range names {
			if arg == name && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(arg, name+"=") {
				return strings.TrimPrefix(arg, name+"=")
			}
		}
	}
	return ""
}

func hasSQLPositional(args []string, skipFirst bool) bool {
	positional := 0
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if skipFirst && positional == 0 {
			positional++
			continue
		}
		if looksLikeSQL(arg) {
			return true
		}
		positional++
	}
	return false
}

func looksLikeSQL(input string) bool {
	trimmed := strings.TrimSpace(strings.Trim(input, `"'`))
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, keyword := range []string{
		"select ", "with ", "insert ", "update ", "delete ", "replace ",
		"show ", "describe ", "desc ", "explain ", "pragma ", "create ",
		"alter ", "drop ", "vacuum", "analyze", "attach ", "detach ",
	} {
		if strings.HasPrefix(lower, keyword) {
			return true
		}
	}
	return false
}

func hasAnyArg(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func hasPSQLFormatArg(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "--csv", arg == "-A", arg == "--no-align", arg == "-H", arg == "--html", arg == "-x", arg == "--expanded":
			return true
		case strings.HasPrefix(arg, "--pset"), strings.HasPrefix(arg, "-P"):
			return true
		}
	}
	return false
}

func hasSQLiteFormatArg(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-json", "-csv", "-table", "-line", "-list", "-markdown", "-quote", "-tabs", "-column":
			return true
		}
	}
	return false
}

func hasDuckDBFormatArg(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-json", "-csv", "-table", "-markdown", "-list":
			return true
		}
	}
	return false
}
