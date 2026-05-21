<p align="center">
  <img src="img/szr-gh.png" alt="szr logo" width="280" />
</p>

# szr

`szr` is short for "sizer". It is a Go-native CLI proxy that trims command output before it reaches an LLM, so the model gets the signal without paying for every line of terminal noise.

## How it works

```mermaid
flowchart LR
  C["Run: `git diff`"]
  C --> W["LLM<br/>higher tokens"]
  C --> Z["szr"]
  Z --> L["LLM<br/>lower tokens"]

  classDef base fill:#F3F4F6,stroke:#9CA3AF,color:#374151;
  classDef blue fill:#31A9F3,stroke:#31A9F3,color:#ffffff;
  class C,W base;
  class Z,L blue;
  linkStyle 0 stroke:#9CA3AF,stroke-width:2px;
  linkStyle 1,2 stroke:#31A9F3,stroke-width:2px;
```


## Current status

The first working scaffold is in place:

- `szr git status`, `szr git log`, and `szr git diff` use dedicated profiles.
- `szr go test` forces `-json` and summarizes package-level failures.
- `szr cargo test`, `szr cargo build`, and `szr cargo clippy` now use Rust-aware reducers.
- `szr uv`, `szr poetry`, `szr pip`, `szr pytest`, `szr ruff`, and `szr mypy` now use Python-aware reducers.
- `szr make`, `szr just`, `szr task`, `szr bazel`, `szr ninja`, and `szr cmake` now use build-system reducers.
- `szr ctest`, `szr clang-tidy`, `szr clang-format`, and `szr bear` now use C/C++ tooling reducers.
- `szr docker ps`, `szr docker compose ps`, `szr docker logs`, and `szr docker compose logs` now use container-aware reducers.
- `szr kubectl get`, `szr kubectl describe`, and `szr kubectl logs` now use Kubernetes-aware reducers.
- `szr gh pr view`, `szr gh run view`, and `szr gh run view --log` now use GitHub-aware reducers.
- `szr npm`, `szr pnpm`, `szr yarn`, `szr turbo`, `szr nx`, `szr vite`, `szr vitest`, and `szr jest` now use JS/TS-aware profiles.
- `szr diff`, `szr patch`, and `szr git apply` now use patch-aware reducers.
- `szr rg` now uses a ripgrep-aware reducer that groups matches by file.
- `szr read`, `szr grep`, `szr json`, `szr log`, and `szr ls` are implemented directly in Go.
- `szr spread` tracks token savings in local JSONL history.
- `szr explain <cmd...>` shows which profile the engine would use and why.
- `szr tee` lists indexed failure artifacts so full logs can be retrieved only when needed.
- `szr install <target>` bootstraps repo-local instructions and hook files for Codex, Claude Code, Cursor, Gemini, and plain shell environments.
- `szr bench` runs built-in compression fixtures to measure latency and output savings.
- project-local `.szr.json`, `.szr.yaml`, and `.szr.yml` rules can define custom match, rewrite, render, and cwd-aware behavior.

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
szr cargo test
szr pytest -k failing_case
szr ruff check .
szr mypy src
szr make test
szr ctest
szr docker logs api
szr kubectl get pods
szr gh run view 123
szr gh run view 123 --log
szr npm test
szr turbo run build
szr git apply fix.patch
szr rg TODO internal
szr grep "TODO" .
szr read internal/cli/app.go --level aggressive
szr spread --history
szr tee
szr tee --latest
szr install codex
szr install shell
szr bench clean-pass
szr explain go test ./...
```

## Local Development

The test suite now lives under `test/`, with coverage enforced against `./internal/...`.
The public Go package lives under `pkg/szr`, while `cmd/szr-dev` is the developer-only launcher path.

```bash
make test
make cover
make smoke
make prepush
```

## Architecture

The codebase is organized around a small execution engine:

- `pkg/szr`: public package entrypoint for embedding or thin launchers
- `cmd/szr`: binary wrapper with `main.go` only
- `cmd/szr-dev`: developer-only binary wrapper
- `internal/bench`: benchmark fixtures and measurement harness
- `internal/cli`: subcommand parsing and dispatch
- `internal/engine`: profile matching, command execution, tee handling
- `internal/installers`: repo-local bootstrap generation for agent/editor targets
- `internal/profiles`: built-in profile registry
- `internal/filters`: text, JSON, grep, git, and test compaction logic
- `internal/history`: local tracking for `szr spread`
- `internal/rules`: project-local rule DSL parsing and validation
- `internal/szrdev`: developer-only launcher wiring
- `internal/config`: config and runtime path resolution

More detail lives in [docs/ARCHITECTURE.md](/Users/alex/Documents/GitHub/szr/docs/ARCHITECTURE.md).

## Roadmap

The current roadmap is organized around three goals:

- lower wrapper latency so `szr` stays on the hot path
- expand parser-first coverage beyond the current Go, Git, Rust, Python, JS/TS, build, C/C++, patch, container, Kubernetes, and GitHub command families
- improve compression quality without hiding actionable failure lines, and extend the bench harness as profiles grow

The detailed plan lives in [docs/ROADMAP.md](/Users/alex/Documents/GitHub/szr/docs/ROADMAP.md).
