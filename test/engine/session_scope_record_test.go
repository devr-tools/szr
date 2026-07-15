package engine_test

import (
	"context"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
)

func TestExecuteRecordsSessionScopeFromEnv(t *testing.T) {
	t.Setenv("SZR_SESSION", "sess-scope-1")
	e, root, paths, succeedPath, _, _ := newExecuteTestEngine(t)
	if _, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{succeedPath},
		Display: []string{"succeed"},
		Cwd:     root,
	}, false); err != nil {
		t.Fatalf("execute: %v", err)
	}

	records, err := history.New(paths.HistoryFile).LoadAll()
	if err != nil || len(records) != 1 {
		t.Fatalf("load history: %v records=%+v", err, records)
	}
	if records[0].SessionScope != "sess-scope-1" {
		t.Fatalf("expected session scope on record, got %+v", records[0])
	}
}
