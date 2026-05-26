# Git Profile Family

Use the `git` family for source-control commands where `szr` can safely request a more compact machine-readable or summary-oriented mode before reduction.

## What belongs here

- `git status`
- `git log`
- `git diff`
- `git ls-files`
- future `git` subcommands with stable, repo-local semantics and a reducer that is specific to Git output rather than generic path-list or line filtering

Do not add non-Git wrappers here. If the command is fundamentally a path list or generic text reducer, prefer the shared family that already owns that output shape.

## Common prepare rules

- Reuse invocation classification instead of re-parsing raw args in each profile.
- Only inject flags when the user did not already request an explicit format.
- Prefer summary-oriented Git modes over raw patch output when the reducer is stat or list based.
- Aggressive rewrites may add more noise-suppression flags, but they must preserve exit code and command intent.

Current examples:

- `git status` adds `--short --branch` unless status format flags are already present.
- `git log` adds `--oneline -n 20` unless the user already requested a log format.
- `git diff` prefers `--stat`-style output and adds `--no-color` or `--no-ext-diff` only when safe.

## Structured-output expectations

- `git` profiles generally use `StructuredModePreferred`, not hard-required structured stdout.
- Prefer Git-native concise formats such as `--short`, `--branch`, `--oneline`, and `--stat`.
- Leave the command alone when the user already chose a machine-readable or no-patch mode that the reducer can consume directly.

If the reducer depends on full patch semantics or custom formatting, keep it in Go code and document the exemption clearly.

## Test strategy

- Add profile tests for `Match` and `Prepare` behavior.
- Cover exemption paths where user-supplied format flags should block rewrites.
- Add reducer tests for truncation, recovery hints, and high-churn edge cases.
- Prefer focused command fixtures over broad repo-state simulations.

Good targets:

- status format already requested
- log format already requested
- diff aggressive mode versus normal mode
- large path lists and large diff summaries

## Code vs declarative fallback

Keep Git reducers in Go when they need:

- subcommand-aware prepare logic
- stateful parsing
- churn aggregation
- path grouping
- recovery behavior beyond simple line omission

Use declarative fallback only for truly line-based generic reduction around unmatched Git-adjacent output. Do not replace `git status`, `git log`, or `git diff` reducers with declarative rules.
