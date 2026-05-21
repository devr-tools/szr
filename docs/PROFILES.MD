# Profiles

This document tracks the builtin profiles currently registered by `internal/profiles.Builtins()`.

Current coverage: `27` builtin profiles across `12` categories.

Anything not listed here does not currently have a dedicated builtin profile in `szr`.

## Coverage Summary

| Category | Count | Covered commands and workflows |
| --- | ---: | --- |
| Go | 2 | `go test`, `go build`, `go vet` |
| Generic fallback | 2 | wrapped `test` commands, shallow `summary` commands |
| Git | 3 | `git status`, `git log`, `git diff` |
| GitHub CLI | 3 | `gh pr view`, `gh run view`, GitHub Actions logs |
| JavaScript | 4 | `npm`, `pnpm`, `yarn`, `turbo`, `nx`, `vite`, `vitest`, `jest` |
| Python | 2 | `uv`, `poetry`, `pip`, `pytest`, `ruff`, `mypy` |
| Build systems | 1 | `make`, `just`, `task`, `bazel`, `ninja`, `cmake` |
| C/C++ tooling | 2 | `ctest`, `clang-tidy`, `clang-format`, `bear` |
| Patch and diff | 1 | `diff`, `patch`, `git apply` |
| Rust | 2 | `cargo test`, `cargo build`, `cargo clippy` |
| Containers | 2 | `docker ps`, `docker logs`, `docker compose ps`, `docker compose logs` |
| Kubernetes | 3 | `kubectl get`, `kubectl describe`, `kubectl logs` |

## Profile Inventory

### Go

- `go-test-json`
  - Matches: `go test`
  - Focus: forces `-json` and keeps package-level failures, tests, and panic lines
- `go-build`
  - Matches: `go build`, `go vet`
  - Focus: trims download noise and surfaces compiler diagnostics first

### Generic fallback

- `generic-test`
  - Matches: wrapped commands whose display name starts with `test`
  - Focus: best-effort failure extraction when no dedicated structured parser exists
- `generic-summary`
  - Matches: commands whose display name starts with `summary`
  - Focus: shallow preview of the first informative lines from long output

### Git

- `git-status`
  - Matches: `git status`
  - Focus: branch state plus staged, unstaged, and untracked file counts
- `git-log`
  - Matches: `git log`
  - Focus: compact oneline history preview
- `git-diff`
  - Matches: `git diff`
  - Focus: file churn and `--stat` style diff summaries

### GitHub CLI

- `gh-pr-view`
  - Matches: `gh pr view`
  - Focus: pull request metadata, review state, and changed files
- `gh-run-log`
  - Matches: `gh run view --log`, `gh run view --log-failed`
  - Focus: failed GitHub Actions jobs and failing steps from raw logs
- `gh-run-view`
  - Matches: `gh run view`
  - Focus: workflow status, branch or event context, failed jobs, and failed steps

### JavaScript

- `js-package-test`
  - Matches: `npm test`, `pnpm test`, `yarn test`, plus `run test` variants
  - Focus: detects `vitest` or `jest` behind package scripts and forwards structured reporter flags
- `js-workspace`
  - Matches: non-test `npm`, `pnpm`, and `yarn` commands plus `turbo`, `nx`, and `vite`
  - Focus: package-manager failures, workspace task failures, and file-anchored Vite or build diagnostics
- `vitest-json`
  - Matches: direct `vitest` commands
  - Focus: requests JSON reporter output and preserves failing suite details
- `jest-json`
  - Matches: direct `jest` commands
  - Focus: requests JSON output and condenses failing suite signal

### Python

- `pytest`
  - Matches: `pytest`, `python -m pytest`, `uv run pytest`
  - Focus: low-noise pytest output with failing test ids, assertion lines, and short summaries
- `python-tooling`
  - Matches: `uv`, `poetry`, `pip`, `pip3`, `ruff`, `mypy`, plus `python -m pip|ruff|mypy`
  - Focus: package resolution, lint, and type-check errors with file anchors and rule codes

### Build systems

- `build-system`
  - Matches: `make`, `just`, `task`, `bazel`, `ninja`, `cmake`
  - Focus: failing targets, orchestration errors, and source-file diagnostics

### C/C++ tooling

- `ctest`
  - Matches: `ctest`
  - Focus: failing test names, output-on-failure bodies, and assertion locations
- `clang-tooling`
  - Matches: `clang-tidy`, `clang-format`, `bear`
  - Focus: file-level warnings and errors plus compilation-database generation signal

### Patch and diff

- `patch-diff`
  - Matches: `diff`, `patch`, `git apply`
  - Focus: touched files, hunk counts, and patch-application failures

### Rust

- `cargo-test`
  - Matches: `cargo test`
  - Focus: failing tests, panic lines, and compiler diagnostics
- `cargo-build`
  - Matches: `cargo build`, `cargo clippy`
  - Focus: compiler diagnostics, warnings, and hints

### Containers

- `docker-ps`
  - Matches: `docker ps`, `docker compose ps`
  - Focus: container state, service names, and image identifiers
- `docker-logs`
  - Matches: `docker logs`, `docker compose logs`
  - Focus: bounded log tails with repeated failures collapsed by service or container

### Kubernetes

- `kubectl-get`
  - Matches: `kubectl get`
  - Focus: resource names, namespaces, readiness, replicas, and phase/status signal
- `kubectl-describe`
  - Matches: `kubectl describe`
  - Focus: object identity plus warning and failure-bearing events
- `kubectl-logs`
  - Matches: `kubectl logs`
  - Focus: bounded log tails with prefixed source identity and grouped repeated errors

## Expansion Read

The current builtin set is strongest in:

- local dev and test loops
- build and lint orchestration
- source control and PR inspection
- container and Kubernetes operational output

The main expansion surface is now deeper structured handling inside the covered ecosystems rather than missing first-pass command families. The next clean expansion path is more tool-specific machine-readable modes, richer domain grouping, and better reducers for commands that still share broad category profiles.
