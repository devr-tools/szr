# szr Architecture

`szr` is a Go-native command proxy for reducing terminal output before it reaches an LLM context.

## Design goals

- Small, single-binary runtime.
- Explicit profile registry instead of a giant monolithic command router.
- Safe failure behavior: keep the wrapped command's exit code.
- Local-only analytics with no network dependency.
- Easy addition of new command profiles and filter stages.

## Current structure

```text
cmd/szr
internal/cli
internal/config
internal/engine
internal/filters
internal/history
internal/profiles
```

## Execution flow

1. The CLI parses global flags and dispatches a subcommand.
2. External commands are converted into an `engine.Invocation`.
3. The profile registry matches the invocation.
4. The engine optionally rewrites the command for a better machine-readable form.
5. The command runs and the selected renderer compresses the output.
6. A local history record is appended for `szr gain`.
7. On failures, raw output can be tee'd into the local data directory.

## Built-in profile strategy

- `git status`: normalize to `--short --branch`, then count staged, unstaged, and untracked files.
- `git log`: prefer `--oneline -n 20` and keep a short commit preview.
- `git diff`: prefer `--stat` and summarize file churn.
- `go test`: force `-json` and collapse pass noise into package and failure summaries.
- `go build` / `go vet`: bias toward compiler diagnostics.
- Generic test and summary wrappers: provide fallback compaction for unstructured tools.

## Why this is better than cloning the Rust layout

- The Go version is profile-driven from the start, so adding new tools does not require expanding a single mega-enum.
- Local file helpers like `read`, `json`, and `log` are implemented directly in Go instead of always shelling out.
- The analytics store is deliberately simple JSONL first; it can be swapped for SQLite later without changing the CLI surface.
- `szr explain` makes the filter decision visible, which is useful when the registry grows.

## Next steps

- Add package-manager aware JS/TS profiles.
- Add hook and instruction installers for Codex, Claude Code, Cursor, and Gemini.
- Add a custom rule DSL so users can define project-local profiles.
- Add benchmark fixtures to measure compression quality and latency.

The broader product plan now lives in [ROADMAP.md](/Users/alex/Documents/GitHub/szr/docs/ROADMAP.md).
