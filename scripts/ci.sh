#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

GO="${GO:-go}"
GOCACHE="${GOCACHE:-${ROOT_DIR}/.gocache}"
MIN_INTERNAL_COVERAGE="${MIN_INTERNAL_COVERAGE:-80.0}"
GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_VERSION:-v2.12.2}"
CODEGUARD_VERSION="${CODEGUARD_VERSION:-v1.9.0}"
CODEGUARD_CONFIG="${CODEGUARD_CONFIG:-codeguard.yaml}"
SMOKE_HOME="${SMOKE_HOME:-${ROOT_DIR}/.tmp-home}"
COVERFILE="${COVERFILE:-.coverage.internal.out}"
BASE_REF="${BASE_REF:-}"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/szr-ci.XXXXXX")"
trap 'rm -rf "${tmpdir}"' EXIT

log() {
	printf '\n==> %s\n' "$1"
}

warn() {
	printf 'warning: %s\n' "$1" >&2
}

die() {
	printf 'error: %s\n' "$1" >&2
	exit 1
}

sanitize_go_env() {
	if [[ -n "${GOROOT:-}" && ! -d "${GOROOT}" ]]; then
		warn "ignoring invalid GOROOT=${GOROOT}"
		unset GOROOT
	fi
}

sanitize_go_env
GO_BIN_DIR="$("${GO}" env GOPATH)/bin"

# tool_version_matches reports whether an already-present binary is the pinned
# version. Both codeguard ("v1.9.0") and golangci-lint ("golangci-lint has
# version 2.12.2 ...") embed the bare number, so comparing against the pin with
# its leading "v" stripped works for both.
tool_version_matches() {
	local binary="$1"
	local version="$2"
	local output

	output="$("${binary}" version 2>/dev/null || true)"
	if [[ -z "${output}" ]]; then
		output="$("${binary}" --version 2>/dev/null || true)"
	fi

	[[ -n "${output}" && "${output}" == *"${version#v}"* ]]
}

# ensure_go_tool resolves a pinned tool, preferring an existing binary only when
# it is already the pinned version. Accepting whatever happens to be on PATH is
# how a stale pin survives unnoticed: every developer runs their own build while
# CI installs the pin on a clean runner, so the two silently diverge.
ensure_go_tool() {
	local binary="$1"
	local module="$2"
	local version="$3"
	local candidate

	for candidate in "$(command -v "${binary}" 2>/dev/null || true)" "${GO_BIN_DIR}/${binary}"; do
		if [[ -n "${candidate}" && -x "${candidate}" ]] && tool_version_matches "${candidate}" "${version}"; then
			printf '%s\n' "${candidate}"
			return 0
		fi
	done

	env GOCACHE="${GOCACHE}" "${GO}" install "${module}@${version}" >/dev/null
	[[ -x "${GO_BIN_DIR}/${binary}" ]] || die "failed to install ${binary} (${module}@${version})"
	printf '%s\n' "${GO_BIN_DIR}/${binary}"
}

if [[ -z "${BASE_REF}" ]]; then
	BASE_REF="$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')"
fi

[[ -n "${BASE_REF}" ]] || die "could not determine BASE_REF; run BASE_REF=main make ci"

base_remote_ref="origin/${BASE_REF}"
git rev-parse --verify --quiet "${base_remote_ref}" >/dev/null || die "missing ${base_remote_ref}; fetch it first or override BASE_REF"

merge_base="$(git merge-base HEAD "${base_remote_ref}")"
base_sha="$(git rev-parse "${base_remote_ref}")"
head_sha="$(git rev-parse HEAD)"
changed_files="$(git diff --name-only "${merge_base}"...HEAD)"

log "Local CI against ${base_remote_ref}"
printf 'merge-base: %s\nhead: %s\n' "${merge_base}" "${head_sha}"
printf 'note: this runs the same checks locally on the current OS; GitHub still validates the ubuntu, macos, and windows matrix.\n'
printf 'note: prerelease version bumps follow commit messages since the last stable tag: BREAKING CHANGE or type!: => major, feat: => minor, everything else => patch.\n'

log "test presence"
# Ignore package-doc files so documentation-only Go changes do not require
# unrelated test churn.
code_changes="$(printf '%s\n' "${changed_files}" | grep -E '^(cmd/|internal/|pkg/).+\.go$' | grep -Ev '(^|/).+_test\.go$|(^|/)doc\.go$' || true)"
test_changes="$(printf '%s\n' "${changed_files}" | grep -E '(^test/|_test\.go$)' || true)"

if [[ -n "${code_changes}" && -z "${test_changes}" ]]; then
	printf 'Go source files changed:\n%s\n' "${code_changes}"
	die "Go source changed without updates under test/ or *_test.go files"
fi

log "vet"
env GOCACHE="${GOCACHE}" "${GO}" vet ./...

log "golangci-lint"
golangci_lint_bin="$(ensure_go_tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint "${GOLANGCI_LINT_VERSION}")"
env GOCACHE="${GOCACHE}" "${golangci_lint_bin}" run --config .golangci.yml --new-from-rev "${merge_base}" ./...

log "codeguard"
codeguard_bin="$(ensure_go_tool codeguard github.com/devr-tools/codeguard/cmd/codeguard "${CODEGUARD_VERSION}")"
env GOCACHE="${GOCACHE}" "${codeguard_bin}" scan \
	-config "${CODEGUARD_CONFIG}" \
	-mode diff \
	-base-ref "${base_remote_ref}" \
	-format text

log "test (ubuntu-latest full)"
env GOCACHE="${GOCACHE}" "${GO}" test ./test/...

log "test (windows-latest smoke package set)"
env GOCACHE="${GOCACHE}" "${GO}" test ./test/config ./test/filters/... ./test/installers ./test/profiles/... ./test/rules ./test/teeindex

log "coverage"
env GOCACHE="${GOCACHE}" "${GO}" test ./test/... -coverpkg=./internal/... -coverprofile="${COVERFILE}"
total="$(env GOCACHE="${GOCACHE}" "${GO}" tool cover -func="${COVERFILE}" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
awk -v total="${total}" -v min="${MIN_INTERNAL_COVERAGE}" 'BEGIN { exit !(total + 0 >= min + 0) }' || die "coverage gate failed: ${total}% (min ${MIN_INTERNAL_COVERAGE}%)"

log "smoke"
mkdir -p "${SMOKE_HOME}"
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr --help >/dev/null
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr profiles >/dev/null
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr explain git status >/dev/null
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr bench clean-pass >/dev/null
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr install codex --print >/dev/null
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr-dev --version >/dev/null

log "benchmark"
env GOCACHE="${GOCACHE}" "${GO}" test ./test/bench -run '^$' -bench . -benchmem | tee "${tmpdir}/bench.txt"
env HOME="${SMOKE_HOME}" GOCACHE="${GOCACHE}" "${GO}" run ./cmd/szr bench --json clean-pass > "${tmpdir}/bench-fixture.json"
sed -n '1,40p' "${tmpdir}/bench-fixture.json"

if [[ "${BASE_REF}" == "develop" ]]; then
	log "doc review"
	ci_cd_only="$(printf '%s\n' "${changed_files}" | grep -E '^(\.github/workflows/|\.github/release-please-config\.json$|\.release-please-manifest\.json$)' || true)"
	doc_sensitive="$(printf '%s\n' "${changed_files}" | grep -E '^(\.goreleaser\.yaml$|Formula/|internal/installers/|internal/selfinstall/|README\.md$|docs/)' || true)"
	docs_changed="$(printf '%s\n' "${changed_files}" | grep -E '^(README\.md|CONTRIBUTING\.md|docs/)' || true)"

	if [[ -n "${doc_sensitive}" && -z "${docs_changed}" ]]; then
		printf 'Doc-sensitive files changed:\n%s\n' "${doc_sensitive}"
		die "install or release changes require updates in README.md, CONTRIBUTING.md, or docs/"
	fi

	if [[ -z "${doc_sensitive}" && -n "${ci_cd_only}" ]]; then
		printf 'Only CI/CD automation files changed. Documentation updates are not required for this branch target.\n'
	fi

	log "dco"
	missing=0
	while IFS= read -r commit; do
		[[ -n "${commit}" ]] || continue
		if ! git show -s --format=%B "${commit}" | grep -qi '^Signed-off-by:'; then
			printf 'missing Signed-off-by trailer: %s\n' "${commit}" >&2
			missing=1
		fi
	done < <(git rev-list "${merge_base}..HEAD")

	[[ "${missing}" -eq 0 ]] || die "one or more commits are missing Signed-off-by trailers"
fi

log "CI checks passed"
