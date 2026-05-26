package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
)

func TestResolveBudgetWithHistoryAdapterTightensConservatively(t *testing.T) {
	store := history.New(filepath.Join(t.TempDir(), "history.jsonl"))
	fingerprint := history.Fingerprint("szr tight")
	for i := 0; i < 4; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 24, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr tight",
			CommandFingerprint: fingerprint,
			Profile:            "custom",
			RawTokens:          120,
			FilteredTokens:     88,
			SavedTokens:        32,
			SavingsPct:         26.67,
			BytesEmitted:       360,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	profile := Profile{Name: "custom", Budget: OutputBudget{MaxLines: 20}}
	budget, adaptation := ResolveBudgetWithAdapter(profile, Invocation{
		Display:  []string{"szr", "tight"},
		Advanced: config.Advanced{AdaptiveBudgets: true},
	}, 12, NewHistoryBudgetAdapter(store))
	if adaptation == nil {
		t.Fatal("expected budget adaptation")
	}
	if adaptation.Direction != string(history.BudgetSuggestionTighten) || adaptation.Suggested.MaxLines >= 20 {
		t.Fatalf("unexpected adaptation metadata: %#v", adaptation)
	}
	if budget.MaxLines != 16 || budget.MaxBytes != 2560 || budget.MaxTokens != 512 {
		t.Fatalf("expected conservative tighten cap, got %#v", budget)
	}
}

func TestExecuteAppliesHistoryDrivenBudgetLoosening(t *testing.T) {
	binDir := t.TempDir()
	var body strings.Builder
	body.WriteString("#!/bin/sh\n")
	for i := 1; i <= 20; i++ {
		body.WriteString(fmt.Sprintf("printf 'line-%02d with enough payload to avoid tiny bypass\\n'\n", i))
	}
	cmdPath := writeExecutable(t, filepath.Join(binDir, "loosen"), body.String())

	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	store := history.New(paths.HistoryFile)
	fingerprint := history.Fingerprint("loosen")
	for i := 0; i < 6; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 24, 12+i, 0, 0, 0, time.UTC),
			Command:            "loosen",
			CommandFingerprint: fingerprint,
			Profile:            "streaming-summary",
			RawTokens:          600,
			FilteredTokens:     480,
			SavedTokens:        120,
			SavingsPct:         20,
			BytesEmitted:       2400,
			FallbackUsed:       true,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	var seenBudget OutputBudget
	cfg := config.Default()
	cfg.Advanced.AdaptiveBudgets = true
	e := New(cfg, paths, store, []Profile{{
		Name: "streaming-summary",
		Match: func(inv Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "loosen"
		},
		Budget: OutputBudget{MaxLines: 10},
		StreamRender: func(_ Invocation, budget OutputBudget) StreamReducer {
			seenBudget = budget
			return &budgetLineReducer{limit: budget.MaxLines}
		},
	}})

	result, err := e.Execute(context.Background(), Invocation{
		Command:  []string{cmdPath},
		Display:  []string{"loosen"},
		Cwd:      root,
		Advanced: config.Advanced{AdaptiveBudgets: true},
	}, false)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if seenBudget.MaxLines != 15 || seenBudget.MaxBytes != 2400 || seenBudget.MaxTokens != 480 {
		t.Fatalf("expected conservative loosen cap, got %#v", seenBudget)
	}
	if strings.TrimSpace(result.Display) == "" {
		t.Fatalf("expected adapted reducer display, got %q", result.Display)
	}
	if result.TeePath == "" {
		t.Fatalf("expected recovery artifact for compressed adapted output, got %#v", result)
	}
}

type budgetLineReducer struct {
	limit int
	buf   strings.Builder
}

func (r *budgetLineReducer) ConsumeStdout(chunk []byte) {
	_, _ = r.buf.Write(chunk)
}

func (r *budgetLineReducer) ConsumeStderr(chunk []byte) {
	_, _ = r.buf.Write(chunk)
}

func (r *budgetLineReducer) Result() string {
	lines := strings.Split(strings.TrimRight(r.buf.String(), "\n"), "\n")
	if r.limit > 0 && len(lines) > r.limit {
		lines = lines[:r.limit]
	}
	return strings.Join(lines, "\n")
}

func (r *budgetLineReducer) BytesParsed() int {
	return r.buf.Len()
}

func (r *budgetLineReducer) FallbackUsed() bool {
	return false
}

func writeExecutable(t *testing.T, path string, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
	return path
}
