package cli

import (
	"testing"

	"github.com/devr-tools/szr/internal/history"
)

func TestFilterSpreadRecordsExcludesUninstallCommands(t *testing.T) {
	records := []recordLike{
		{command: "szr git status --short"},
		{command: "uninstall claude-code"},
		{command: "szr uninstall"},
		{command: "szr go test ./..."},
	}

	filtered := filterSpreadRecords(toHistoryRecords(records))
	if len(filtered) != 2 {
		t.Fatalf("expected 2 kept records, got %#v", filtered)
	}
	if filtered[0].Command != "szr git status --short" || filtered[1].Command != "szr go test ./..." {
		t.Fatalf("unexpected filtered records: %#v", filtered)
	}
}

func TestColorizeTextByRateUsesSemanticColors(t *testing.T) {
	if got := colorizeTextByRate(10, "ok", true, true); got != ansiGreen+"ok"+ansiReset {
		t.Fatalf("expected positive savings to be green, got %q", got)
	}
	if got := colorizeTextByRate(-10, "bad", true, true); got != ansiRed+"bad"+ansiReset {
		t.Fatalf("expected negative savings to be red, got %q", got)
	}
	if got := colorizeTextByRate(0, "idle", true, false); got != ansiGreen+"idle"+ansiReset {
		t.Fatalf("expected low failure rates to be green, got %q", got)
	}
}

func TestColorizeEmbeddedBarColorsSavingsBars(t *testing.T) {
	cell := "  -77.8% ▕░░░░░░░░░░░▏ "
	got := colorizeEmbeddedBar(cell, true)
	want := "  -77.8% " + ansiRed + "▕░░░░░░░░░░░▏" + ansiReset + " "
	if got != want {
		t.Fatalf("unexpected colored cell: %q", got)
	}
}

type recordLike struct {
	command string
}

func toHistoryRecords(items []recordLike) []history.Record {
	records := make([]history.Record, 0, len(items))
	for _, item := range items {
		records = append(records, history.Record{Command: item.command})
	}
	return records
}
