# Releasing szr

`szr` uses `release-please` to propose stable releases and `goreleaser` to publish artifacts.

## Stable releases

Pushes to `main` trigger `.github/workflows/cd.yml`. That workflow runs `release-please`, which opens or updates a release PR based on conventional commits and `CHANGELOG.md`.

When the release PR is merged:

- `release-please` creates the stable tag
- `.github/workflows/release.yml` runs through `workflow_call`
- `goreleaser` publishes release artifacts and `checksums.txt`

## Prereleases

Pushes to `develop`, or manual `workflow_dispatch` runs on non-`main` branches, create prerelease tags like `v0.2.0-rc.5` and publish them through the same reusable release workflow.

## Manual fallback

If you need to cut a release directly, pushed tags that match `v*` still trigger `.github/workflows/release.yml`:

```bash
git push origin v0.1.0
```

Use stable semantic versions for stable releases and prerelease suffixes like `-rc.1` for prereleases.

## Homebrew follow-up

After a stable release is published, update `Formula/szr.rb` to point at the public release archive or tag archive and copy the matching `sha256`.

For `v0.1.0`, the formula uses the public tag archive:

`https://github.com/devr-tools/szr/archive/refs/tags/v0.1.0.tar.gz`
