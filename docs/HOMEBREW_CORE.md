# Homebrew Core Readiness

This repo is set up to ship a stable Homebrew formula from the `devr-tools/szr` tap and to validate formula builds on the two platforms that matter for Homebrew formulae:

- macOS
- Linux

## What is already in place

- stable tagged release: `v0.1.0`
- stable formula: `Formula/szr.rb`
- Apache-2.0 license at repo root
- SPDX license metadata in the formula
- public source tarball with pinned `sha256`
- GitHub Actions coverage for formula install and `brew test` on macOS and Linux
- release automation for future tags and source archives

## Repo-side blockers before a `homebrew/core` submission

1. Confirm the project meets Homebrew's current notability threshold for self-submitted software.
2. Run `brew audit --new --formula szr` cleanly against the final core-target formula.

## Notes

- Windows support is useful for users and releases, but it is not relevant to `homebrew/core`.
- `homebrew/core` acceptance is still a maintainer decision even after the repo is submission-ready.
