#!/usr/bin/env bash
# Baseline (6f9829d, V8 9.0) vs new v8go (consumer build: gcc + prebuilt
# bridge): v8go benchmarks, gojs benchmarks and soak tests.
#
# Run from the host with:
#   GH_TOKEN=$(gh auth token) tools/docker/dev.sh bench/run.sh [v8go] [gojs] [soak]
# With no argument every section runs; name sections to re-run only those.
# COUNT (default 10) and SOAK_ITERATIONS (default 100000) tune the run; OUT
# overrides the results directory (default bench/results/<date>), e.g.
#   tools/docker/dev.sh 'OUT=/src/v8go/bench/results/2026-10-06 bench/run.sh soak'
#
# GH_TOKEN (forwarded by dev.sh) gives go access to the private
# github.com/botify-hq modules gojs needs; it only lives in the throwaway
# container's git config.
set -euo pipefail
source /etc/profile.d/v8go.sh

COUNT=${COUNT:-10}
OUT=${OUT:-/src/v8go/bench/results/$(date +%F)}
SECTIONS=("$@")
[[ ${#SECTIONS[@]} -gt 0 ]] || SECTIONS=(v8go gojs soak)
mkdir -p "$OUT"
BENCH=(-run '^$' -benchmem -count "$COUNT" -timeout 4h)
# Cleanup1000Values has a long untimed setup per iteration: run it apart, with
# the same fixed iteration count for both versions, to bound its wall time.
SLOW='^BenchmarkCleanup1000Values$'

BASE=/src/cdf-gojs-baseline
UPGRADE=/src/cdf-gojs-upgrade

want() { [[ " ${SECTIONS[*]} " == *" $1 "* ]]; }

if [[ -n ${GH_TOKEN:-} ]]; then
  git config --global url."https://x-access-token:${GH_TOKEN}@github.com/".insteadOf "https://github.com/"
  export GOPRIVATE='github.com/botify-hq/*'
fi

# The worktrees' .git files hold host (Windows) paths: read their HEADs through
# the main repositories' worktree metadata.
rev() { git --git-dir="$1" rev-parse --short HEAD 2>/dev/null || echo unknown; }
{
  echo "date: $(date -Is)"
  echo "cpu: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ //')"
  echo "nproc: $(nproc)"
  echo "mem: $(grep MemTotal /proc/meminfo | awk '{print $2 " kB"}')"
  echo "kernel: $(uname -r)"
  echo "go: $(go version)"
  echo "gcc: $(gcc --version | head -1)"
  echo "COUNT=$COUNT SOAK_ITERATIONS=${SOAK_ITERATIONS:-100000}"
  echo "v8go: $(rev /src/v8go/.git), baseline: $(rev /src/v8go/.git/worktrees/v8go-baseline)"
  echo "cdf upgrade: $(rev /src/cdf/.git/worktrees/cdf-gojs-upgrade), baseline: $(rev /src/cdf/.git/worktrees/cdf-gojs-baseline)"
} >"$OUT/env-$(date +%H%M%S).txt"

# v8go_bench <output file> [go test flags...]
v8go_bench() {
  local out=$1
  shift
  GOWORK=off go test "$@" -bench . -skip "$SLOW" "${BENCH[@]}" . | tee "$out"
  GOWORK=off go test "$@" -bench "$SLOW" -benchtime=2000x "${BENCH[@]}" . | tee -a "$out"
}

# cdf master's gojs imports dop251/goja without requiring it (pre-existing bug):
# baseline gojs runs add the require (versions from its go.sum) for their
# duration only. go.mod/go.sum are saved first and copied back byte for byte,
# also by the EXIT trap (baseline_gojs runs inside pipelines, i.e. subshells),
# so the baseline worktree keeps only its untracked soak files. git can't be
# used here: the worktree's .git file holds a host path.
GOJA_REQUIRES=(
  -require=github.com/dop251/goja@v0.0.0-20251008123653-cf18d89f3cf6
  -require=github.com/dlclark/regexp2@v1.11.4
  -require=github.com/go-sourcemap/sourcemap@v2.1.3+incompatible
  -require=github.com/google/pprof@v0.0.0-20230207041349-798e818bf904
)
SAVED=$(mktemp -d)
restore_baseline() {
  if [[ -f $SAVED/go.mod ]]; then
    cp "$SAVED/go.mod" "$SAVED/go.sum" "$BASE/gojs/"
  fi
}
trap restore_baseline EXIT
if want gojs || want soak; then
  if grep -q 'dop251/goja' "$BASE/gojs/go.mod"; then
    echo "$BASE/gojs/go.mod already requires goja (left over?): restore it first with git checkout," \
      "run on the host, not in the container (the worktree's .git file holds a Windows path):" \
      "git -C D:/botify/cdf-gojs-baseline checkout gojs/go.mod gojs/go.sum" >&2
    exit 1
  fi
  cp "$BASE/gojs/go.mod" "$BASE/gojs/go.sum" "$SAVED/"
fi
# baseline_gojs <command...>: runs in the baseline gojs dir with the goja require.
baseline_gojs() {
  (cd "$BASE/gojs" && go mod edit "${GOJA_REQUIRES[@]}") || return 1
  local rc=0
  (cd "$BASE/gojs" && "$@") || rc=$?
  restore_baseline
  return $rc
}

if want v8go; then
  echo "== v8go benchmarks"
  cd /src/v8go/bench
  use_baseline
  v8go_bench "$OUT/v8go-baseline.txt" -tags v8baseline
  use_consumer
  v8go_bench "$OUT/v8go-new.txt"
  benchstat baseline="$OUT/v8go-baseline.txt" new="$OUT/v8go-new.txt" | tee "$OUT/v8go-benchstat.txt"
fi

if want gojs || want soak; then
  # The soak test is committed on the upgrade branch only; the baseline keeps an
  # untracked copy, plus its own soak_gc_test.go (rogchap.com/v8go import).
  cp "$UPGRADE/gojs/soak_test.go" "$BASE/gojs/soak_test.go"
  if [[ ! -f $BASE/gojs/soak_gc_test.go ]]; then
    echo "$BASE/gojs/soak_gc_test.go is missing (untracked copy of the upgrade one, importing rogchap.com/v8go)" >&2
    exit 1
  fi
fi

if want gojs; then
  echo "== gojs benchmarks"
  use_baseline
  gojs_work baseline
  baseline_gojs go test -bench 'BenchmarkV8_' "${BENCH[@]}" . | tee "$OUT/gojs-baseline.txt"
  use_consumer
  gojs_work new
  (cd "$UPGRADE/gojs" && go test -bench 'BenchmarkV8_' "${BENCH[@]}" .) | tee "$OUT/gojs-new.txt"
  benchstat baseline="$OUT/gojs-baseline.txt" new="$OUT/gojs-new.txt" | tee "$OUT/gojs-benchstat.txt"
fi

soak_failed=
soak_errors=
# soak_status <name> <log>: after a failed soak pipeline, tells a soak FAIL (a
# result: "--- FAIL" in the log) from a build or install error.
soak_status() {
  if grep -q -- '--- FAIL' "$2"; then
    soak_failed+=" $1"
  else
    soak_errors+=" $1"
  fi
}
if want soak; then
  # A soak FAIL or error is recorded, and the remaining soaks still run.
  echo "== soak"
  cd /src/v8go/bench
  use_baseline
  SOAK_OUT="$OUT/soak-v8go-baseline.csv" GOWORK=off go test -tags 'soak v8baseline' -run TestSoakCleanup -v -count 1 -timeout 4h . 2>&1 |
    tee "$OUT/soak-v8go-baseline.txt" || soak_status v8go-baseline "$OUT/soak-v8go-baseline.txt"
  use_consumer
  SOAK_OUT="$OUT/soak-v8go-new.csv" GOWORK=off go test -tags soak -run TestSoakCleanup -v -count 1 -timeout 4h . 2>&1 |
    tee "$OUT/soak-v8go-new.txt" || soak_status v8go-new "$OUT/soak-v8go-new.txt"
  use_baseline
  gojs_work baseline
  baseline_gojs env SOAK_OUT="$OUT/soak-gojs-baseline.csv" go test -tags soak -run TestSoakV8Cleanup -v -count 1 -timeout 4h . 2>&1 |
    tee "$OUT/soak-gojs-baseline.txt" || soak_status gojs-baseline "$OUT/soak-gojs-baseline.txt"
  use_consumer
  gojs_work new
  (cd "$UPGRADE/gojs" && SOAK_OUT="$OUT/soak-gojs-new.csv" go test -tags soak -run TestSoakV8Cleanup -v -count 1 -timeout 4h .) 2>&1 |
    tee "$OUT/soak-gojs-new.txt" || soak_status gojs-new "$OUT/soak-gojs-new.txt"
fi

echo "Results in $OUT"
[[ -z $soak_failed ]] || echo "Soak FAIL:$soak_failed" >&2
[[ -z $soak_errors ]] || echo "Soak not run (build or install error, see the soak-*.txt logs):$soak_errors" >&2
[[ -z $soak_failed$soak_errors ]] || exit 1
