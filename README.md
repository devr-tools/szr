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

## Why szr

**Fidelity is a runtime guarantee, not a promise.** A built-in retention verifier checks every render against the raw output — error lines, `file:line` anchors, diagnostic codes, failing test names — and repairs anything a filter dropped by appending the missing detail. Failing commands can never render content-free, explicit flags you pass are never overridden, and exit codes always match the wrapped command.

**Re-runs cost almost nothing.** Agents run `git status`, `git diff`, and test suites over and over. When output is byte-identical to a recent run, szr emits a two-line reference instead (`unchanged from previous run (39s ago, x3 identical) [ref: …]`) — and `szr expand <ref>` recovers the original byte-exact, on demand. The store is machine-level, so concurrent agents share it automatically: what one agent already saw, another can reference.

- **Edit-test loops render as deltas.** When a rerun's output *changed* instead of repeating, szr can emit a compact change digest — `since last run (2m ago): +3 -1 lines` plus the changed lines themselves — but only when that is strictly cheaper than the normal summary, and never at the cost of a critical line: a newly-failing test always appears in the digest. The baseline stays one `szr expand <ref>` away.
- **Agent fleets get their own scope.** Export `SZR_SESSION=<id>` before launching parallel agents and the whole fleet shares one dedup/delta scope: what one agent rendered, its siblings reference. Runs without the variable stay in the machine scope and never cross-match scoped sessions; `szr expand` resolves refs from any scope.

**Anomalies are the payload.** List and table summaries always keep the rows that differ — the one stopped instance among 800 running, the `past_due` row in a result set — plus exact counts. Uniform JSON arrays render as a compact table that halves the token cost of list-shaped API responses at equal information.

**Everything is recoverable.** Whatever a summary omits is preserved in a local artifact with an exact pointer — compression never costs you the ability to look at the full output.

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

- stdin pipe mode for filtering output already in flight
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
| `szr expand <ref>` | Recover the byte-exact original output behind a dedup or delta baseline reference. |
| `SZR_SESSION=<id> szr <cmd...>` | Scope dedup and delta references to one agent session; export it fleet-wide so parallel agents share a scope. |
| `szr tee --latest` | Inspect the latest preserved full-output artifact. |
| `szr explain go test ./...` | Show the matched profile, budget, and rewrite decisions for a command. |
| `szr commands` | Show the full command catalog for power users and agents. |
| `szr profiles` | List built-in reducer profiles. |

Reasoning budget modes:

- `standard`: balanced for human readability
- `agent`: tighter defaults for agent loops
- `aggressive`: smallest previews for spread-heavy workflows, including terser `git diff` summaries

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
