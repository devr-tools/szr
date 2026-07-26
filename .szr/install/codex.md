# Codex installer

Global instruction file: ~/.codex/AGENTS.md
Repository instruction file: ./AGENTS.md

Suggested repo policy:

Use `szr` as the default entrypoint for noisy repository commands.

- Use `./bin/szr <command...>` by default for normal agent inspection so szr can reduce noisy output.
- Always use `./bin/szr git ...` for Git operations, including managing branches and worktrees, checking status and diffs, staging, committing, pulling, and pushing.
- `./bin/szr proxy <command...>` is the raw-output escape hatch. Use it only when raw output or exact formatting matters, including when a downstream pipeline consumes that format.
- `./bin/szr expand <ref>` is recovery, not execution. Use it only after szr returns a dedup or delta reference; it reads the stored byte-exact output without rerunning the command.
- Prefer `./bin/szr git status`, `./bin/szr git diff`, `./bin/szr git log`, and `./bin/szr go test ./...` over raw shell commands.
- Codex does not install a Bash rewrite hook today, so tool calls must invoke `./bin/szr` explicitly.
- When using pipes, redirection, or absolute-path binaries, keep `./bin/szr` on the noisy command itself, for example `./bin/szr proxy git diff --stat HEAD~1..HEAD 2>&1 | head -200`.
- For git diff review loops, prefer `./bin/szr git diff ... --stat` and `./bin/szr proxy git diff ... -- path/to/file | tail -80` instead of raw piped `git diff` calls.
- Prefer `./bin/szr find <path> --name "*.py"` for file discovery, and `./bin/szr grep <pattern> <path>` or `./bin/szr rg <pattern> <path>` for grouped code search.
- If exact `/usr/bin/find` or `/usr/bin/grep` flags matter, wrap them explicitly with `./bin/szr run /usr/bin/find ...` or `./bin/szr run /usr/bin/grep ...`.
- Use `./bin/szr explain <cmd...>` when you need to inspect the active profile before bypassing it.
- A `since last run (...): +N -M lines` digest shows only what changed since the previous run; use `./bin/szr expand <id>` on the baseline ref if you need the full previous output. Orchestrators export `SZR_SESSION=<id>` so parallel agents share references.
- A `missing detail:` section contains critical lines szr's retention verifier recovered from raw output; treat it as part of the command output.
- For long agent loops, prefer the `agent` reasoning budget mode (`./bin/szr settings`) for tighter default budgets.
- If `szr` reports a tee artifact for a failure, inspect that full artifact path instead of rerunning the command unfiltered.

Use `szr install codex --global` to merge this guidance into `$CODEX_HOME/AGENTS.md` for every repository. Use `szr install codex` for the nearest Git root, or `szr install codex --root <path>` for one explicit repository.
