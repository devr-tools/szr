# Architecture & System Boundaries

Key architectural decisions, service boundaries, data flow, integration points, and why things are the way they are.

## Local JSONL stores (history, tee index, dedup index)

- Every record embeds the full command text, which is unbounded. Writers must clip it (`jsonl.Clip`) and readers must use `jsonl.Scan`, which skips lines over the cap instead of failing. A plain `bufio.Scanner` caps tokens at 64 KiB and treats a longer line as a fatal error, which permanently broke `szr spread`, `doctor`, `recommend`, and `discover` on one long command.
- `history.LoadAll` runs on most commands (adaptive budget lookup), so anything that makes it slower or fallible is a per-command cost, not a reporting-only cost.

## History compaction and lifetime totals

- Compaction (2 MiB / 2500 records, hardcoded in `internal/history/store.go`) deletes records, so anything derived only from records on disk silently shrinks over time. Records it removes are folded into `history-totals.json` beside the history file.
- **When adding a field to `history.Summary`, decide explicitly whether it is additive.** Additive counters must also be added to `history.Totals` and `foldRecords`/`addArchivedCounters`, or the field quietly becomes window-only after the first compaction. Percentiles, per-command tables, and hotspot scores cannot be rebuilt from sums and are deliberately left window-scoped, with `Summary.ArchivedCommands` telling readers how many runs the totals cover beyond them.
- Any record filter applied for reporting must be applied identically at fold time (`SavingsExcludedCommand` is shared between `filterSpreadRecords` and `foldRecords`), otherwise compaction visibly changes the reported numbers.
- `compact()` archives only *after* the compacted file is renamed into place. A crash between the two loses one batch of counters, which is deliberate: double counting would permanently inflate reported savings.
- An oversized-but-parseable record is repaired (command clipped, measurements kept), not dropped. Reads set a pending-repair flag so the next `Append` compacts even below the size trigger; otherwise the bloated line is re-parsed by every command until the file happens to cross 2 MiB.

## Local CI gates

- CI runs `golangci-lint --new-from-rev <merge-base>`, so `maintidx`/`gocognit` findings in new code fail the build even though the repo carries hundreds of pre-existing ones. Long straight-line field arithmetic trips `maintidx` (Halstead volume) — split it into helpers.
- codeguard enforces `max_file_lines: 400` per file; `internal/cli/spread.go` sits close to it.
- `make test`/`make cover` write `.gomodcache/` in the repo, and `make commit` runs `git add .`.
