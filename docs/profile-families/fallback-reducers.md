# Generic Fallback And Declarative Reducers

Use declarative reducers for simple line-oriented fallback paths. They are intentionally narrow and should complement Go profiles, not replace structured reducers.

## What belongs here

- generic compact output for unmatched commands
- read-only line filtering for simple file previews
- failure-biased line extraction where regex and truncation are enough
- small reducers that only need keep or strip patterns, head or tail limits, line clipping, and an empty-result message

Do not put command-specific prepare logic, stateful parsing, or structured report handling in this layer.

## Common prepare rules

- Declarative reducers do not own command rewrites.
- The engine or matched profile decides the command to run first.
- Fallback selection should happen after classification and profile selection, not instead of them.

If a reducer needs command-family awareness to work correctly, it likely belongs in a Go profile.

## Structured-output expectations

- Declarative reducers are text-first and line-oriented.
- They should consume already-captured output, not request or depend on a protocol such as JSON.
- Omission metadata should flow through the shared recovery contract so hidden output can be retrieved deterministically.

Prefer a Go profile when the tool exposes a structured mode that materially improves fidelity.

## Test strategy

- Validate reducer specs and builtin loading.
- Add focused tests for keep or strip behavior, head or tail limits, clipping, and empty-result handling.
- Verify omission metadata and recovery integration.
- Add engine tests when fallback selection changes user-visible behavior.

Good targets:

- failure-focused extraction on nonzero exit
- compact line fallback on success
- empty output produces the configured fallback message

## Current adoption

The current declarative builtin surface is intentionally small:

- `compact_lines`
- `interesting_error_lines`
- `read_minimal`

These are used in three ways:

- generic helper calls such as compact-line fallback
- engine fallback for unmatched commands
- explicit profile-level reducers for true line-only summaries

Current explicit profile-level adopters:

- `generic-summary`
- `gh-run-list`
- `kubectl-top`

At the moment, that profile-level set is effectively the boundary for obvious line-only reducers in the builtin profile families.

## Code vs declarative fallback

Use declarative fallback when all of the following are true:

- the output is line-oriented
- regex and truncation rules are enough
- there is no command-specific prepare logic
- no stateful parser is required

Use Go code when any of the following are true:

- the reducer needs structured parsing
- the reducer groups or aggregates records
- the reducer needs custom recovery semantics
- the reducer depends on command-family semantics
- the reducer mutates command args or relies on profile capabilities

Examples that should stay in Go in this repo:

- failure compaction reducers such as `generic-test`, `go-build`, and `gh-pr-checks`
- structured reducers such as `go-test-json`, `gh-pr-view`, `gh-run-view`, `json-query`, `sql-query`, and `http-api`
- grouped log or event reducers such as `gh-run-log`, `kubectl-logs`, `kubectl-events`, and `cloud-logs`
- domain-aware inventory summaries such as `kubectl-get`, `docker ps`, `cloud-list`, `tabular`, directory listings, and tree previews

If a proposed migration needs command awareness to justify why one line matters more than another, it is not a declarative reducer candidate.
