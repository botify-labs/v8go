#!/usr/bin/env bash
# Builds the variants (build_variants.sh), then runs them interleaved, COUNT
# rounds of one run each, so that machine drift spreads over all variants.
# Results: $OUT/<variant>.txt, benchstat in $OUT/benchstat.txt.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=${OUT:?}
COUNT=${COUNT:-10}
BENCH=${BENCH:-'CallGoFromJS1000|CallJSFromGo|RunScriptTrivial|NewValueString|ObjectSetGet'}
bash "$HERE/build_variants.sh"
source /etc/profile.d/v8go.sh
mkdir -p "$OUT"
ALL="base ${VARIANTS:-v0 v1 v2 v3}"
for v in $ALL; do : >"$OUT/$v.txt"; done
cd /src/v8go/bench
for i in $(seq "$COUNT"); do
  for v in $ALL; do
    /tmp/bin/$v.test -test.run '^$' -test.bench "$BENCH" -test.benchmem -test.count 1 >>"$OUT/$v.txt"
    /tmp/bin/$v.test -test.run '^$' -test.bench '^BenchmarkCleanup1000Values$' -test.benchtime 2000x \
      -test.benchmem -test.count 1 >>"$OUT/$v.txt"
  done
  echo "round $i done"
done
args=()
for v in $ALL; do args+=("$v=$OUT/$v.txt"); done
benchstat "${args[@]}" | tee "$OUT/benchstat.txt"
