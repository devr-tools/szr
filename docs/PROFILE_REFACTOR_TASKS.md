# Profile Refactor Task List

This document breaks the `profiles` refactor into agent-sized tasks. The goal is to keep `szr`'s runtime profile registry while importing the stronger architectural discipline that tools like RTK apply around rewrite isolation, fallback layers, and recovery contracts.

## Goals

- Keep the runtime `engine.Profile` model and project-local profile overlays.
- Separate command classification from profile matching so matching, rewrites, and exemptions are easier to reason about.
- Add a declarative fallback path for simple reducers without replacing Go-based structured profiles.
- Centralize truncation and recovery behavior so hidden output is always recoverable.
- Reduce repeated profile boilerplate by introducing reusable builders.
- Make profile capabilities explicit so rewrite safety and structured-mode requirements are inspectable instead of implicit.

## Working Rules

- Keep PRs single-theme. Do not mix registry changes, fallback DSL work, and doc cleanup in one branch.
- Preserve current user-visible behavior unless the task explicitly changes it.
- Add or update tests with every behavior change.
- If a task introduces a new abstraction, migrate one or two representative profiles first before broad rollout.
- Do not convert structured reducers to declarative rules unless the reducer is genuinely line-based.

## Phase Order

1. Define contracts and vocabulary.
2. Extract rewrite classification from profile matching.
3. Introduce explicit profile capabilities.
4. Add reusable builders for common profile wiring.
5. Add declarative fallback reducers.
6. Centralize truncation and recovery policy.
7. Write contributor docs by ecosystem and extension path.
8. Migrate profiles incrementally.

## Task 1: Architecture Contracts

**Objective**

Define the target architecture before moving code.

**Scope**

- Update [docs/ARCHITECTURE.md](/Users/alex/Documents/GitHub/szr/docs/ARCHITECTURE.md:1) with the new separation:
  - classification
  - profile selection
  - prepare/rewrite
  - render/reduce
  - recovery
- Define terms for:
  - command classification
  - profile capabilities
  - structured mode
  - declarative fallback reducer
  - truncation recovery
- Add a short migration section describing what stays profile-driven versus what moves into shared infrastructure.

**Deliverables**

- Architecture doc update.
- A small glossary section that later tasks can reference.

**Acceptance**

- A contributor can tell where new logic belongs without reading multiple packages.

## Task 2: Rewrite Classification Layer

**Objective**

Split "what command is this?" from "which profile handles it?".

**Scope**

- Introduce a classification package or module, likely under `internal/profiles` or `internal/engine`, for:
  - normalized command identity
  - command family
  - subcommand
  - rewrite exemptions
  - structured-output eligibility
- Move common guard logic out of profile `Match` and `Prepare` functions when it is about command classification rather than reducer behavior.
- Ensure the engine can classify once and reuse that result during profile selection and prepare.

**Suggested shape**

- `Classification` struct with normalized command metadata.
- Shared helpers for:
  - command prefix normalization
  - machine-readable mode detection
  - known exemption detection
  - wrapper/toolchain normalization

**Deliverables**

- New classification API.
- Engine wiring that computes classification once per invocation.
- Migration of at least `git` and `javascript` profiles to use it.

**Acceptance**

- `Match` functions become simpler and more declarative.
- `Prepare` logic no longer re-derives the same classification facts ad hoc.

## Task 3: Profile Capabilities

**Objective**

Make profile safety and structured-mode expectations first-class.

**Scope**

- Extend [`internal/engine/profile.go`](/Users/alex/Documents/GitHub/szr/internal/engine/profile.go:1) with explicit profile capabilities or requirements.
- Cover questions like:
  - does this profile require structured stdout
  - does it inject flags
  - does it need stderr passthrough
  - does it support aggressive rewrites
  - does it allow fallback escape on parse failure
- Replace implicit behavior checks with capability-driven checks where practical.

**Suggested shape**

- `ProfileCapabilities` or `ProfileRequirements` on `engine.Profile`.
- Narrow enums or booleans for structured mode and rewrite policy rather than free-form comments.

**Deliverables**

- Type changes in the engine.
- One or two engine decisions rewritten to use capabilities.
- Representative migrations in `git` and `javascript`.

**Acceptance**

- Engine behavior that depends on profile behavior becomes inspectable from profile metadata.

## Task 4: Reusable Profile Builders

**Objective**

Reduce repeated setup code in profile definitions.

**Scope**

- Create builder helpers for the repeated patterns currently visible in:
  - [internal/profiles/javascript/profiles.go](/Users/alex/Documents/GitHub/szr/internal/profiles/javascript/profiles.go:1)
  - [internal/profiles/git/profile.go](/Users/alex/Documents/GitHub/szr/internal/profiles/git/profile.go:1)
- Normalize repeated wiring for:
  - output budgets
  - latency budgets
  - buffered reducers
  - combined stdout/stderr summarizers
  - stdout-only summarizers
  - common explain strings where appropriate

**Guardrails**

- Builders should remove ceremony, not hide profile-specific logic.
- Avoid a giant fluent API if plain helper constructors are enough.

**Deliverables**

- Shared builder/helper package.
- Migration of `git` and `javascript` profiles.
- No broad mechanical migration until the helpers prove useful.

**Acceptance**

- Representative profile files lose duplicated reducer wiring without becoming harder to read.

## Task 5: Declarative Fallback Reducers

**Objective**

Add a lightweight declarative fallback layer for simple line-oriented reducers.

**Scope**

- Design a minimal config format for built-in and optionally project-local declarative reducers.
- Limit the first version to line-based operations such as:
  - keep lines matching regex
  - strip lines matching regex
  - head/tail limits
  - truncate line width
  - empty-result message
- Keep structured parsing, state machines, and command-specific argument injection in Go profiles.

**Suggested placement**

- A new package under `internal/filters` or `internal/rules`.
- Built-in reducer definitions stored in repo-managed files.

**Deliverables**

- Declarative reducer schema.
- Loader and executor.
- Engine fallback integration.
- Two or three migrated low-complexity reducers.

**Acceptance**

- A contributor can add a simple fallback reducer without editing Go code.
- Existing structured profiles remain code-defined.

## Task 6: Truncation And Recovery Policy

**Objective**

Define one shared policy for hidden-output recovery.

**Scope**

- Introduce shared truncation policy types and helpers.
- Standardize when reducers must:
  - tee raw output
  - emit a recovery hint
  - keep offsets or file paths for tail-style continuation
- Audit reducers that currently hide lines or files without a predictable recovery path.

**Deliverables**

- Shared truncation/recovery helpers.
- Engine or filter integration point for recovery hints.
- Migration of at least:
  - large git diff summaries
  - path-list reducers
  - one log-oriented reducer

**Acceptance**

- Any reducer that hides meaningful content provides a deterministic recovery path.

## Task 7: Contributor Docs By Ecosystem

**Objective**

Lower the cost of adding or changing reducers.

**Scope**

- Add docs for profile families under `docs/` or adjacent to `internal/profiles`.
- Start with:
  - `git`
  - `javascript`
  - generic fallback/declarative reducers
- Each doc should explain:
  - what belongs in that family
  - common prepare rules
  - structured-output expectations
  - test strategy
  - when to use code versus declarative fallback

**Deliverables**

- Family-level contributor docs.
- Links from [CONTRIBUTING.md](/Users/alex/Documents/GitHub/szr/CONTRIBUTING.md:1) and [docs/ARCHITECTURE.md](/Users/alex/Documents/GitHub/szr/docs/ARCHITECTURE.md:1).

**Acceptance**

- A new contributor can add a reducer in one ecosystem without reverse-engineering unrelated profiles.

## Task 8: Incremental Migration Sweep

**Objective**

Adopt the new architecture without a risky flag day.

**Scope**

- Migrate profile families in small batches.
- Suggested order:
  1. `git`
  2. `javascript`
  3. search/path-list families
  4. build/test families
  5. cloud and Kubernetes
- Track migration status in this document.

**Deliverables**

- Batched migration PRs.
- Test updates for each family.
- Removal of dead helpers after each batch.

**Acceptance**

- Old and new patterns do not coexist indefinitely without a reason.

## Parallel Work Matrix

These tasks can overlap once Task 1 lands:

- Task 2 and Task 7 can run in parallel.
- Task 3 should start after Task 2 has a proposed classification shape.
- Task 4 should start after Task 3 defines the stable metadata that builders must populate.
- Task 5 can start once Task 1 defines the fallback boundary.
- Task 6 can start before Task 5 finishes, but its helper API should be reused by Task 5.
- Task 8 should only migrate a family after the needed infrastructure for that family is merged.

## Suggested PR Breakdown

1. `docs: define profile refactor architecture and task plan`
2. `refactor: introduce command classification layer`
3. `refactor: add explicit profile capabilities`
4. `refactor: add shared profile builders for buffered reducers`
5. `feat: add declarative fallback reducer engine`
6. `refactor: centralize truncation and recovery hints`
7. `docs: add contributor guides for git js and fallback reducers`
8. `refactor: migrate git profiles to classification and builders`
9. `refactor: migrate javascript profiles to classification and builders`

## Migration Checklist

- [ ] Task 1 complete
- [ ] Task 2 complete
- [ ] Task 3 complete
- [ ] Task 4 complete
- [ ] Task 5 complete
- [ ] Task 6 complete
- [ ] Task 7 complete
- [ ] `git` family migrated
- [ ] `javascript` family migrated
- [ ] search/path-list families migrated
- [ ] build/test families migrated
- [ ] cloud/Kubernetes families migrated
