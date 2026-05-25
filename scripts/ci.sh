#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

GO="${GO:-go}"
GOCACHE="${GOCACHE:-${ROOT_DIR}/.gocache}"
MIN_INTERNAL_COVERAGE="${MIN_INTERNAL_COVERAGE:-80.0}"
GOVULNCHECK_VERSION="${GOVULNCHECK_VERSION:-v1.3.0}"
GOVULNCHECK_MODE="${GOVULNCHECK_MODE:-warn}"
GOCYCLO_VERSION="${GOCYCLO_VERSION:-v0.6.0}"
SMOKE_HOME="${SMOKE_HOME:-${ROOT_DIR}/.tmp-home}"
COVERFILE="${COVERFILE:-.coverage.internal.out}"
BASE_REF="${BASE_REF:-}"
GO_BIN_DIR="$("${GO}" env GOPATH)/bin"

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

validate_govulncheck_mode() {
	case "${GOVULNCHECK_MODE}" in
		required|warn|off)
			;;
		*)
			die "invalid GOVULNCHECK_MODE=${GOVULNCHECK_MODE}; expected required, warn, or off"
			;;
	esac
}

ensure_go_tool() {
	local binary="$1"
	local module="$2"
	local version="$3"

	if command -v "${binary}" >/dev/null 2>&1; then
		command -v "${binary}"
		return 0
	fi

	if [[ -x "${GO_BIN_DIR}/${binary}" ]]; then
		printf '%s\n' "${GO_BIN_DIR}/${binary}"
		return 0
	fi

	env GOCACHE="${GOCACHE}" "${GO}" install "${module}@${version}" >/dev/null
	[[ -x "${GO_BIN_DIR}/${binary}" ]] || die "failed to install ${binary} (${module}@${version})"
	printf '%s\n' "${GO_BIN_DIR}/${binary}"
}

ensure_go_tool_optional() {
	local binary="$1"
	local module="$2"
	local version="$3"

	if ensure_go_tool "${binary}" "${module}" "${version}"; then
		return 0
	fi

	case "${GOVULNCHECK_MODE}" in
		required)
			die "failed to install ${binary} (${module}@${version})"
			;;
		warn)
			warn "skipping ${binary}: failed to install ${module}@${version} with ${GO}"
			return 1
			;;
		off)
			return 1
			;;
	esac
}

run_govulncheck() {
	local govulncheck_bin

	if [[ "${GOVULNCHECK_MODE}" == "off" ]]; then
		warn "skipping govulncheck: GOVULNCHECK_MODE=off"
		return 0
	fi

	if ! govulncheck_bin="$(ensure_go_tool_optional govulncheck golang.org/x/vuln/cmd/govulncheck "${GOVULNCHECK_VERSION}")"; then
		return 0
	fi

	if ! "${govulncheck_bin}" ./...; then
		warn "govulncheck reported findings"
	fi
}

validate_govulncheck_mode

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

log "fmt"
find . -type f -name '*.go' ! -path './.gocache/*' ! -path './bin/*' -print0 \
	| while IFS= read -r -d '' file; do
		printf '%s\t%s\n' "$(git hash-object "${file}")" "${file}"
	done \
	> "${tmpdir}/fmt-before.txt"
env GOCACHE="${GOCACHE}" "${GO}" fmt ./...
find . -type f -name '*.go' ! -path './.gocache/*' ! -path './bin/*' -print0 \
	| while IFS= read -r -d '' file; do
		printf '%s\t%s\n' "$(git hash-object "${file}")" "${file}"
	done \
	> "${tmpdir}/fmt-after.txt"

fmt_changed="$(
	awk -F '\t' '
		NR == FNR { before[$2] = $1; next }
		before[$2] != $1 { print $2 }
	' "${tmpdir}/fmt-before.txt" "${tmpdir}/fmt-after.txt"
)"

if [[ -n "${fmt_changed}" ]]; then
	printf 'go fmt rewrote these files:\n%s\n' "${fmt_changed}"
	die "formatting changes are required"
fi

log "vet"
env GOCACHE="${GOCACHE}" "${GO}" vet ./...

log "gocyclo"
gocyclo_files=()
while IFS= read -r file; do
	[[ -n "${file}" ]] || continue
	gocyclo_files+=("${file}")
done < <(find cmd internal pkg -type f -name '*.go' ! -name '*_test.go' | sort)
if [[ "${#gocyclo_files[@]}" -gt 0 ]]; then
	gocyclo_bin="$(ensure_go_tool gocyclo github.com/fzipp/gocyclo/cmd/gocyclo "${GOCYCLO_VERSION}")"
	"${gocyclo_bin}" -over 15 "${gocyclo_files[@]}" | tee "${tmpdir}/gocyclo.out"
	[[ ! -s "${tmpdir}/gocyclo.out" ]] || die "gocyclo found functions above the limit"
fi

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

log "security scan"
run_govulncheck

critical="$(printf '%s\n' "${changed_files}" | grep -E '^(cmd/szr/|cmd/szr-dev/|internal/cli/|internal/engine/|internal/installers/|internal/selfinstall/|internal/config/|go\.mod|go\.sum|Formula/|\.goreleaser\.yaml|\.github/workflows/.*\.yml|\.github/release-please-config\.json|\.release-please-manifest\.json)' || true)"
if [[ -n "${critical}" ]]; then
	warn "critical szr files modified"
	printf '%s\n' "${critical}"
fi

patterns="$(git diff "${merge_base}"...HEAD | grep -E '^\+.*(exec\.Command(Context)?\(|http\.(Get|Post)\(|net\.Dial\(|os\.RemoveAll\(|os\.Setenv\(|syscall\.|unsafe \{|panic!\(|TODO|FIXME)' || true)"
if [[ -n "${patterns}" ]]; then
	warn "potentially dangerous additions detected"
	printf '%s\n' "${patterns}" | sed -n '1,40p'
fi

if git diff "${merge_base}"...HEAD -- go.mod | grep -E '^\+[^+]' > "${tmpdir}/go_mod_additions.txt"; then
	warn "go.mod additions detected"
	sed -n '1,80p' "${tmpdir}/go_mod_additions.txt"
fi

log "semgrep"
if command -v semgrep >/dev/null 2>&1; then
	semgrep scan --config auto --baseline-commit "${base_sha}" --error
else
	docker run --rm -v "${ROOT_DIR}:/src" -w /src semgrep/semgrep semgrep scan --config auto --baseline-commit "${base_sha}" --error
fi

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
