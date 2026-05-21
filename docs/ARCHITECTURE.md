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
pkg/szr
cmd/szr
cmd/szr-dev
internal/bench
internal/cli
internal/config
internal/engine
internal/filters
internal/history
internal/installers
internal/profiles
internal/rules
internal/szrdev
```

## Execution flow

1. `cmd/szr/main.go` calls the public `pkg/szr` package.
2. The CLI parses global flags and dispatches a subcommand.
3. External commands are converted into an `engine.Invocation`.
4. The profile registry matches the invocation.
5. The engine optionally rewrites the command for a better machine-readable form.
6. The command runs and the selected renderer compresses the output.
7. A local history record is appended for `szr spread`.
8. On failures, raw output can be tee'd into the local data directory.
9. Installer and benchmark commands reuse dedicated internal packages instead of embedding that logic in the core router.

## Built-in profile strategy

- `git status`: normalize to `--short --branch`, then count staged, unstaged, and untracked files.
- `git log`: prefer `--oneline -n 20` and keep a short commit preview.
- `git diff`: prefer `--stat` and summarize file churn.
- `go test`: force `-json` and collapse pass noise into package and failure summaries.
- `npm test` / `pnpm test` / `yarn test`: inspect `package.json`, detect `jest` or `vitest`, and forward structured reporter flags.
- `vitest` / `jest`: prefer JSON-capable output and summarize failing suites, assertions, and file paths.
- `go build` / `go vet`: bias toward compiler diagnostics.
- Generic test and summary wrappers: provide fallback compaction for unstructured tools.

## Why this is better than cloning the Rust layout

- The Go version is profile-driven from the start, so adding new tools does not require expanding a single mega-enum.
- Local file helpers like `read`, `json`, and `log` are implemented directly in Go instead of always shelling out.
- The analytics store is deliberately simple JSONL first; it can be swapped for SQLite later without changing the CLI surface.
- `szr explain` makes the filter decision visible, which is useful when the registry grows.
- `pkg/szr` keeps the public embedding surface explicit, while `internal/szrdev` contains developer-only launcher wiring.
- Project-local `.szr.json` rules compile into the same profile model, so local overrides stay deterministic and inspectable.
- The benchmark harness lives beside embedded fixtures, which keeps compression measurements reproducible without polluting the core execution path.

## Next steps

- Add more semantic profiles for Python, Rust, Docker, Kubernetes, and GitHub CLI workflows.
- Expand `szr bench` into a richer comparison surface with per-profile aggregates and failure indicators.
- Grow the rule DSL carefully without turning local config into a second monolithic router.
- Add editor- and agent-specific install refinements on top of the current repo-local bootstrap flow.

The broader product plan now lives in [ROADMAP.md](/Users/alex/Documents/GitHub/szr/docs/ROADMAP.md).
