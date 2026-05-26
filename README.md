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

`szr` is a Go-native CLI proxy that reduces noisy terminal output before it reaches an LLM context.

It keeps the useful signal, preserves the wrapped command's exit code, and helps you spend fewer tokens on logs, diffs, and test output.

## What it does

- wraps the commands you already run
- rewrites supported tools into more compact machine-friendly output when possible
- summarizes noisy output without hiding the important anchors
- records local history so you can inspect token savings over time

## Install

Install the CLI with Go:

```bash
go install github.com/devr-tools/szr/cmd/szr@latest
szr self doctor
```

Or build from a local checkout:

```bash
make build
./bin/szr self install
szr self doctor
```

Homebrew install via tap:

```bash
brew install devr-tools/tap/szr
szr self doctor
```

## Main commands

Run your normal commands through `szr`:

```bash
szr git status
szr git diff
szr go test ./...
szr find . --name "*.py"
```

## AI tool support

AI bootstrap targets:

| Tool | Install command | Uninstall command | Description |
| --- | --- | --- | --- |
| Codex | `szr install codex` | `szr uninstall codex` | Writes `~/.codex/szr.md` (or `$CODEX_HOME/szr.md`) and patches the repo `AGENTS.md` to reference it. |
| Claude Code | `szr install claude-code` | `szr uninstall claude-code` | Installs `~/.claude/szr.md`, a Claude hook script, and a `settings.json` hook registration. |
| Cursor | `szr install cursor` | `szr uninstall cursor` | Installs `~/.cursor/hooks.json` plus a `preToolUse` hook script under `~/.cursor/hooks/`. |
| Gemini | `szr install gemini` | `szr uninstall gemini` | Installs `~/.gemini/settings.json` BeforeTool registration plus a hook script under `~/.gemini/hooks/`. |

## Integration surface

External agents, hooks, and plugins can call `szr rewrite --json` to reuse the same shell-routing policy that powers the built-in Claude, Cursor, and Gemini integrations.

Example:

```bash
szr rewrite --json --command 'git diff HEAD~1..HEAD --stat | tail -30'
```

Example response:

```json
{
  "command": "git diff HEAD~1..HEAD --stat | tail -30",
  "rewrite": "szr proxy git diff HEAD~1..HEAD --stat | tail -30",
  "hint": "szr git diff ... --stat or szr proxy git diff ... -- path/to/file | head -200",
  "reason": "wrap noisy producer inside shell pipeline",
  "auto_rewrite": true,
  "wrap_mode": "proxy",
  "producer_only": true,
  "already_routed": false
}
```

Use this surface when you want to:

- apply the same routing logic from custom Codex tooling or future plugins
- distinguish between safe auto-rewrites and hint-only guidance
- avoid re-encoding `git diff`, `grep`, `find`, and pipeline policy in multiple places

## Upcoming features

- stronger command recommendations based on real usage history and low-savings hotspots
- deeper repository-aware noise filtering for generated files, vendor trees, and build output
- more stable agent-facing output for long-running automated workflows
- broader reducer coverage and better fallback handling for noisy real-world commands

## Command reference

| Command | Description |
| --- | --- |
| `szr git status` | Run common Git commands through `szr` with reduced output. |
| `szr go test ./...` | Compress noisy test output while preserving failures and anchors. |
| `szr find <path> --name "*.py"` | Find files or directories with repo-noise suppression and a bounded match summary. |
| `szr grep <pattern> <path>` | Group search matches by file via ripgrep-backed summaries with conservative repo-noise excludes. |
| `szr run /usr/bin/grep ...` | Preserve exact grep semantics while still routing output through `szr`. |
| `szr rewrite --json --command '<cmd>'` | Return the shared shell-routing decision for external agents and integrations. |
| `szr spread` | Show token savings, usage patterns, and hotspot summaries. |
| `szr spread --history` | Inspect savings history across recent commands. |
| `szr doctor [--json]` | Check runtime diagnostics and local history health. |
| `szr self doctor [--json]` | Check install state, `PATH`, config, cache, and version details. |
| `szr settings` | Open the interactive settings menu for update checks, auto update, and other local preferences. |
| `szr tee --latest` | Inspect the latest preserved full-output artifact. |
| `szr explain go test ./...` | Show the matched profile, budget, and rewrite decisions for a command. |
| `szr commands` | Show the full command catalog for power users and agents. |
| `szr profiles` | List built-in reducer profiles. |

## Update notices

- Interactive shells: `szr` can print update notices on `stderr` when update checks are enabled.
- Agent or non-interactive tool runs: inline update notices are suppressed to keep tool output stable.
- Hosts can poll `szr doctor --json` or `szr self doctor --json` and render their own user-facing notification with the returned `update` object.
- Opt-in auto update is available for recognized Homebrew or `go install` installs:

```json
{
  "update_check": {
    "enabled": true,
    "interval_hours": 24,
    "auto_update": true
  }
}
```

## More docs

- [Contributing](CONTRIBUTING.md)
- [Local CI](docs/LOCAL_CI.md)
- [Releasing](docs/RELEASING.md)
- [Go package](docs/GO_PACKAGE.md)
- [Architecture](docs/ARCHITECTURE.md)

## Authors

@alxxjohn
