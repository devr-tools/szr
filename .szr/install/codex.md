# Codex installer

Instruction file: ./AGENTS.md
Hook script: ./.szr/hooks/pre-command.sh

Hook command:

```sh
./.szr/hooks/pre-command.sh "$@"
```

Suggested repo policy:

Use `szr` as the default entrypoint for noisy repository commands.

- Prefer `./bin/szr git status`, `./bin/szr git diff`, `./bin/szr git log`, and `./bin/szr go test ./...` over raw shell commands.
- Use `./bin/szr explain <cmd...>` when you need to inspect the active profile before bypassing it.
- Use `./bin/szr proxy <cmd...>` when raw output matters more than compression.
- If `szr` reports a tee artifact for a failure, inspect that full artifact path instead of rerunning the command unfiltered.
- The repository-local reminder hook lives at `./.szr/hooks/pre-command.sh`.

This installer is repo-local on purpose. Keep the generated instruction file committed, and wire the hook command into your codex shell/tool configuration separately.
