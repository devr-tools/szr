# JavaScript Profile Family

Use the `javascript` family for Node and frontend tooling where `szr` can detect runners, forward reporter flags, and reduce noisy package-manager or workspace output into actionable failures.

## What belongs here

- direct test runners such as `vitest`, `jest`, and `bun test`
- package-manager test entrypoints such as `npm test`, `pnpm test`, and `yarn test`
- workspace and tooling commands such as `turbo`, `nx`, `vite`, `eslint`, `tsc`, `webpack`, and similar JS build or task orchestration tools

Add a dedicated JS profile when the command needs runner-aware prepare logic or a JS-specific reducer. If the output is only a generic line list, prefer shared fallback infrastructure instead.

## Common prepare rules

- Use classification to distinguish package-manager tests, direct runners, and general workspace commands.
- Detect the underlying runner behind package scripts before injecting structured reporter flags.
- Insert package-manager flags before `--` forwarding boundaries.
- In aggressive mode, add noise-suppression flags such as `--color=false`, `--silent`, `--no-progress`, `--no-audit`, or `--reporter=append-only` only when they are not already present.

Current examples:

- `js-package-test` inspects `package.json` to detect `vitest` or `jest` behind a package-manager command.
- `vitest-json` and `jest-json` request structured output when the command did not already ask for it.
- `js-workspace` suppresses package-manager noise but does not force a structured runner protocol.

## Structured-output expectations

- Direct `vitest` and `jest` profiles use `StructuredModeStdoutRequired`.
- Package-manager test wrappers and `bun test` prefer structured output when available, but still need fallback escape and full capture because wrappers can leak mixed stdout or stderr.
- General workspace commands usually stay text-based and rely on JS-specific summarizers rather than strict structured stdout.

If a new tool has a stable JSON reporter, prefer a dedicated Go profile over pushing it through the generic workspace reducer.

## Test strategy

- Add `Match` and `Prepare` coverage for direct runners, package-manager wrappers, and workspace commands.
- Cover `package.json` runner detection and argument-forwarding behavior.
- Test aggressive prepare rewrites separately from normal mode.
- Keep reducer tests focused on failing suites, assertions, file anchors, build diagnostics, and fallback escape behavior.

Good targets:

- package-manager test script resolves to `vitest`
- package-manager test script resolves to `jest`
- direct runner already has JSON or reporter flags
- workspace command preserves `--` argument forwarding

## Code vs declarative fallback

Keep JavaScript reducers in Go when they need:

- script inspection
- runner detection
- structured report parsing
- mixed stdout and stderr handling
- failure escape or full-capture requirements

Use declarative fallback only for simple line-based compaction on unmatched tooling output. Do not convert `vitest`, `jest`, package-manager test wrappers, or workspace reducers to declarative rules unless the reducer truly becomes line-only.
