# Contributing to szr

`szr` is a CLI that sits in the execution path between developer tools and LLMs. Changes should preserve that bias: low latency, deterministic behavior, and output that is smaller without becoming less actionable.

## Before you start

- Check existing issues and docs first so work does not duplicate an active thread.
- Prefer small, reviewable pull requests over large multi-theme batches.
- Follow the expectations in [CODE_OF_CONDUCT.md](/Users/alex/Documents/GitHub/szr/CODE_OF_CONDUCT.md) in issues, reviews, and PR discussion.
- If the change affects command behavior, update tests in `test/` with the code change.
- If the change affects installation, releases, or contributor workflow, update the relevant docs in `README.md` or `docs/`.

## Local setup

```bash
go install ./cmd/szr
make test
make smoke
```

Common checks before opening a PR:

```bash
make fmt
make test
make cover
```

Use `make prepush` when you want the full local gate.

## Project layout

- `cmd/szr`: end-user CLI entrypoint
- `cmd/szr-dev`: developer-only helper entrypoint
- `internal/cli`: command routing and help text
- `internal/engine`: execution, matching, rendering, and streaming behavior
- `internal/profiles`: tool-specific reducers and command rewrites
- `internal/filters`: shared summarizers and reducers
- `test/`: centralized tests for CLI behavior, reducers, profiles, config, and install flows

## Coding expectations

- Keep reducers deterministic. Do not add network calls or model dependencies to the filtering path.
- Preserve identifiers first: file paths, package names, test names, symbols, exit codes, and anchor lines.
- Prefer structured output modes such as JSON, porcelain, or machine-readable flags when the wrapped tool supports them.
- Remove dead helpers and stale scaffolding when you touch an area. Do not leave unfinished placeholder text in user-facing output or docs.
- Keep comments short and only where they clarify non-obvious logic.

## Tests and verification

- Add or update focused tests for behavior changes.
- Keep snapshots or assertions resilient to irrelevant formatting churn.
- Mention the commands you ran in the PR description.
- If you could not run a check locally, say that explicitly and explain why.

## Commit guidance

- Use focused commits with clear subjects.
- Prefer conventional-commit style prefixes when practical because release automation depends on readable history.
- Do not mix refactors, feature work, and doc-only cleanup into one commit unless they are tightly coupled.
- If your PR targets `develop`, include a `Signed-off-by:` trailer to satisfy the DCO workflow.

Example:

```text
fix: restore git log summary header

Signed-off-by: Your Name <you@example.com>
```

## Pull request etiquette

- Explain the problem first, then the fix.
- Call out any user-visible behavior changes, migration notes, or follow-up work.
- Include test coverage or a short manual verification note.
- Keep review threads resolved with code or clear reasoning, not by silence.
- Avoid force-pushing away review context after feedback has started unless the branch genuinely needs to be rebased or cleaned up.

## Scope for good first PRs

- reduce noisy CLI output without dropping actionable anchors
- expand reducer coverage for common developer tools
- improve docs, examples, install guidance, or release automation
- add tests for edge cases in filters, profiles, and routing
