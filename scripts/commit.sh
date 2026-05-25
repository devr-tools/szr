#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

die() {
	printf 'error: %s\n' "$1" >&2
	exit 1
}

print_banner() {
	printf 'szr deploy your changes\n\n' >&2
}

require_clean_message() {
	local value="$1"
	[[ -n "${value}" ]] || die "commit message cannot be empty"
}

require_branch() {
	local branch

	branch="$(git branch --show-current)"
	[[ -n "${branch}" ]] || die "detached HEAD; switch to a branch before running make commit"
	printf '%s\n' "${branch}"
}

select_prefix() {
	local choice

	printf 'Choose commit type:\n' >&2
	printf '  1. major  -> feat!: breaking change\n' >&2
	printf '  2. minor  -> feat: new functionality\n' >&2
	printf '  3. patch  -> fix: patch-level change\n' >&2
	printf '  4. cancel -> exit without git add, commit, or push\n' >&2
	printf 'Selection [1-4]: ' >&2
	read -r choice

	case "${choice}" in
		1) printf 'feat!\n' ;;
		2) printf 'feat\n' ;;
		3) printf 'fix\n' ;;
		4) printf 'Cancelled.\n' >&2; exit 0 ;;
		*) die "invalid selection: ${choice}" ;;
	esac
}

commit_subject() {
	local prefix="$1"
	local summary

	printf 'Commit summary: ' >&2
	read -r summary
	require_clean_message "${summary}"
	printf '%s: %s\n' "${prefix}" "${summary}"
}

push_branch() {
	local branch="$1"

	if git rev-parse --verify --quiet "@{upstream}" >/dev/null; then
		git push
	else
		git push -u origin "${branch}"
	fi
}

branch="$(require_branch)"
print_banner
prefix="$(select_prefix)"
subject="$(commit_subject "${prefix}")"

printf 'Staging tracked and untracked changes with git add .\n'
git add .

if git diff --cached --quiet; then
	die "no staged changes to commit"
fi

printf 'Creating commit: %s\n' "${subject}"
git commit -m "${subject}"

printf 'Pushing branch: %s\n' "${branch}"
push_branch "${branch}"
