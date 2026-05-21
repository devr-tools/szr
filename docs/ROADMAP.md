# szr Roadmap

`szr` should win on three axes at the same time:

- Fast enough to stay in the critical path of everyday terminal use.
- Smart enough to preserve the exact lines an agent or user needs next.
- Cheap enough to materially reduce LLM context spend without hiding failures.

This roadmap is biased toward deterministic, parser-first compression. The core rule is simple: `szr` should not spend model tokens in order to save model tokens.

## North-Star Targets

- Keep wrapper overhead under `10ms` p50 and `35ms` p95 for common commands where no heavy parsing is needed.
- Reach `60%+` token savings across mixed real-world usage and `80%+` on noisy commands like tests, logs, and diffs.
- Keep specialized-profile match coverage above `75%` for commands recorded in `szr gain`.
- Keep false-compaction risk low enough that users rarely need `proxy` or tee fallbacks.

## Product Principles

- Prefer structured output over raw text whenever the underlying tool supports JSON, NDJSON, porcelain, or stats modes.
- Prefer streaming reducers over full-buffer post-processing on large outputs.
- Prefer deterministic summarization over LLM summarization in the hot path.
- Preserve exit codes and failure visibility even when output is heavily compressed.
- Make compression decisions inspectable so users can trust the tool.

## Phase 1: Measurement First

Before expanding the command surface, `szr` needs better proof that a profile is both fast and safe.

### Deliverables

- Add a benchmark harness for profile latency and byte/token reduction.
- Add golden fixtures for representative outputs: clean pass, noisy fail, giant diff, repeated logs, and compiler errors.
- Extend `szr gain` with per-profile savings, p50/p95 duration, fallback rate, and failure rate.
- Add a `szr bench` command to compare profiles against fixtures.

### Why this matters

Without measurement, it is too easy to optimize for compression ratio while degrading fidelity or latency.

## Phase 2: Make The Hot Path Extremely Fast

The engine is already small and profile-driven. The next step is reducing avoidable work.

### Deliverables

- Move large-output reducers toward streaming scanners instead of buffering entire stdout and stderr before compaction.
- Add an early-bypass path for tiny outputs where compaction overhead is larger than the token savings.
- Introduce output budgets per profile so the renderer can stop collecting once enough signal has been captured.
- Separate `raw bytes read` from `bytes emitted` in metrics to expose expensive profiles.
- Add a profile capability for stderr-first tools so the engine avoids unnecessary merging work in common failure cases.

### Candidate engineering changes

- Add optional `RenderStream` or reducer interfaces alongside the current `Render` function.
- Replace generic line splitting in hot reducers with scanner-based state machines.
- Keep ANSI stripping and token estimation cheap and incremental.

## Phase 3: Expand High-Value Profiles

The biggest user savings will come from covering the tools that generate the most noise.

### Priority profiles

1. `pytest`, `python -m pytest`, and `uv run pytest`
2. `npm test`, `pnpm test`, `yarn test`, `vitest`, and `jest`
3. `cargo test`, `cargo build`, and `cargo clippy`
4. `docker`, `docker compose`, and container logs
5. `kubectl get`, `kubectl describe`, and `kubectl logs`
6. `gh pr view`, `gh run view`, and GitHub Actions output

### Profile strategy

- Force structured modes where possible.
- Fold repeated stacktrace frames and duplicate failure bodies.
- Collapse pass noise to package, suite, and failing-test summaries.
- Keep the first actionable remediation lines near the top.

## Phase 4: Advanced Token Reduction

Once the profile surface is broader, `szr` should compress more intelligently inside each profile.

### Deliverables

- Adaptive output budgets by command type, failure mode, and `--ultra-compact` level.
- Repetition folding for logs, warnings, and test retries.
- Path-aware grouping so changes and matches are clustered by file instead of raw line order.
- Stacktrace folding that preserves root cause, top frames, and unique branches.
- Semantic diff compaction that keeps churn totals, touched symbols, and file hotspots before raw hunks.
- Heuristics for "next action" extraction: missing import, failing test name, broken package, bad flag, missing file.

### Guardrails

- Never hide the only failing line.
- Preserve machine-actionable identifiers: package names, test names, file paths, line numbers, exit codes.
- Fall back to raw or tee output when confidence is low.

## Phase 5: Project-Aware Compression

The engine should become more useful inside real repositories without becoming magical or opaque.

### Deliverables

- Support project-local rules such as `.szr.json` or `.szr.yaml`.
- Allow custom profiles to declare matchers, rewrites, and reducers for repo-specific tools.
- Add include/exclude patterns for directories like `dist`, `node_modules`, generated code, or vendor trees.
- Let teams define preferred machine-readable flags for internal CLIs.
- Add rule introspection so `szr explain` shows user-defined rules alongside built-ins.

### Long-term direction

Use local history to recommend custom profiles for commands that repeatedly produce poor savings or excessive fallbacks.

## Phase 6: Agent-Native Workflows

`szr` becomes much more valuable when it is the default execution layer for coding agents.

### Deliverables

- Installer flows for Codex, Claude Code, Cursor, Gemini CLI, and plain shell aliases.
- Repo bootstrap that can generate agent instructions describing when to use `szr`, `proxy`, and `explain`.
- Safer tee artifact naming and lookup so agents can inspect full failure logs only when needed.
- A compact "reasoning budget" mode that emits short, stable summaries optimized for agent loops.

## What Not To Do

- Do not add an LLM call into the default filtering path.
- Do not overfit to synthetic benchmarks while ignoring real developer commands.
- Do not optimize token savings by dropping identifiers or making failures harder to diagnose.
- Do not add a giant monolithic router that makes profiles difficult to test or extend.

## Suggested Build Order

1. Add benchmark fixtures and richer `gain` analytics.
2. Add fast-path and streaming engine capabilities.
3. Ship the next six high-volume profiles.
4. Add adaptive compaction and repetition folding.
5. Add project-local rules and agent installers.

## Definition Of Success

`szr` is successful when users stop thinking of it as a novelty wrapper and start treating it as the default way to expose terminal output to an AI system. That requires trust, low latency, and measurable token savings, not just aggressive truncation.
