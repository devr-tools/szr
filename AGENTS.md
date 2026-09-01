# Agent guide for szr

## What this repo is

`szr` is a token-aware CLI proxy: it sits in the execution path between
developer tools and LLMs, running a wrapped command and trimming its output
without dropping the parts an agent needs. It is a Go module with **zero
dependencies** — there is no `go.sum`, and that is deliberate.

## Hard rules

- **Do not add dependencies.** The module is stdlib-only. Adding one needs an
  explicit case, not a convenient import.
- **Keep reducers deterministic.** No network calls and no model dependencies
  on the filtering path. The same input must always produce the same output.
- **Preserve identifiers first.** File paths, package names, test names,
  symbols, exit codes, and anchor lines survive reduction; prose is what gets
  trimmed.
- **Latency is a feature.** szr runs on every wrapped command, so allocation
  and blocking-I/O regressions are felt directly by the user.
- **Tests live in `test/`,** mirroring the package tree, not beside the code.

## Build and verify

The local Go toolchain may have a stale `GOROOT`, so Go invocations need
`env -u GOROOT`. The Makefile handles this via `GO_ENV`; prefer make targets
over raw `go` commands.

- `make fmt` gofmts the project.
- `make build` builds `bin/szr` and `bin/szr-dev`.
- `make test` runs the centralized suite under `test/`.
- `make cover` runs it with the internal coverage gate.
- `make smoke` runs quick CLI smoke checks, including bench and install flows.
- `make prepush` is the local gate: fmt, test, cover, smoke.
- `make ci` reproduces the GitHub CI pipeline on the host.
- `make ci-docker` runs that pipeline inside the pinned Linux CI container.
- `make help` lists every target.

The internal coverage gate is 80% (`MIN_INTERNAL_COVERAGE`). CI also enforces
a test-presence check: a change under `cmd/`, `internal/`, or `pkg/` must come
with a change under `test/` or a `_test.go` file.

## Project layout

- `cmd/szr`: end-user CLI entrypoint
- `cmd/szr-dev`: developer-only helper entrypoint
- `internal/cli`: command routing and help text
- `internal/engine`: execution, matching, rendering, and streaming behavior
- `internal/profiles`: tool-specific reducers and command rewrites
- `internal/filters`: shared summarizers and reducers
- `test/`: centralized tests for CLI behavior, reducers, profiles, config, and
  install flows

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and
[CONTRIBUTING.md](CONTRIBUTING.md).

## Lint budget

`.golangci.yml` enables only complexity linters: `funlen` (120 lines / 60
statements), `gocognit` (min-complexity 15), `maintidx` (under 50). codeguard
additionally caps files at 400 lines. Split a file rather than fighting these.

CI runs golangci-lint with `--new-from-rev`, so the gate is on *new* issues
relative to the PR base, not the absolute count.

## Commits

Conventional commits — release automation depends on them. PRs targeting
`develop` need a `Signed-off-by:` trailer for the DCO check.
