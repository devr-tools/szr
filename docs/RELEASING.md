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

`release-please` must authenticate with `RELEASE_PLEASE_TOKEN`, not the default `GITHUB_TOKEN`. GitHub suppresses `pull_request` workflow runs for PRs created by `GITHUB_TOKEN`, which leaves required checks pending forever on the release PR. Configure `RELEASE_PLEASE_TOKEN` as a repo secret backed by the `please-release` bot account or a GitHub App token with permission to open and update pull requests.

When the release PR is merged:

- `release-please` creates the stable tag in `v1.2.3` form
- the tag push triggers `.github/workflows/release.yml`
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

After a stable release is published, `.github/workflows/release.yml` clones `devr-tools/homebrew-tap`, updates `Formula/szr.rb` there to the released tag, and opens or refreshes a PR in `devr-tools/homebrew-tap` with the matching source tarball `sha256`.

When `RELEASE_PLEASE_TOKEN` is available, the workflow pushes a branch in `devr-tools/homebrew-tap` and opens a PR against that repository's default branch. The generated PR is expected to run the tap repo's normal Homebrew validation workflow before merge.

If `RELEASE_PLEASE_TOKEN` is missing or cannot push or open the PR, the release still succeeds and the workflow writes a manual follow-up summary with the exact tag archive URL and `sha256` to apply in `devr-tools/homebrew-tap/Formula/szr.rb`.

If the automation needs help, the manual fallback is still:

1. Download or inspect the published source archive for the stable tag.
2. Compute or copy the matching `sha256`.
3. Update `Formula/szr.rb` in `devr-tools/homebrew-tap`.
4. Run the tap repo's Homebrew validation workflow or equivalent local validation before merging the formula change.
