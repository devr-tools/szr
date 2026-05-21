#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
  exit 0
fi

if [ "$1" = "szr" ]; then
  exit 0
fi

hint=""

case "$1" in
  git)
    case "${2:-}" in
      status|diff|log|show)
        hint="./bin/szr git ${2:-}"
        ;;
    esac
    ;;
  go)
    case "${2:-}" in
      test|build|vet|list)
        hint="./bin/szr go ${2:-}"
        ;;
    esac
    ;;
  npm|pnpm|yarn|bun)
    hint="./bin/szr $*"
    ;;
  pytest|cargo|docker)
    hint="./bin/szr $*"
    ;;
esac

if [ -n "$hint" ]; then
  printf 'szr hint: prefer %s\n' "$hint" >&2
fi
