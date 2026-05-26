# Profiles

This document tracks the builtin profiles currently registered by `internal/profiles.Builtins()`.

Current coverage: `41` builtin profiles across `14` categories.

Anything not listed here does not currently have a dedicated builtin profile in `szr`.

## Coverage Summary

| Category | Count | Covered commands and workflows |
| --- | ---: | --- |
| Go | 2 | `go test`, `go build`, `go vet` |
| Generic fallback | 2 | wrapped `test` commands, shallow `summary` commands |
| Search | 2 | `rg` plain-text search, `find` path discovery |
| Git | 3 | `git status`, `git log`, `git diff` |
| GitHub CLI | 5 | `gh pr view`, `gh pr checks`, `gh run list`, `gh run view`, GitHub Actions logs |
| JavaScript | 5 | `npm`, `pnpm`, `yarn`, `bun`, `turbo`, `nx`, `vite`, `vitest`, `jest`, `eslint`, `tsc` |
| PHP | 1 | `composer`, `phpunit`, `pest`, `phpstan`, `psalm`, `phpcs`, `phpcbf`, `php-cs-fixer`, `php artisan` |
| Python | 2 | `uv`, `poetry`, `pip`, `pytest`, `ruff`, `mypy` |
| Build systems | 1 | `make`, `just`, `task`, `bazel`, `ninja`, `cmake` |
| C/C++ tooling | 2 | `ctest`, `clang-tidy`, `clang-format`, `bear` |
| Patch and diff | 1 | `diff`, `patch`, `git apply` |
| Rust | 2 | `cargo test`, `cargo build`, `cargo clippy` |
| Containers | 2 | `docker ps`, `docker logs`, `docker compose ps`, `docker compose logs` |
| Kubernetes | 5 | `kubectl get`, `kubectl describe`, `kubectl logs`, `kubectl top`, `kubectl events` |

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

### Search

- `ripgrep`
  - Matches: plain-text `rg` search commands outside JSON, files-only, and count-oriented modes
  - Focus: groups matches by file with stable filename and line-number output, applies conservative repo-noise excludes, and streams to early cutoff when enough visible context is collected
- `path-find`
  - Matches: plain `find` commands that emit path lists without `-exec`, `-delete`, or custom print formats
  - Focus: bounded path previews for file-discovery output with repo-noise suppression and streaming cutoff

### Git

- `git-status`
  - Matches: `git status`
  - Focus: branch state plus staged, unstaged, and untracked file counts
- `git-log`
  - Matches: `git log`
  - Focus: compact oneline history preview
- `git-diff`
  - Matches: `git diff`
  - Focus: file churn and `--stat` style diff summaries, with top-churn truncation for large diffs and a tighter aggressive mode

### GitHub CLI

- `gh-pr-view`
  - Matches: `gh pr view`
  - Focus: pull request metadata, review state, and changed files
- `gh-pr-checks`
  - Matches: `gh pr checks`
  - Focus: failed and pending check rows from raw check output
- `gh-run-log`
  - Matches: `gh run view --log`, `gh run view --log-failed`
  - Focus: failed GitHub Actions jobs and failing steps from raw logs
- `gh-run-list`
  - Matches: `gh run list`
  - Focus: newest informative workflow rows from run history tables
- `gh-run-view`
  - Matches: `gh run view`
  - Focus: workflow status, branch or event context, failed jobs, and failed steps

### JavaScript

- `bun-test`
  - Matches: `bun test`
  - Focus: failing suites, assertions, and file-level failure signal
- `js-package-test`
  - Matches: `npm test`, `pnpm test`, `yarn test`, plus `run test` variants
  - Focus: detects `vitest` or `jest` behind package scripts and forwards structured reporter flags
- `js-workspace`
  - Matches: non-test `npm`, `pnpm`, `yarn`, and `bun` commands plus direct workspace and toolchain entrypoints such as `turbo`, `nx`, `vite`, `eslint`, `tsc`, `tsx`, `ts-node`, `tsup`, `esbuild`, `rollup`, `webpack`, `next`, and `biome`
  - Focus: package-manager failures, workspace task failures, and file-anchored Vite or build diagnostics
- `vitest-json`
  - Matches: direct `vitest` commands
  - Focus: requests JSON reporter output and preserves failing suite details
- `jest-json`
  - Matches: direct `jest` commands
  - Focus: requests JSON output and condenses failing suite signal

### PHP

- `php-tooling`
  - Matches: `composer`, `phpunit`, `pest`, `phpstan`, `psalm`, `phpcs`, `phpcbf`, `php-cs-fixer`, and `php artisan` style entrypoints
  - Focus: dependency-resolution errors, failing assertions, static-analysis diagnostics, and PHP file anchors

### Python

- `pytest`
  - Matches: `pytest`, `python -m pytest`, `uv run pytest`
  - Focus: low-noise pytest output with failing test ids, assertion lines, and short summaries
- `python-tooling`
  - Matches: `uv`, `poetry`, `pip`, `pip3`, `ruff`, `mypy`, plus `python -m pip|ruff|mypy`
  - Focus: package resolution, lint, and type-check errors with file anchors and rule codes

### Build systems

- `build-system`
  - Matches: `make`, `just`, `task`, `bazel`, `ninja`, `cmake`, `terraform`, `tofu`, `helm`, `gradle`, `mvn`, `docker build`
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
- `kubectl-events`
  - Matches: `kubectl events`
  - Focus: warning-bearing event lines and failure reasons from event streams
- `kubectl-logs`
  - Matches: `kubectl logs`
  - Focus: bounded log tails with prefixed source identity and grouped repeated errors
- `kubectl-top`
  - Matches: `kubectl top`
  - Focus: compact previews of the newest resource metrics rows

## Expansion Read

The current builtin set is strongest in:

- local dev and test loops
- build and lint orchestration
- source control and PR inspection
- container and Kubernetes operational output

The main expansion surface is now deeper structured handling inside the covered ecosystems rather than missing first-pass command families. The next clean expansion path is more tool-specific machine-readable modes, richer domain grouping, and better reducers for commands that still share broad category profiles.
