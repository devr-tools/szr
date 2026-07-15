package profiles_test

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

// kubectl writes "No resources found in <namespace>" to STDERR and exits 0,
// while the kubectl-get profile reduces stdout only. The engine must render
// the compact stderr message instead of flagging an empty-render fallback.
func TestKubectlGetRendersNoResourcesStderrWithoutFallback(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	kubectlPath := testutil.WriteExecutable(t, binDir, "kubectl", "#!/bin/sh\nprintf 'No resources found in default namespace.\\n' >&2\nexit 0\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	e := engine.New(config.Default(), paths, store, profiles.Builtins(12))

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{kubectlPath, "get", "pods"},
		Display: []string{"kubectl", "get", "pods"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute kubectl get: %v", err)
	}
	if result.ProfileName != "kubectl-get" {
		t.Fatalf("expected kubectl-get profile, got %#v", result)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected benign exit, got %#v", result)
	}
	if !strings.Contains(result.Display, "No resources found in default namespace.") {
		t.Fatalf("expected compact stderr message in display, got %q", result.Display)
	}
	if result.FallbackUsed {
		t.Fatalf("did not expect fallback for empty kubectl get result, got %#v", result)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 {
		t.Fatalf("expected one history record, got %#v", records)
	}
	rec := records[0]
	if rec.FallbackUsed || rec.EmptyResult || rec.ExitCode != 0 || rec.SavedTokens < 0 {
		t.Fatalf("expected clean non-failure record, got %#v", rec)
	}
}
