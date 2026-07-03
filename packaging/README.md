# Packaging: npm and PyPI

`szr` is a Go binary released by GoReleaser to GitHub Releases. The npm and PyPI
packages here are thin wrappers that ship those **prebuilt binaries** — no Go
toolchain, and no network access, at install time.

| Registry | Install | Package(s) |
| --- | --- | --- |
| npm  | `npm install -g @devr-tools/szr` | `@devr-tools/szr` (launcher) + one `@devr-tools/szr-<os>-<cpu>` per platform |
| PyPI | `pip install szr` | one `szr-<ver>-py3-none-<platform>.whl` per platform |

Both are published automatically by `.github/workflows/release.yml` for **stable**
releases only (prereleases are skipped, matching the Homebrew job).

## How it works

- **npm** uses esbuild-style `optionalDependencies`. The `@devr-tools/szr`
  launcher declares one optional dependency per platform (each constrained by
  `os`/`cpu`). npm installs only the package matching the host, and the
  `bin/szr` launcher `require.resolve`s that package's binary and execs it.
- **PyPI** ships one wheel per platform. Each wheel carries the binary as a
  *data script* (`szr-<ver>.data/scripts/szr`), so pip drops `szr` straight onto
  PATH. There is no Python module and no shim.

## Layout

```
packaging/
  extract-binaries.sh      # download + extract release binaries into .staging/
  npm/
    launcher/bin/szr       # the launcher shim (committed source of truth)
    launcher/README.md     # README published with the main npm package
    build.sh               # generate dist/ from .staging/ + version
  pypi/
    build_wheels.py        # generate dist/*.whl from .staging/ + version
```

## Build and test locally

```bash
# 1. Stage binaries from a published release (needs an authenticated gh).
./packaging/extract-binaries.sh v0.16.0 0.16.0

# 2. npm packages -> packaging/npm/dist/
./packaging/npm/build.sh 0.16.0

# 3. wheels -> packaging/pypi/dist/
python3 packaging/pypi/build_wheels.py 0.16.0 packaging/.staging packaging/pypi/dist
```

`npm publish --dry-run <dir>` and `pipx run twine check packaging/pypi/dist/*.whl`
validate the artifacts without publishing.

## One-time prerequisites (before the first automated release)

These are external-registry setup steps the CI cannot perform for you. Both
registries use OIDC trusted publishing — no long-lived tokens live in CI.

1. **npm org + scope.** Create/own the `@devr-tools` npm org (or org scope) so
   the scoped packages can be published publicly.

2. **npm trusted publishing (OIDC).** npm requires each package to *already
   exist* before you can add a trusted publisher, and trusted publishers are
   configured **per package**. So:

   a. **Bootstrap once** — publish all six packages from your machine (needs
      `npm login` or a token in `~/.npmrc` with publish rights to
      `@devr-tools`):

      ```bash
      ./packaging/npm/bootstrap-publish.sh v0.16.0 0.16.0
      ```

   b. **Configure a trusted publisher** for each package on npmjs.com
      (package → Settings → Trusted Publisher → GitHub Actions):
      - Organization/user: `devr-tools`
      - Repository: `szr`
      - Workflow filename: `release.yml`
      - Environment: *(leave blank)*
      - Allowed actions: `npm publish`

      Packages: `@devr-tools/szr` plus `@devr-tools/szr-{darwin-x64,darwin-arm64,linux-x64,linux-arm64,win32-x64}`.

   After that, the `publish-npm` job publishes via OIDC (`id-token: write`,
   npm ≥ 11.5.1 which the job installs). No `NPM_TOKEN` needed, and provenance
   is attached automatically. Because stable releases trigger `release.yml`
   directly on the tag push, the trusted-publisher workflow name is simply
   `release.yml` (the `workflow_call` path is prerelease-only and not published).

3. **PyPI trusted publisher (OIDC).** On PyPI, add a *pending publisher* for
   project `szr`:
   - Owner / repo: `devr-tools/szr`
   - Workflow filename: `release.yml`
   - Environment: *(leave blank — the job sets none)*

   This lets the `publish-pypi` job authenticate via `id-token: write` with no
   long-lived token. (Alternatively, set a `PYPI_API_TOKEN` secret and pass it
   to `pypa/gh-action-pypi-publish` with `password:`.)

## Platform matrix

Keep this in sync with the `.goreleaser.yaml` build matrix. Currently:

| GOOS/GOARCH | npm package | wheel platform tag(s) |
| --- | --- | --- |
| darwin/amd64  | `@devr-tools/szr-darwin-x64`   | `macosx_10_9_x86_64` |
| darwin/arm64  | `@devr-tools/szr-darwin-arm64` | `macosx_11_0_arm64` |
| linux/amd64   | `@devr-tools/szr-linux-x64`    | `manylinux2014_x86_64`, `musllinux_1_1_x86_64` |
| linux/arm64   | `@devr-tools/szr-linux-arm64`  | `manylinux2014_aarch64`, `musllinux_1_1_aarch64` |
| windows/amd64 | `@devr-tools/szr-win32-x64`    | `win_amd64` |

GoReleaser ignores windows/arm64, so there is no wrapper for it.
