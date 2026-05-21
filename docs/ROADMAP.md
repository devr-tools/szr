# szr Roadmap

`szr` should become the default execution layer between developer tools and AI systems. That means it has to do more than truncate output. It needs to be fast enough to sit in the hot path, smart enough to preserve what matters, and disciplined enough to cut token usage aggressively without making failures harder to diagnose.

The design bias remains the same: deterministic, parser-first, low-latency compression. `szr` should not spend model tokens in order to save model tokens.

## North-Star Targets

- Keep wrapper overhead under `10ms` p50 and `35ms` p95 for common commands.
- Reach `60%+` token savings across mixed usage and `80%+` on noisy workloads like tests, logs, and diffs.
- Push specialized-profile coverage above `75%` of commands seen in `szr spread`.
- Keep low-confidence fallbacks rare enough that `proxy` and tee files are exception paths, not routine escapes.
- Make every compression decision inspectable so users and agents can trust what was dropped.

## Product Principles

- Prefer structured output over raw text whenever the tool supports JSON, NDJSON, porcelain, stats, or machine-readable modes.
- Prefer streaming reducers over whole-buffer post-processing on large output.
- Preserve identifiers first: package names, test names, file paths, line numbers, symbols, exit codes.
- Optimize for next action, not for pretty summaries.
- Keep the default path local, deterministic, and cheap.

## Phase 1: Adaptive Token Economy

After the profile surface is broad enough, `szr` should get more selective about what it keeps. This is where it becomes meaningfully more advanced than a filter library.

### Goals

- Allocate token budget intentionally instead of using static line caps everywhere.
- Compress repetitive output more aggressively without losing root cause.
- Adapt to failure shape, verbosity mode, and user intent.

### Deliverables

- Replace static max-line heuristics with adaptive output budgets by command type, failure mode, and `--ultra-compact` level.
- Add repetition folding for logs, flaky retries, repeated warnings, and repeated compiler notes.
- Add stacktrace folding that keeps root cause, top frames, branch points, and unique file paths.
- Add salience ranking so reducers prioritize error-bearing lines, identifiers, and remediation hints above boilerplate.
- Add low-confidence escape hatches that automatically preserve more output when the reducer cannot confidently identify the actionable core.

### Boundary-pushing direction

- Use local command history to suggest tighter or looser budgets for commands that are consistently too noisy or too aggressively compressed.
- Add entropy-aware compaction so large repeated regions are collapsed faster than unique diagnostic regions.
- Introduce profile-level budget contracts such as "keep at least one failing test, one stack anchor, and one remediation hint."
- Add a reasoning-budget mode tuned for agent loops, where stability and token predictability matter more than human readability.

## Phase 2: Project-Aware And Agent-Native szr

The final phase is where `szr` stops being only a CLI wrapper and becomes infrastructure for AI-assisted development.

### Goals

- Make `szr` learn the repository it runs in.
- Make it easy for agents to rely on `szr` by default.
- Keep the system extensible without turning it into opaque magic.

### Deliverables

- Support project-local rules such as `.szr.json` or `.szr.yaml` for custom matchers, rewrites, reducers, and directory heuristics.
- Let teams define machine-readable flag preferences for internal CLIs and generated tooling.
- Add rule introspection so `szr explain` shows built-in and project-local decisions side by side.
- Add installer flows for Codex, Claude Code, Cursor, Gemini CLI, and plain shell environments.
- Add repo bootstrap that generates agent instructions describing when to use `szr`, `proxy`, `explain`, and tee artifacts.
- Add safer tee indexing and retrieval so full logs are easy to inspect only when needed.

### Boundary-pushing direction

- Recommend custom profiles automatically based on command history and poor-savings hotspots.
- Add repository-specific ignore intelligence for generated files, vendor trees, build output, and known noise sources.
- Make agent-targeted output modes stable enough that long-running agent loops can depend on them without prompt churn.
- Position `szr` as a programmable compression layer, not just a command alias.

## What Not To Do

- Do not add an LLM call into the default filtering path.
- Do not optimize token savings by dropping the identifiers needed to fix the problem.
- Do not overfit to synthetic fixtures while ignoring real command history.
- Do not let custom rules become a giant untestable router.
- Do not chase "advanced" features that add latency without measurable savings or fidelity wins.

## Suggested Build Order

1. Add adaptive budget allocation and stronger repetition folding.
2. Land project-local rules, agent installers, and repository-aware intelligence.

## Definition Of Success

`szr` succeeds when users and coding agents stop treating terminal output as raw text and start treating `szr` as the trusted, default compression layer between execution and reasoning. It should save tokens aggressively, preserve the next action reliably, and feel fast enough that nobody hesitates to keep it in the loop.
