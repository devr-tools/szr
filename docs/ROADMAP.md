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

## Completed Foundations

The original Phase 1 and Phase 2 work is now baseline, not planned work.

### Landed From The Old Adaptive Token Economy Scope

- Adaptive budgets now vary by profile, confidence, verbosity, `--ultra-compact`, and reasoning-budget mode instead of relying on one static line cap.
- Repetition folding is built into the shared reducers so repeated warnings, retries, and noisy log regions collapse quickly.
- Stack and diagnostic anchoring is implemented across the failure reducers so root causes, unique file paths, and salient frames survive compaction.
- Low-confidence and failure escape paths preserve more output automatically when the reducer cannot confidently isolate the actionable core.
- Local history already feeds budget suggestions, and agent-oriented reasoning budgets now enforce minimum failure, anchor, and hint contracts.

### Landed From The Old Project-Aware And Agent-Native Scope

- Project-local `.szr.json`, `.szr.yaml`, and `.szr.yml` rules support custom profiles, rewrites, reducers, and cwd-aware matching.
- Project-local `preferences` provide machine-readable flag rewrites for internal CLIs and generated tooling.
- `szr explain` shows project-local and built-in decisions side by side, including applied preference rewrites.
- `szr install` bootstraps Codex, Claude Code, Cursor, Gemini, and plain shell environments with repo-local instructions and hook files.
- Tee artifacts are indexed and retrievable so full failure logs stay available without making raw output the default path.

## Active Roadmap

The next roadmap should focus on what is still not done, not on restating completed foundations.

### Phase 3: Coverage, Calibration, And Stability

#### Goals

- Expand structured coverage where commands still fall back to broad or generic reducers.
- Turn existing history, bench, and tee data into tighter quality feedback loops.
- Make agent-facing output modes predictable enough for long-running automated use.

#### Deliverables

- Recommend custom profiles automatically based on command history and poor-savings hotspots.
- Add repository-specific ignore intelligence for generated files, vendor trees, build output, and known noise sources.
- Tighten stability guarantees for agent-targeted output modes so long-running loops see less prompt churn.
- Grow bench coverage with more real-world failure fixtures and clearer fidelity regressions for reducers that over-compress.
- Surface fallback-heavy and low-savings hotspots more explicitly so profile tuning work is easy to prioritize.

### Phase 4: Programmable Compression Layer

#### Goals

- Push `szr` beyond command aliasing into repository infrastructure that teams can tune safely.
- Keep customizability inspectable, testable, and operationally cheap.

#### Deliverables

- Position `szr` as a programmable compression layer with stronger tooling around profile coverage, overrides, and rule evolution.
- Improve artifact lifecycle management so tee storage remains searchable, useful, and bounded over time.
- Keep latency and savings quality gates visible as the profile surface grows.

## What Not To Do

- Do not add an LLM call into the default filtering path.
- Do not optimize token savings by dropping the identifiers needed to fix the problem.
- Do not overfit to synthetic fixtures while ignoring real command history.
- Do not let custom rules become a giant untestable router.
- Do not chase "advanced" features that add latency without measurable savings or fidelity wins.

## Suggested Build Order

1. Expand structured reducers and hotspot-driven recommendations.
2. Harden agent stability, repository intelligence, and artifact operations.

## Definition Of Success

`szr` succeeds when users and coding agents stop treating terminal output as raw text and start treating `szr` as the trusted, default compression layer between execution and reasoning. It should save tokens aggressively, preserve the next action reliably, and feel fast enough that nobody hesitates to keep it in the loop.
