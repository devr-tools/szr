#!/usr/bin/env bash
#
# Download the GoReleaser archives for a release tag and extract each `szr`
# binary into a stable staging layout that the npm and PyPI builders consume:
#
#   <staging>/<goos>_<goarch>/szr        (szr.exe on windows)
#
# Usage: extract-binaries.sh <tag> <version> [staging_dir]
#
#   tag         release tag, e.g. v0.16.0 or szr-v0.16.0
#   version     bare semver used in asset names, e.g. 0.16.0
#   staging_dir output dir (default: packaging/.staging)
#
# Requires: gh (authenticated), tar, unzip.
set -euo pipefail

tag="${1:?tag required}"
version="${2:?version required}"
staging="${3:-"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.staging"}"

# goos goarch -> archive suffix. Keep in sync with .goreleaser.yaml build matrix.
targets=(
  "darwin amd64 tar.gz"
  "darwin arm64 tar.gz"
  "linux amd64 tar.gz"
  "linux arm64 tar.gz"
  "windows amd64 zip"
)

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

rm -rf "$staging"
mkdir -p "$staging"

for target in "${targets[@]}"; do
  read -r goos goarch ext <<<"$target"
  asset="szr_${version}_${goos}_${goarch}.${ext}"
  dest="$staging/${goos}_${goarch}"
  mkdir -p "$dest"

  echo "==> $asset"
  gh release download "$tag" --repo devr-tools/szr --pattern "$asset" --dir "$workdir" --clobber

  if [ "$ext" = "zip" ]; then
    unzip -o "$workdir/$asset" "szr.exe" -d "$dest" >/dev/null
    chmod +x "$dest/szr.exe"
  else
    tar -xzf "$workdir/$asset" -C "$dest" szr
    chmod +x "$dest/szr"
  fi
done

echo "Binaries staged in $staging"
