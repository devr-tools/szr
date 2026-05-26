# Profiles

This document tracks the builtin profiles currently registered by `internal/profiles.Builtins()`.

Current coverage: `50` builtin profiles across `21` categories.

Anything not listed here does not currently have a dedicated builtin profile in `szr`.

Contributor guides for profile families:

- [Git profile family](/Users/alex/Documents/GitHub/szr/docs/profile-families/git.md)
- [JavaScript profile family](/Users/alex/Documents/GitHub/szr/docs/profile-families/javascript.md)
- [Generic fallback and declarative reducers](/Users/alex/Documents/GitHub/szr/docs/profile-families/fallback-reducers.md)

## Coverage Summary

| Category | Count | Covered commands and workflows |
| --- | ---: | --- |
| Filesystem | 2 | `ls`, `tree`, single-file `cat` previews |
| Go | 2 | `go test`, `go build`, `go vet` |
| Generic fallback | 2 | wrapped `test` commands, shallow `summary` commands |
| Search | 5 | `grep`, `rg` plain-text search, `rg --files`, `rg --files-with-matches`, `find` path discovery |
| Git | 4 | `git ls-files`, `git status`, `git log`, `git diff` |
| GitHub CLI | 5 | `gh pr view`, `gh pr checks`, `gh run list`, `gh run view`, GitHub Actions logs |
| JavaScript | 5 | `npm`, `pnpm`, `yarn`, `bun`, `turbo`, `nx`, `vite`, `vitest`, `jest`, `eslint`, `tsc` |
| PHP | 1 | `composer`, `phpunit`, `pest`, `phpstan`, `psalm`, `phpcs`, `phpcbf`, `php-cs-fixer`, `php artisan` |
| Python | 2 | `uv`, `poetry`, `pip`, `pytest`, `ruff`, `mypy` |
| Build systems | 1 | `make`, `just`, `task`, `bazel`, `ninja`, `cmake` |
| C/C++ tooling | 2 | `ctest`, `clang-tidy`, `clang-format`, `bear` |
| Patch and diff | 1 | `diff`, `patch`, `git apply` |
| Rust | 2 | `cargo test`, `cargo build`, `cargo clippy` |
| Containers | 2 | `docker ps`, `docker logs`, `docker compose ps`, `docker compose logs` |
| Cloud inventory | 2 | `aws`, `gcloud`, `az`, `doctl`, `oci`, `openstack`, `vercel`, `supabase`, `heroku` inventory and deployment listings |
| Cloud logs | 3 | cloud log streams plus scoped Supabase function logs and Heroku router logs |
| Kubernetes | 5 | `kubectl get`, `kubectl describe`, `kubectl logs`, `kubectl top`, `kubectl events` |
| HTTP API | 1 | `curl`, `http`, `httpie`, stdout-oriented `wget` API fetches |
| JSON query | 1 | `jq`, `yq`, JSON file reads, `python -m json.tool` |
| SQL | 1 | `psql`, `sqlite3`, `mysql`, `duckdb` query commands |
| Tabular | 1 | `ps`, `df`, `du`, `systemctl`, `helm list`, `kubectl get -o wide` |

## Profile Inventory

### Filesystem

- `directory-listing`
  - Matches: `ls`, `tree`
  - Focus: concise directory and file group previews with normalized output
- `cat-read`
  - Matches: single-file non-JSON `cat`
  - Focus: structural file previews that prefer headings, declarations, and anchored lines

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

- `grep`
  - Matches: recursive `grep` search with stable line-oriented output
  - Focus: grouped matches by file with conservative repo-noise suppression
- `ripgrep-files`
  - Matches: `rg --files`
  - Focus: bounded path previews for repo file listings
- `ripgrep-files-with-matches`
  - Matches: `rg --files-with-matches`, `rg -l`
  - Focus: bounded path previews for matching files only
- `ripgrep`
  - Matches: plain-text `rg` search commands outside JSON, files-only, and count-oriented modes
  - Focus: groups matches by file with stable filename and line-number output, applies conservative repo-noise excludes, and streams to early cutoff when enough visible context is collected
- `path-find`
  - Matches: plain `find` commands that emit path lists without `-exec`, `-delete`, or custom print formats
  - Focus: bounded path previews for file-discovery output with repo-noise suppression and streaming cutoff

### Git

- `git-ls-files`
  - Matches: `git ls-files`
  - Focus: bounded tracked-file previews for large file lists
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

### Cloud inventory

- `vercel-deployments`
  - Matches: `vercel list`, `vercel ls`
  - Focus: deployment target, ready state, URL, branch, creator, and metadata-aware Vercel deployment previews
- `cloud-list`
  - Matches: inventory-style `aws`, `gcloud`, `az`, `doctl`, `oci`, `openstack`, `vercel`, `supabase`, and `heroku` commands
  - Focus: resource identity, status, region or zone, framework or project context, and provider-specific nested metadata

### Cloud logs

- `supabase-function-logs`
  - Matches: `supabase functions logs`
  - Focus: scoped Edge Function failures with function slug, path, method, status code, and repeated invocation grouping
- `heroku-router-logs`
  - Matches: `heroku logs` when the user did not already scope by dyno or app-source noise
  - Focus: router-only request failures such as `H12`, status code, dyno, and request path signal
- `cloud-logs`
  - Matches: `aws logs`, `gcloud logging read`, `az monitor`, `oci logging-search`, `doctl apps logs`, `openstack console log show`, `vercel logs`, `supabase logs`, `heroku logs`
  - Focus: time range, source identity, and repeated failure signatures across provider log streams

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

### HTTP API

- `http-api`
  - Matches: API-oriented `curl`, `http`, `httpie`, and stdout-targeted `wget` fetches
  - Focus: response status, content type, and JSON body previews with selected scalar values

### JSON query

- `json-query`
  - Matches: `jq`, `yq`, JSON file reads, and `python -m json.tool`
  - Focus: JSON structure plus selected scalar previews instead of raw pretty-printed output

### SQL

- `sql-query`
  - Matches: non-interactive `psql`, `sqlite3`, `mysql`, and `duckdb` query commands
  - Focus: row summaries, row-count signal, JSON results, and common SQL errors

### Tabular

- `csv-tabular`
  - Matches: wide tabular CLI output such as `ps`, `df`, `du`, `systemctl`, `helm list`, and `kubectl get -o wide`
  - Focus: row-oriented key-field summaries for tables that are too wide to replay verbatim

## Expansion Read

The current builtin set is strongest in:

- local dev and test loops
- build and lint orchestration
- source control and PR inspection
- container and Kubernetes operational output

The main expansion surface is now deeper structured handling inside the covered ecosystems rather than missing first-pass command families. The next clean expansion path is more tool-specific machine-readable modes, richer domain grouping, and better reducers for commands that still share broad category profiles.
