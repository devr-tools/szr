<p align="center">
  <img src="img/szr-gh.png" alt="szr logo" width="280" />
</p>

<p align="center">
  <a href="https://github.com/devr-tools/szr/releases">
    <img src="https://img.shields.io/github/v/release/devr-tools/szr?display_name=tag&include_prereleases" alt="release version" />
  </a>

  <a href="https://github.com/devr-tools/szr/actions/workflows/cd.yml">
    <img src="https://github.com/devr-tools/szr/actions/workflows/cd.yml/badge.svg?branch=main&event=push" alt="CD" />
  </a>

  <a href="https://github.com/devr-tools/szr/actions/workflows/ci.yml">
    <img src="https://github.com/devr-tools/szr/actions/workflows/ci.yml/badge.svg" alt="CI" />
  </a>

  <a href="https://github.com/devr-tools/szr/actions/workflows/homebrew-validation.yml">
    <img src="https://github.com/devr-tools/szr/actions/workflows/homebrew-validation.yml/badge.svg" alt="homebrew-validation" />
  </a>

  <a href="https://pkg.go.dev/github.com/devr-tools/szr/pkg/szr">
    <img src="https://pkg.go.dev/badge/github.com/devr-tools/szr/pkg/szr.svg" alt="Go Reference" />
  </a>

  <a href="https://goreportcard.com/report/github.com/devr-tools/szr">
    <img src="https://goreportcard.com/badge/github.com/devr-tools/szr" alt="Go Report Card" />
  </a>

  <a href="https://opensource.org/licenses/MIT">
    <img src="https://img.shields.io/badge/License-Apache-green.svg" alt="License: Apache 2.0" />
  </a>

  <a href="https://www.linkedin.com/in/alxjohn">
    <img src="https://img.shields.io/badge/LinkedIn-alxjohn-blue?logo=linkedin" alt="LinkedIn" />
  </a>
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

Homebrew is also wired up through this repo's tap:

```bash
# From a local checkout
brew tap devr-tools/szr "$(pwd)"
brew install szr
szr self doctor
```

```bash
# From GitHub
brew tap devr-tools/szr
brew install szr
szr self doctor
```

```bash
# One-line direct install from the tap
brew install devr-tools/szr/szr
szr self doctor
```

```bash
# Latest main branch instead of the stable tag
brew install --HEAD devr-tools/szr/szr
szr self doctor
```

The first stable formula release is `v0.1.0`. Future tagged releases are documented in [docs/RELEASING.md](/docs/RELEASING.md).

### Updating szr

`szr self update` upgrades installs that `szr` can recognize today:

- Homebrew installs use `brew upgrade szr`
- `go install` installs use `go install github.com/devr-tools/szr/cmd/szr@latest`

`szr self install` from a local build copies a binary into your user bin directory, so `szr` cannot safely infer how to refresh that binary later. In that case, rebuild and rerun `szr self install`.

If you want `szr` to periodically check for new stable releases and surface an upgrade notice, opt in through `~/.config/szr/config.json`:

```json
{
  "update_check": {
    "enabled": true,
    "interval_hours": 24
  }
}
```

When enabled:

- `szr doctor` and `szr self doctor` show the latest stable release and whether an update is available
- normal `szr` commands print a cached stderr notice when a newer release is available
- the last successful check is cached under `szr`'s data directory

### Repo bootstrap

Once the binary is globally available, bootstrap repo-local guidance separately:

```bash
szr install codex
szr install shell
szr uninstall codex
```

That keeps global binary installation separate from repo-specific agent/editor wiring.

## Usage

```bash
# Install szr from GitHub
go install github.com/devr-tools/szr/cmd/szr@latest
szr self doctor

# Bootstrap this repo for agent/shell use
szr install codex
szr install shell

# Run your usual commands through szr
szr git status
szr git diff
szr go test ./...

# Check token savings and command history
szr spread
szr spread --history

# Useful follow-ups
szr tee --latest
szr explain go test ./...
szr commands
```

## Local CI

There are now two local CI entrypoints:

- `make ci`: host-mode reproduction of the pipeline using your local Go toolchain, network, Docker, and installed binaries
- `make ci-docker`: pinned Linux container reproduction with Go, Semgrep, and `govulncheck` preinstalled

Use the containerized path when you want a more consistent result across machines:

```bash
make ci-docker
make ci-docker BASE_REF=develop
```

Before running `make ci-docker`, make sure the local Docker daemon is available because the target builds `Dockerfile.ci` and then runs `./scripts/ci.sh` inside that image.

`make ci-docker` is intentionally closer to the Ubuntu GitHub runner, but it still does not replace the hosted matrix entirely. GitHub CI remains the source of truth for:

- `macos-latest` behavior
- `windows-latest` behavior
- pull-request-only metadata and GitHub job summaries

## Deployment And Releases

Deployment for `szr` is release-driven:

- pushes to `main` or `master` let `release-please` manage the next stable release PR
- merges of that release PR create the stable tag and publish artifacts through `goreleaser`
- pushes to `develop` create prerelease tags like `v0.2.0-rc.5`

Before shipping, run one of the local CI paths and confirm the release-sensitive docs still match the repo:

- `README.md`
- `CONTRIBUTING.md`
- `docs/RELEASING.md`
- `Formula/szr.rb` for stable Homebrew releases

The full release and deployment checklist lives in [docs/RELEASING.md](/docs/RELEASING.md).

## Go Package

The public Go package for embedding `szr` is:

```go
import "github.com/devr-tools/szr/pkg/szr"
```

The `pkg.go.dev` pages are:

- library package: `https://pkg.go.dev/github.com/devr-tools/szr/pkg/szr`
- CLI command: `https://pkg.go.dev/github.com/devr-tools/szr/cmd/szr`

## Local Development

The test suite now lives under `test/`, with coverage enforced against `./internal/...`.
The public Go package lives under `pkg/szr`, while `cmd/szr-dev` is the developer-only launcher path.

```bash
make test
make cover
make smoke
make ci
make commit
make prepush
```

`make commit` stages the current worktree with `git add .`, asks you to choose one of three commit lanes, prompts for the summary text, then commits and pushes the current branch:

- `major` -> `feat!:` for breaking changes
- `minor` -> `feat:` for new functionality
- `patch` -> `fix:` for patch-level changes

If the push is rejected because the remote branch moved ahead, `make commit` offers to run `git pull --rebase origin <current-branch>` and retry the push.

## Release Versioning

Prereleases are produced from `develop` and non-`main`/`master` manual CD runs. The prerelease workflow computes the next semver bump from commit messages since the last stable tag:

- `BREAKING CHANGE` or `type!:` -> major
- `feat:` -> minor
- anything else -> patch

That produces release candidate tags like `v0.2.0-rc.123`.

Stable releases are produced from `main` or `master` through release-please, which opens or updates the release PR and creates the final stable tag when those changes land.

## Contributing

Public contributions are welcome. Start with [CONTRIBUTING.md](/CONTRIBUTING.md) for local setup, test expectations, commit hygiene, and PR etiquette.

For most code changes, the minimum bar is:

- keep the scope tight
- add or update tests with the behavior change
- run `make fmt` and `make test`
- explain the user-facing impact and verification steps in the PR
