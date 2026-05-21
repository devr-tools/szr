<p align="center">
  <img src="img/szr-gh.png" alt="szr logo" width="280" />
</p>

# szr

`szr` is short for "sizer". It is a Go-native CLI proxy that trims command output before it reaches an LLM, so the model gets the signal without paying for every line of terminal noise.



## Current status

The first working scaffold is in place:

- `szr git status`, `szr git log`, and `szr git diff` use dedicated profiles.
- `szr go test` forces `-json` and summarizes package-level failures.
- `szr read`, `szr grep`, `szr json`, `szr log`, and `szr ls` are implemented directly in Go.
- `szr gain` tracks token savings in local JSONL history.
- `szr explain <cmd...>` shows which profile the engine would use and why.

## Why build this in Go

- Fast single-binary distribution.
- Straightforward cross-platform process execution.
- Good fit for structured parsers and profile registries.
- Easier to keep the architecture modular as the command surface grows.

## Usage

```bash
szr git status
szr git diff
szr go test ./...
szr grep "TODO" .
szr read internal/cli/app.go --level aggressive
szr gain --history
szr explain go test ./...
```

## Architecture

The codebase is organized around a small execution engine:

- `cmd/szr`: entrypoint
- `internal/cli`: subcommand parsing and dispatch
- `internal/engine`: profile matching, command execution, tee handling
- `internal/profiles`: built-in profile registry
- `internal/filters`: text, JSON, grep, git, and test compaction logic
- `internal/history`: local tracking for `szr gain`
- `internal/config`: config and runtime path resolution

More detail lives in [docs/ARCHITECTURE.md](/Users/alex/Documents/GitHub/szr/docs/ARCHITECTURE.md).

## Roadmap

- Add JS/TS, Python, Docker, and GitHub CLI profiles.
- Add profile benchmarking and compression-quality fixtures.
- Add agent hook installers and project instruction generation.
- Add project-local custom rules so teams can teach `szr` their own command patterns.
