#!/bin/sh

set -eu

ROOT="${1:-$(mktemp -d "${TMPDIR:-/tmp}/szr-sandbox-perf.XXXXXX")}"
REPO_ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
GOROOT_DEFAULT="/opt/homebrew/opt/go/libexec"
GOROOT_VALUE="${GOROOT:-}"
BOOTSTRAP_DIR="$ROOT/bootstrap"
INSTALL_DIR="$ROOT/install/bin"
HOME_DIR="$ROOT/home"
WORK_DIR="$ROOT/work"
REPORT_DIR="$ROOT/report"
BOOTSTRAP_BIN="$BOOTSTRAP_DIR/szr"
INSTALLED_BIN="$INSTALL_DIR/szr"
GO_CACHE_DIR="$REPO_ROOT/.cache/go-build"
GO_MOD_CACHE_DIR="$REPO_ROOT/.cache/go-mod"

mkdir -p "$BOOTSTRAP_DIR" "$INSTALL_DIR" "$HOME_DIR/.config" "$HOME_DIR/.cache" "$WORK_DIR" "$REPORT_DIR"
mkdir -p "$GO_CACHE_DIR" "$GO_MOD_CACHE_DIR"

if [ -z "$GOROOT_VALUE" ] || [ ! -d "$GOROOT_VALUE" ]; then
  GOROOT_VALUE="$GOROOT_DEFAULT"
fi

export HOME="$HOME_DIR"
export XDG_CONFIG_HOME="$HOME_DIR/.config"
export XDG_CACHE_HOME="$HOME_DIR/.cache"
export PATH="$INSTALL_DIR:$BOOTSTRAP_DIR:$PATH"

build_bootstrap() {
  printf 'build bootstrap=%s\n' "$BOOTSTRAP_BIN"
  (
    cd "$REPO_ROOT"
    GOROOT="$GOROOT_VALUE" GOCACHE="$GO_CACHE_DIR" GOMODCACHE="$GO_MOD_CACHE_DIR" go build -o "$BOOTSTRAP_BIN" ./cmd/szr
  )
}

install_sandbox_binary() {
  printf 'install root=%s\n' "$ROOT"
  "$BOOTSTRAP_BIN" self install --path "$INSTALL_DIR" >"$REPORT_DIR/self-install.stdout" 2>"$REPORT_DIR/self-install.stderr"
  if [ ! -x "$INSTALLED_BIN" ]; then
    echo "szr sandbox install failed: missing $INSTALLED_BIN" >&2
    exit 1
  fi
}

seed_fixtures() {
  cat >"$WORK_DIR/sample.json" <<'EOF'
{
  "service": "szr",
  "env": "sandbox",
  "checks": [
    {"name": "bench", "status": "ok"},
    {"name": "doctor", "status": "ok"},
    {"name": "tee", "status": "pending"}
  ]
}
EOF

  cat >"$WORK_DIR/sample.log" <<'EOF'
INFO starting sandbox command sweep
INFO starting sandbox command sweep
WARN cache path missing, falling back
WARN cache path missing, falling back
ERROR benchmark fixture drift detected
ERROR benchmark fixture drift detected
EOF

  cat >"$WORK_DIR/emit-summary.sh" <<'EOF'
#!/bin/sh
printf 'line-01\nline-02\nline-03\nline-04\nline-05\nline-06\nline-07\nline-08\nline-09\nline-10\n'
EOF

  cat >"$WORK_DIR/emit-test.sh" <<'EOF'
#!/bin/sh
printf '=== RUN   TestAlpha\n'
printf '--- FAIL: TestAlpha (0.00s)\n'
printf '    alpha_test.go:12: expected 1 got 2\n'
printf 'FAIL\n'
exit 1
EOF

  cat >"$WORK_DIR/emit-fail.sh" <<'EOF'
#!/bin/sh
printf 'stdout-one\nstdout-two\n'
printf 'stderr-one\nstderr-two\n' >&2
exit 7
EOF

  chmod +x "$WORK_DIR/emit-summary.sh" "$WORK_DIR/emit-test.sh" "$WORK_DIR/emit-fail.sh"
}

now_seconds() {
  perl -MTime::HiRes=time -e 'printf "%.6f", time'
}

run_case() {
  name="$1"
  cwd="$2"
  shift 2

  stdout_file="$REPORT_DIR/$name.stdout"
  stderr_file="$REPORT_DIR/$name.stderr"
  start=$(now_seconds)
  if (
    cd "$cwd"
    "$@"
  ) >"$stdout_file" 2>"$stderr_file"; then
    code=0
  else
    code=$?
  fi
  end=$(now_seconds)
  duration_ms=$(awk "BEGIN { printf \"%.3f\", ($end - $start) * 1000 }")
  stdout_bytes=$(wc -c <"$stdout_file" | tr -d ' ')
  stderr_bytes=$(wc -c <"$stderr_file" | tr -d ' ')
  printf '%s\trc=%s\tdur_ms=%s\tstdout_bytes=%s\tstderr_bytes=%s\n' "$name" "$code" "$duration_ms" "$stdout_bytes" "$stderr_bytes" | tee -a "$REPORT_DIR/summary.tsv"
}

run_suite() {
  : >"$REPORT_DIR/summary.tsv"

  run_case commands "$REPO_ROOT" "$INSTALLED_BIN" commands
  run_case profiles "$REPO_ROOT" "$INSTALLED_BIN" profiles
  run_case doctor-json "$REPO_ROOT" "$INSTALLED_BIN" doctor --json
  run_case self-doctor-json "$REPO_ROOT" "$INSTALLED_BIN" self doctor --json
  run_case install-print "$REPO_ROOT" "$INSTALLED_BIN" install --all --print
  run_case uninstall-print "$REPO_ROOT" "$INSTALLED_BIN" uninstall --all --print
  run_case rewrite-json "$REPO_ROOT" "$INSTALLED_BIN" rewrite --json --command "git diff HEAD~1..HEAD --stat | tail -30"
  run_case ls "$REPO_ROOT" "$INSTALLED_BIN" ls .
  run_case find "$REPO_ROOT" "$INSTALLED_BIN" find . --name "*.go"
  run_case read "$REPO_ROOT" "$INSTALLED_BIN" read README.md
  run_case grep "$REPO_ROOT" "$INSTALLED_BIN" grep szr README.md
  run_case rg "$REPO_ROOT" "$INSTALLED_BIN" rg "func main" cmd
  run_case json "$WORK_DIR" "$INSTALLED_BIN" json "$WORK_DIR/sample.json"
  run_case log "$WORK_DIR" "$INSTALLED_BIN" log "$WORK_DIR/sample.log"
  run_case summary "$WORK_DIR" "$INSTALLED_BIN" summary "$WORK_DIR/emit-summary.sh"
  run_case test "$WORK_DIR" "$INSTALLED_BIN" test "$WORK_DIR/emit-test.sh"
  run_case run-echo "$WORK_DIR" "$INSTALLED_BIN" run /bin/echo hello-sandbox
  run_case proxy-echo "$WORK_DIR" "$INSTALLED_BIN" proxy /bin/echo hello-sandbox
  run_case compare-echo "$WORK_DIR" "$INSTALLED_BIN" compare /bin/echo hello-sandbox
  run_case explain-go-test "$REPO_ROOT" "$INSTALLED_BIN" explain go test ./...
  run_case git-status "$REPO_ROOT" "$INSTALLED_BIN" git status
  run_case run-fail "$WORK_DIR" "$INSTALLED_BIN" run "$WORK_DIR/emit-fail.sh"
  run_case tee-latest-json "$WORK_DIR" "$INSTALLED_BIN" tee --latest --json
  run_case tee-find "$WORK_DIR" "$INSTALLED_BIN" tee find emit-fail
  run_case tee-prune "$WORK_DIR" "$INSTALLED_BIN" tee prune
  run_case bench "$REPO_ROOT" "$INSTALLED_BIN" bench
  run_case bench-json "$REPO_ROOT" "$INSTALLED_BIN" bench --json
  run_case spread-json "$REPO_ROOT" "$INSTALLED_BIN" spread --json
  run_case hotspots-json "$REPO_ROOT" "$INSTALLED_BIN" hotspots --json
  run_case recommend-json "$REPO_ROOT" "$INSTALLED_BIN" recommend --json
}

print_tail_summary() {
  echo
  printf 'sandbox_root=%s\n' "$ROOT"
  printf 'installed_binary=%s\n' "$INSTALLED_BIN"
  printf 'report_summary=%s\n' "$REPORT_DIR/summary.tsv"
  printf 'bench_json=%s\n' "$REPORT_DIR/bench-json.stdout"
}

build_bootstrap
install_sandbox_binary
seed_fixtures
run_suite
print_tail_summary
