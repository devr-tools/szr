package engine_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/test/testutil"
)

// These tests mutate SZR_SESSION via t.Setenv and therefore must not run in
// parallel with each other.

func TestSessionScopeIsolatesDedup(t *testing.T) {
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	t.Setenv(dedup.ScopeEnv, "swarm-a")
	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	scoped := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if scoped.DedupRef == "" || !strings.Contains(scoped.Display, "unchanged from previous run") {
		t.Fatalf("expected dedup within the same scope: %#v", scoped)
	}

	t.Setenv(dedup.ScopeEnv, "swarm-b")
	otherScope := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if otherScope.DedupRef != "" || strings.Contains(otherScope.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup across scopes: %#v", otherScope)
	}

	t.Setenv(dedup.ScopeEnv, "")
	machineFirst := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if machineFirst.DedupRef != "" || strings.Contains(machineFirst.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup from scoped entries into the machine scope: %#v", machineFirst)
	}
	machineSecond := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if machineSecond.DedupRef == "" || !strings.Contains(machineSecond.Display, "unchanged from previous run") {
		t.Fatalf("expected machine-scope dedup after a machine-scope run: %#v", machineSecond)
	}

	// Refs are content-addressed, so lookups resolve across scopes: the
	// machine-scope run can expand the reference a scoped run stored.
	store := dedup.New(paths.DataDir)
	entry, ok, err := store.FindRef(scoped.DedupRef)
	if err != nil || !ok {
		t.Fatalf("expected cross-scope ref resolution: ok=%t err=%v", ok, err)
	}
	if !store.VerifyArtifact(entry) {
		t.Fatalf("expected scoped artifact to verify: %#v", entry)
	}
}

func TestSessionScopeIsolatesDeltaBaselines(t *testing.T) {
	cfg := config.Default()
	e, _, script, cwd := newDeltaTestEngine(t, cfg)

	t.Setenv(dedup.ScopeEnv, "swarm-a")
	runDeltaCommand(t, e, cfg, script, cwd)
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"a scoped edit only swarm-a has a baseline for\n")
	sameScope := runDeltaCommand(t, e, cfg, script, cwd)
	if sameScope.DeltaRef == "" || !strings.Contains(sameScope.Display, "since last run") {
		t.Fatalf("expected delta digest within the same scope: %#v", sameScope)
	}

	t.Setenv(dedup.ScopeEnv, "swarm-b")
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"another edit that swarm-b never saw a baseline for\n")
	otherScope := runDeltaCommand(t, e, cfg, script, cwd)
	if otherScope.DeltaRef != "" || strings.Contains(otherScope.Display, "since last run") {
		t.Fatalf("expected no delta baseline across scopes: %#v", otherScope)
	}

	t.Setenv(dedup.ScopeEnv, "")
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"a machine-scope edit without any machine-scope baseline\n")
	machine := runDeltaCommand(t, e, cfg, script, cwd)
	if machine.DeltaRef != "" || strings.Contains(machine.Display, "since last run") {
		t.Fatalf("expected no delta baseline from scoped entries: %#v", machine)
	}
}
