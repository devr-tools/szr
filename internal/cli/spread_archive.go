package cli

import (
	"fmt"

	"github.com/devr-tools/szr/internal/history"
)

// renderSpreadArchived states which numbers cover the archived runs and which
// only cover the records still on disk, so a compacted history never reads as
// if the tables below describe every run.
func renderSpreadArchived(ui spreadUI, summary history.Summary) {
	if summary.ArchivedCommands == 0 && summary.ArchivedDroppedRecords == 0 {
		return
	}
	detail := fmt.Sprintf(
		"%d older runs folded into the totals above - durations, tables, and hotspots below cover the %d runs still on disk",
		summary.ArchivedCommands,
		summary.Commands-summary.ArchivedCommands,
	)
	if summary.ArchivedDroppedRecords > 0 {
		detail += fmt.Sprintf("; %d unreadable records were discarded", summary.ArchivedDroppedRecords)
	}
	ui.metric("archived history", detail, "")
}
