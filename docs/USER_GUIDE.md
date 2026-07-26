# szr user guide

This guide covers the features that are useful once you are comfortable running commands through `szr`.

## Everyday use

Run `szr` before a command you already use. It chooses a profile for supported tools, preserves the command's exit code, and keeps useful error lines and file locations. If a compact result would be larger or less useful than the original output, szr prints the original output.

```bash
szr git status
szr git diff
szr go test ./...
szr grep "TODO" .
```

You can also use common shell wrappers. For example, `szr env -u GOROOT go test ./...` is handled like `szr go test ./...`.

## AI tool integrations

Install an integration with `szr install <tool>` and remove it with `szr uninstall <tool>`.

| Tool | Install command | What it sets up |
| --- | --- | --- |
| Codex | `szr install codex --global` | Global szr guidance in `$CODEX_HOME/AGENTS.md` (normally `~/.codex/AGENTS.md`) for every repository. |
| Claude Code | `szr install claude-code` | A Claude instruction file and hook registration. |
| Cursor | `szr install cursor` | A Cursor pre-tool hook. |
| Gemini | `szr install gemini` | A Gemini before-tool hook. |

Codex also supports repository scopes:

```bash
szr install codex               # nearest Git root, or the current directory outside Git
szr install codex --root <path> # one explicit repository
szr uninstall codex --global    # remove only the global szr guidance
```

Global installation is recommended when you use Codex across several repositories. Repository-level installs remain useful when only selected projects should use szr or need local overrides.

## See your savings

`szr spread` summarizes recent output and token savings. Add `--history` for recent commands, or `--cost` to include an estimate based on an input-token price.

```bash
szr spread
szr spread --history
szr spread --cost --rate 3.00
```

`szr discover` can scan local AI-agent transcripts, without changing or uploading them, to estimate savings from commands that did not use szr. `szr usage` compares recorded agent token use with szr's estimates.

```bash
szr discover
szr usage
```

## Recover complete output

szr keeps local references to output it has compacted. Use `szr expand <ref>` to retrieve the original output.

```bash
szr expand abc123
```

Use `szr tee --latest` to inspect the most recent saved artifact. Full-output retention is bounded by default; adjust it in `szr settings` if needed.

## For integrations and advanced workflows

- `szr rewrite --json --command '<command>'` returns szr's routing decision for custom tools and hooks.
- `szr watch --jsonl` streams sanitized local execution events for integrations. Add `--once` for one snapshot.
- `szr diagnostics status` checks local diagnostics storage. Diagnostics export is disabled by default; when enabled, it uses a bounded local outbox and makes one final bounded delivery attempt on command exit. Export failures never change the wrapped command's result.
- `SZR_SESSION=<id> szr <command>` shares deduplication and delta history across agents in one session.

See [Integrations](INTEGRATIONS.md) for the stable event contract and routing details.

## Full command catalog

Run `szr commands` for the complete command list on the version installed on your machine. Useful commands include:

| Command | Purpose |
| --- | --- |
| `szr profiles` | List the built-in and user-defined profiles. |
| `szr doctor --json` | Return runtime diagnostics in JSON. |
| `szr self doctor --refresh` | Check the installation and refresh the release check. |
| `szr settings` | Change local preferences interactively. |
| `szr diagnostics purge --yes` | Explicitly remove local diagnostics data. |
