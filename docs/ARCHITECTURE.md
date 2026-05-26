# szr Architecture

`szr` is a Go-native command proxy for reducing terminal output before it reaches an LLM context.

## Design Goals

- Small, single-binary runtime.
- Explicit profile registry instead of a giant monolithic command router.
- Safe failure behavior: keep the wrapped command's exit code.
- Local-only analytics with no network dependency.
- Easy addition of new command profiles and filter stages.
- Inspectable compression decisions so contributors can reason about what was rewritten, reduced, or hidden.

## Current Structure

```text
pkg/szr
cmd/szr
cmd/szr-dev
internal/bench
internal/cli
internal/config
internal/engine
internal/filters
internal/history
internal/installers
internal/profiles
internal/rules
internal/szrdev
```

## Refactor Direction

`szr` keeps the runtime `engine.Profile` model and project-local profile overlays. The refactor does not replace profiles with a command enum or a separate CLI tree. Instead, it tightens the boundaries around how profiles are selected and how common behavior is shared.

The target split is:

1. Classification: normalize the invocation and derive reusable command facts once.
2. Profile selection: choose the best builtin or project-local profile from the registry.
3. Prepare/rewrite: apply machine-readable flags or noise-reduction rewrites only after a profile is chosen.
4. Render/reduce: run the command and compress output via structured reducers, streaming reducers, or declarative fallback reducers.
5. Recovery: guarantee deterministic access to hidden output when truncation or parse fallback occurs.

## Execution Flow

### Current flow

1. `cmd/szr/main.go` calls the public `pkg/szr` package.
2. The CLI parses global flags and dispatches a subcommand.
3. External commands are converted into an `engine.Invocation`.
4. The profile registry matches the invocation.
5. The engine optionally rewrites the command for a better machine-readable form.
6. The command runs and the selected renderer compresses the output.
7. A local history record is appended for `szr spread`.
8. On failures, raw output can be tee'd into the local data directory and indexed for later retrieval.
9. Installer and benchmark commands reuse dedicated internal packages instead of embedding that logic in the core router.

### Target flow

1. The CLI builds an `engine.Invocation`.
2. A classification layer derives normalized command metadata from the invocation.
3. The engine uses that classification plus project-local overlays to select a profile.
4. The chosen profile decides whether to prepare or rewrite the command based on explicit capabilities and requirements.
5. The engine executes the command and routes output through:
   - a structured reducer for parser-first profiles
   - a streaming reducer for long-running or line-oriented output
   - a declarative fallback reducer for simple line-based compaction
6. Shared truncation and recovery policy determines whether a tee file, continuation hint, or fallback escape should be emitted.
7. The engine records history and exit status without hiding the wrapped command's outcome.

## Layer Responsibilities

### Classification

Classification answers "what command is this?" without deciding "how should it be reduced?".

This layer owns:

- normalized command identity
- command family and subcommand extraction
- wrapper and toolchain normalization
- machine-readable mode detection
- rewrite exemptions and safety guards
- structured-output eligibility

This layer does not own:

- choosing a profile
- injecting flags
- rendering output
- truncation policy

The practical goal is to compute these facts once per invocation instead of repeating them in multiple `Match` and `Prepare` functions.

### Profile Selection

Profile selection answers "which reducer path should handle this invocation?".

This layer owns:

- builtin profile registration
- project-local profile overlays
- profile ordering and precedence
- profile confidence
- matching against invocation plus classification facts

This layer does not own:

- ad hoc command normalization
- repeated flag-safety checks that belong in classification
- shared recovery policy

Profiles remain the core extension unit in `szr`. A contributor should still add a new builtin reducer by adding a profile, not by editing a giant command enum.

### Prepare And Rewrite

Prepare/rewrite answers "what command should actually run?" once a profile is selected.

This layer owns:

- appending machine-readable flags
- adding noise-suppression flags
- preserving user intent when the user already selected an explicit format
- honoring aggressive rewrite modes only when the profile declares support

Prepare logic should rely on classification and profile capabilities rather than re-parsing the raw command repeatedly.

### Render And Reduce

Render/reduce answers "how should the captured output be compressed?".

This layer owns:

- structured reducers for JSON, NDJSON, porcelain, stats, or domain-specific text
- streaming reducers for line-oriented output
- shared builder helpers for common reducer wiring
- declarative fallback reducers for low-complexity line filtering

This layer does not own:

- command-family detection
- profile precedence
- tee/recovery policy rules beyond calling the shared helpers

### Recovery

Recovery answers "how can the user or agent retrieve hidden output deterministically?".

This layer owns:

- tee file creation policy
- continuation hints for truncated output
- parse-failure fallback escape behavior
- shared truncation policy and thresholds

Every reducer that hides meaningful content must have a predictable recovery path.

## What Stays Profile-Driven

These concerns should remain profile-defined:

- domain-specific matching once classification facts exist
- command preparation that depends on tool semantics
- reducer selection
- domain-specific explanation text
- structured parsing behavior

Examples:

- `git diff` deciding between stat-oriented and more aggressive stat-oriented rewrites
- `go test` forcing `-json`
- package-manager test wrappers detecting `vitest` or `jest`

## What Moves Into Shared Infrastructure

These concerns should move out of individual profiles when practical:

- command normalization repeated across families
- machine-readable mode detection helpers
- rewrite safety guards and exemptions
- repeated buffered reducer wiring
- repeated budget and latency helper patterns
- truncation and tee recovery rules
- declarative line-based fallback reduction

Profiles should describe behavior, not reimplement framework logic.

## Glossary

### Command classification

Normalized metadata about an invocation, derived before profile selection. This includes the effective command family, subcommand, wrapper normalization, known exemptions, and whether structured output is already requested.

### Profile capability

Explicit metadata on a profile describing what it can or requires, such as whether it injects flags, expects structured stdout, supports aggressive rewrites, or allows a fallback escape after parse failure.

### Structured mode

Any machine-readable output mode that is materially easier to reduce than free-form text, such as JSON, NDJSON, porcelain, `--short`, `--stat`, or tool-specific structured reporters.

### Declarative fallback reducer

A low-complexity reducer defined by configuration or data rather than Go code. These reducers are limited to line-oriented operations such as keep, strip, head, tail, truncate, and empty-result messaging.

### Truncation recovery

A deterministic mechanism for reaching hidden output after a reducer truncates or compacts it. Recovery may be provided through a tee file, a tail-style continuation hint, or another shared recovery helper.

## Builtin Profile Strategy

The builtin profile strategy remains parser-first:

- `git status`: normalize to `--short --branch`, then count staged, unstaged, and untracked files.
- `git log`: prefer `--oneline -n 20` and keep a short commit preview.
- `git diff`: prefer `--stat` and summarize file churn.
- `go test`: force `-json` and collapse pass noise into package and failure summaries.
- `npm test` / `pnpm test` / `yarn test`: inspect `package.json`, detect `jest` or `vitest`, and forward structured reporter flags.
- `vitest` / `jest`: prefer JSON-capable output and summarize failing suites, assertions, and file paths.
- `go build` / `go vet`: bias toward compiler diagnostics.
- Generic test and summary wrappers: provide fallback compaction for unstructured tools.

The main expansion surface is not replacing profiles. It is making profile internals more regular, safer, and cheaper to extend.

## Migration Guide

When touching an existing profile family during the refactor:

1. Move shared command facts into classification helpers first.
2. Add explicit profile capabilities before adding more engine-side branching.
3. Replace repeated setup code with shared builders only after the repeated pattern is clear.
4. Use declarative fallback reducers only for genuinely line-based logic.
5. Add shared recovery hints whenever a reducer truncates useful content.

Do not:

- replace structured reducers with declarative rules
- introduce a giant command enum as the primary extension model
- duplicate classification logic in both `Match` and `Prepare`
- add truncation without a recovery path

## Related Docs

- [ROADMAP.md](/Users/alex/Documents/GitHub/szr/docs/ROADMAP.md)
- [PROFILES.MD](/Users/alex/Documents/GitHub/szr/docs/PROFILES.MD)
- [PROFILE_REFACTOR_TASKS.md](/Users/alex/Documents/GitHub/szr/docs/PROFILE_REFACTOR_TASKS.md)
- [profile-families/git.md](/Users/alex/Documents/GitHub/szr/docs/profile-families/git.md)
- [profile-families/javascript.md](/Users/alex/Documents/GitHub/szr/docs/profile-families/javascript.md)
- [profile-families/fallback-reducers.md](/Users/alex/Documents/GitHub/szr/docs/profile-families/fallback-reducers.md)
