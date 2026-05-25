# Local CI

`szr` has two local CI entrypoints:

- `make ci`: host-mode reproduction of the GitHub CI pipeline using your local toolchain and installed dependencies
- `make ci-docker`: pinned Linux container reproduction with Go, Semgrep, `govulncheck`, and `gocyclo` baked into the image

Use the containerized path when you want a result that is closer to the Ubuntu GitHub runner:

```bash
make ci-docker
make ci-docker BASE_REF=develop
```

Before running `make ci-docker`, make sure your local Docker daemon is available because the target builds `Dockerfile.ci` and runs `./scripts/ci.sh` inside that image.

The container path is designed to reduce host drift:

- pinned Linux image with Semgrep, `govulncheck`, and `gocyclo` preinstalled
- ephemeral Go build and module caches under `/tmp` inside the container
- container process runs as your local UID/GID to avoid root-owned files in the checkout

`make ci-docker` is closer to hosted CI, but GitHub Actions remains the source of truth for:

- `macos-latest`
- `windows-latest`
- pull-request-only metadata and GitHub job summaries

For quick local development, the most useful commands are:

```bash
make fmt
make test
make cover
make smoke
make prepush
```
