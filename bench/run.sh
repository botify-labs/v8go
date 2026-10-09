#!/usr/bin/env bash
# Baseline (6f9829d, V8 9.0) vs new v8go (consumer build: gcc + prebuilt
# bridge): v8go benchmarks and soak tests.
#
# Run from the host with:
#   tools/docker/dev.sh bench/run.sh [v8go-il] [v8go] [soak]
# Sections:
#   v8go-il  v8go benchmarks, baseline and new interleaved: COUNT rounds of one
#            run each (-test.count 1), the order of the two versions
#            alternating each round, so that machine drift spreads over both
#            (v8go-il-*.txt);
#   v8go     v8go benchmarks, -count COUNT for one version, then the other
#            (v8go-*.txt): sensitive to machine drift;
#   soak     v8go's TestSoakCleanup, baseline and new.
# With no argument: v8go-il and soak. Name sections to run only those. COUNT
# (default 10) and SOAK_ITERATIONS (default 100000) tune the run; OUT overrides
# the results directory (default bench/results/<date>), e.g.
#   tools/docker/dev.sh 'OUT=$V8GO_DIR/bench/results/2026-10-06 bench/run.sh soak'
#
# The baseline is v0.6.0-botify-baseline checked out in ../v8go-baseline
# (/src/v8go-baseline in the container; bench/go.mod replaces rogchap.com/v8go
# with it).
set -euo pipefail
source /etc/profile.d/v8go.sh

COUNT=${COUNT:-10}
OUT=${OUT:-$V8GO_DIR/bench/results/$(date +%F)}
SECTIONS=("$@")
if [[ ${#SECTIONS[@]} -eq 0 ]]; then
  SECTIONS=(v8go-il soak)
fi
mkdir -p "$OUT"
BENCH=(-run '^$' -benchmem -count "$COUNT" -timeout 4h)
# Cleanup1000Values has a long untimed setup per iteration: run it apart, with
# the same fixed iteration count for both versions, to bound its wall time.
SLOW='^BenchmarkCleanup1000Values$'

want() { [[ " ${SECTIONS[*]} " == *" $1 "* ]]; }

# A worktree's .git file holds a host (Windows) path: the baseline's HEAD is
# read through the main repository's worktree metadata.
rev() { git --git-dir="$1" rev-parse --short HEAD 2>/dev/null || echo unknown; }
rev_dir() { git -C "$1" rev-parse --short HEAD 2>/dev/null || echo unknown; }
{
  echo "date: $(date -Is)"
  echo "cpu: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ //')"
  echo "nproc: $(nproc)"
  echo "mem: $(grep MemTotal /proc/meminfo | awk '{print $2 " kB"}')"
  echo "kernel: $(uname -r)"
  echo "go: $(go version)"
  echo "gcc: $(gcc --version | head -1)"
  echo "COUNT=$COUNT SOAK_ITERATIONS=${SOAK_ITERATIONS:-100000} sections: ${SECTIONS[*]}"
  echo "v8go: $(rev_dir "$V8GO_DIR"), baseline: $(rev /src/v8go/.git/worktrees/v8go-baseline)"
} >"$OUT/env-$(date +%H%M%S).txt"

# v8go_bench <output file> [go test flags...]
v8go_bench() {
  local out=$1
  shift
  GOWORK=off go test "$@" -bench . -skip "$SLOW" "${BENCH[@]}" . | tee "$out"
  GOWORK=off go test "$@" -bench "$SLOW" -benchtime=2000x "${BENCH[@]}" . | tee -a "$out"
}

if want v8go-il; then
  echo "== v8go benchmarks, interleaved"
  cd "$V8GO_DIR/bench"
  bin=$(mktemp -d)
  use_baseline
  GOWORK=off go test -c -tags v8baseline -o "$bin/baseline.test" .
  use_consumer
  GOWORK=off go test -c -o "$bin/new.test" .
  for v in baseline new; do : >"$OUT/v8go-il-$v.txt"; done
  for i in $(seq "$COUNT"); do
    order="baseline new"
    ((i % 2)) || order="new baseline"
    for v in $order; do
      "$bin/$v.test" -test.run '^$' -test.bench . -test.skip "$SLOW" -test.benchmem -test.count 1 \
        -test.timeout 4h >>"$OUT/v8go-il-$v.txt"
      "$bin/$v.test" -test.run '^$' -test.bench "$SLOW" -test.benchtime 2000x -test.benchmem -test.count 1 \
        -test.timeout 4h >>"$OUT/v8go-il-$v.txt"
    done
    echo "round $i done ($order)"
  done
  benchstat baseline="$OUT/v8go-il-baseline.txt" new="$OUT/v8go-il-new.txt" | tee "$OUT/v8go-il-benchstat.txt"
fi

if want v8go; then
  echo "== v8go benchmarks"
  cd "$V8GO_DIR/bench"
  use_baseline
  v8go_bench "$OUT/v8go-baseline.txt" -tags v8baseline
  use_consumer
  v8go_bench "$OUT/v8go-new.txt"
  benchstat baseline="$OUT/v8go-baseline.txt" new="$OUT/v8go-new.txt" | tee "$OUT/v8go-benchstat.txt"
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
  # A soak FAIL or error is recorded, and the other soak still runs.
  echo "== soak"
  cd "$V8GO_DIR/bench"
  use_baseline
  SOAK_OUT="$OUT/soak-v8go-baseline.csv" GOWORK=off go test -tags 'soak v8baseline' -run TestSoakCleanup -v -count 1 -timeout 4h . 2>&1 |
    tee "$OUT/soak-v8go-baseline.txt" || soak_status v8go-baseline "$OUT/soak-v8go-baseline.txt"
  use_consumer
  SOAK_OUT="$OUT/soak-v8go-new.csv" GOWORK=off go test -tags soak -run TestSoakCleanup -v -count 1 -timeout 4h . 2>&1 |
    tee "$OUT/soak-v8go-new.txt" || soak_status v8go-new "$OUT/soak-v8go-new.txt"
fi

echo "Results in $OUT"
[[ -z $soak_failed ]] || echo "Soak FAIL:$soak_failed" >&2
[[ -z $soak_errors ]] || echo "Soak not run (build or install error, see the soak-*.txt logs):$soak_errors" >&2
[[ -z $soak_failed$soak_errors ]] || exit 1
