package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecuteKeepsFinalDisplayWithinCompressionContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, profiles.Builtins(6))

	cases := []struct {
		name    string
		display []string
		output  string
	}{
		{
			name:    "generic-summary",
			display: []string{"summary"},
			output:  benchmarkEngineGenericSummaryLongInput,
		},
		{
			name:    "gh-run-list",
			display: []string{"gh", "run", "list"},
			output:  benchmarkEngineGHRunListLongInput,
		},
		{
			name:    "kubectl-top",
			display: []string{"kubectl", "top", "pods"},
			output:  benchmarkEngineKubectlTopLongInput,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			script := "#!/bin/sh\ncat <<'EOF'\n" + tc.output + "\nEOF\n"
			commandPath := testutil.WriteExecutable(t, root, tc.name+"-emit", script)
			result, err := e.Execute(context.Background(), engine.Invocation{
				Command: []string{commandPath},
				Display: tc.display,
				Cwd:     root,
			}, false)
			if err != nil {
				t.Fatalf("execute %s: %v", tc.name, err)
			}

			rawTokens := history.EstimateTokens(tc.output)
			displayTokens := history.EstimateTokens(result.Display)
			allowedTokens := retainedTokenCap(rawTokens)
			if displayTokens > allowedTokens {
				t.Fatalf("expected final display <= %d tokens, got %d in %q", allowedTokens, displayTokens, result.Display)
			}
			if result.TeePath == "" {
				t.Fatalf("expected tee path for compressed successful output, got %#v", result)
			}
			if !strings.Contains(result.Display, "[recovery: ") && !strings.Contains(result.Display, "[tee: ") && !strings.Contains(result.Display, "[full output saved]") {
				t.Fatalf("expected compact recovery suffix in display, got %q", result.Display)
			}
		})
	}
}
