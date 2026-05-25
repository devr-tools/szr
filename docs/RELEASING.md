# Releasing szr

`szr` uses `release-please` to propose stable releases and `goreleaser` to publish artifacts.

Releases are deployment-driven:

- pushes to `main` or `master` let `release-please` manage the next stable release PR
- merges of that release PR create the stable tag and publish release artifacts
- pushes to `develop`, or manual runs on non-stable branches, create prerelease tags like `v0.2.0-rc.5`

## Release checklist

Before you cut or approve a release:

1. Run `make ci` or `make ci-docker`.
2. Confirm `README.md`, `CONTRIBUTING.md`, and release-facing docs still describe the current install and CI flow.
3. If the Homebrew formula will move to a new stable tag, prepare to update `Formula/szr.rb` with the published archive and checksum after the GitHub release exists.
4. If you are releasing from anything other than the default stable branch, decide whether this should be a prerelease instead of a stable tag.

## Stable releases

Pushes to `main` or `master` trigger `.github/workflows/cd.yml`. That workflow runs `release-please`, which opens or updates a release PR based on conventional commits and `CHANGELOG.md`.

When the release PR is merged:

- `release-please` creates the stable tag
- `.github/workflows/release.yml` runs through `workflow_call`
- `goreleaser` publishes release artifacts and `checksums.txt`
- the workflow also uploads the `dist/*` bundle as a GitHub Actions artifact for the tagged release run

For routine stable releases, this is the preferred path. Do not manually dispatch `.github/workflows/cd.yml` on `main` or `master`; that workflow intentionally fails there and tells you to use `.github/workflows/release.yml` instead.

## Prereleases

Pushes to `develop`, or manual `workflow_dispatch` runs on non-`main` branches, create prerelease tags like `v0.2.0-rc.5` and publish them through the same reusable release workflow.

The prerelease tag is computed from the latest stable tag plus commit history:

- breaking changes bump major
- `feat:` commits bump minor
- everything else bumps patch and appends `-rc.<run-number>`

## Versioning

Prereleases are produced from `develop` and non-`main` or non-`master` manual release runs.

- `BREAKING CHANGE` or `type!:` bumps major
- `feat:` bumps minor
- anything else bumps patch

Stable releases are produced from `main` or `master` through `release-please`, which opens or updates the release PR and creates the final stable tag when those changes land.

## Manual fallback

If release automation needs help, there are two manual fallback paths:

1. Dispatch `.github/workflows/release.yml` with:
   - `tag=v1.2.3` and `prerelease=false` for a stable release
   - `tag=v1.2.3-rc.4` and `prerelease=true` for a prerelease
   - the workflow also normalizes common inputs like `1.2.3`, `refs/tags/v1.2.3`, `tag=v1.2.3`, or `szr: v1.2.3`
2. Push a tag that matches `v*`, which still triggers `.github/workflows/release.yml`:

```bash
git push origin v0.1.0
```

Use stable semantic versions for stable releases and prerelease suffixes like `-rc.1` for prereleases.

`release.yml` validates that:

- stable releases use tags like `v1.2.3`
- prereleases use tags like `v1.2.3-rc.4`
- the requested tag exists remotely, or the workflow creates and pushes it from the current SHA

## Homebrew follow-up

After a stable release is published, update `Formula/szr.rb` to point at the public release archive or tag archive and copy the matching `sha256`.

For `v0.1.0`, the formula uses the public tag archive:

`https://github.com/devr-tools/szr/archive/refs/tags/v0.1.0.tar.gz`

Minimum stable-release follow-up:

1. Download or inspect the published source archive for the stable tag.
2. Compute or copy the matching `sha256`.
3. Update `Formula/szr.rb`.
4. Run the Homebrew validation workflow or equivalent local validation before merging the formula change.
