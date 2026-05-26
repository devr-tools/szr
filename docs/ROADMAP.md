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
