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
  C --> Z["szr<br/>filter output"]
  Z --> L["LLM<br/>lower tokens"]

  classDef base fill:#F3F4F6,stroke:#9CA3AF,color:#374151;
  classDef blue fill:#31A9F3,stroke:#31A9F3,color:#ffffff;
  class C,W base;
  class Z,L blue;
  linkStyle 0 stroke:#9CA3AF,stroke-width:2px;
  linkStyle 1,2 stroke:#31A9F3,stroke-width:2px;
```

## Install

There are now two separate install layers:

- global install: make the `szr` binary available on your shell `PATH`
- repo bootstrap: teach a specific repo and agent environment to prefer `szr`

### Global install

From a local checkout, install the binary into your Go bin directory:

```bash
go install ./cmd/szr
szr self doctor
```

Or build locally and let `szr` install itself into `~/.local/bin` or `~/bin`:

```bash
make build
./bin/szr self install
./bin/szr self doctor
```

If your shell does not already include that install directory on `PATH`, `szr self install` prints the exact line to add to `~/.zshrc`, `~/.bashrc`, or the detected shell rc file. To let `szr` append that line for you:

```bash
./bin/szr self install --update-shell
```

### Repo bootstrap

Once the binary is globally available, bootstrap repo-local guidance separately:

```bash
szr install codex
szr install shell
```

That keeps global binary installation separate from repo-specific agent/editor wiring.

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
szr self install
szr self doctor
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
