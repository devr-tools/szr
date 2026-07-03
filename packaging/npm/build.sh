#!/usr/bin/env bash
#
# Assemble the publishable npm packages from staged binaries.
#
# Layout produced under packaging/npm/dist/:
#   @devr-tools/szr                 main launcher package (optionalDependencies)
#   @devr-tools/szr-darwin-x64      platform package (binary payload)
#   @devr-tools/szr-darwin-arm64
#   @devr-tools/szr-linux-x64
#   @devr-tools/szr-linux-arm64
#   @devr-tools/szr-win32-x64
#
# Usage: build.sh <version> [staging_dir]
set -euo pipefail

version="${1:?version required}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
staging="${2:-"$here/../.staging"}"
dist="$here/dist"

# npm-platform-key  npm-os  npm-cpu  staging-subdir  binary-name
platforms=(
  "darwin-x64  darwin  x64    darwin_amd64   szr"
  "darwin-arm64 darwin arm64  darwin_arm64   szr"
  "linux-x64   linux   x64    linux_amd64    szr"
  "linux-arm64 linux   arm64  linux_arm64    szr"
  "win32-x64   win32   x64    windows_amd64  szr.exe"
)

rm -rf "$dist"
mkdir -p "$dist"

optional_deps=""

for entry in "${platforms[@]}"; do
  read -r key npm_os npm_cpu subdir binname <<<"$entry"
  pkgname="@devr-tools/szr-${key}"
  pkgdir="$dist/szr-${key}/package"
  mkdir -p "$pkgdir/bin"

  src="$staging/$subdir/$binname"
  if [ ! -f "$src" ]; then
    echo "missing staged binary: $src" >&2
    exit 1
  fi
  cp "$src" "$pkgdir/bin/$binname"
  chmod +x "$pkgdir/bin/$binname"

  cat >"$pkgdir/package.json" <<JSON
{
  "name": "$pkgname",
  "version": "$version",
  "description": "The $key binary for szr.",
  "homepage": "https://github.com/devr-tools/szr",
  "repository": { "type": "git", "url": "git+https://github.com/devr-tools/szr.git" },
  "license": "Apache-2.0",
  "os": ["$npm_os"],
  "cpu": ["$npm_cpu"],
  "files": ["bin/$binname"]
}
JSON

  optional_deps="${optional_deps}    \"$pkgname\": \"$version\",\n"
done

# Trim trailing comma+newline from the assembled optionalDependencies block.
optional_deps="$(printf "%b" "$optional_deps" | sed '$ s/,$//')"

# Main launcher package.
maindir="$dist/szr/package"
mkdir -p "$maindir/bin"
cp "$here/launcher/bin/szr" "$maindir/bin/szr"
chmod +x "$maindir/bin/szr"
cp "$here/launcher/README.md" "$maindir/README.md"

cat >"$maindir/package.json" <<JSON
{
  "name": "@devr-tools/szr",
  "version": "$version",
  "description": "Token-aware CLI proxy that trims noisy command output for LLM workflows.",
  "homepage": "https://github.com/devr-tools/szr",
  "repository": { "type": "git", "url": "git+https://github.com/devr-tools/szr.git" },
  "license": "Apache-2.0",
  "keywords": ["cli", "llm", "proxy", "tokens", "agent"],
  "bin": { "szr": "bin/szr" },
  "files": ["bin/szr", "README.md"],
  "optionalDependencies": {
$optional_deps
  }
}
JSON

echo "npm packages built in $dist"
