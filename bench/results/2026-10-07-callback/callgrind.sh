#!/usr/bin/env bash
# callgrind of each benchmark in CG_BENCHES at 20 and 40 ops per variant: the
# difference / 20000 is the instruction count per JS->Go callback
# (CallGoFromJS1000) or per value (Cleanup1000Values), overall and per
# function. Runs inside tools/docker/dev.sh (installs valgrind in the
# throwaway container). CG_TOGGLE restricts collection to one function;
# CG_EXTRA adds valgrind options (e.g. --cache-sim=yes; no per-function diff).
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=${OUT:?}
CG_VARIANTS=${CG_VARIANTS:-"base v0 v3 v4"}
CG_BENCHES=${CG_BENCHES:-"CallGoFromJS1000 Cleanup1000Values"}
apt-get update -qq >/dev/null && apt-get install -y -qq valgrind >/dev/null
[ -n "${SKIP_BUILD:-}" ] || bash "$HERE/build_variants.sh"
mkdir -p "$OUT"
cd /src/v8go/bench
for b in $CG_BENCHES; do
  for v in $CG_VARIANTS; do
    for n in 20 40; do
      GODEBUG=asyncpreemptoff=1 valgrind --tool=callgrind --smc-check=all-non-file \
        ${CG_TOGGLE:+--toggle-collect=$CG_TOGGLE} ${CG_EXTRA:-} \
        --callgrind-out-file="$OUT/$b-$v-$n.cg" \
        /tmp/bin/$v.test -test.run '^$' -test.bench "^Benchmark$b\$" -test.benchtime ${n}x -test.cpu 1 \
        >"$OUT/$b-$v-$n.log" 2>&1
      callgrind_annotate --threshold=100 --inclusive=no "$OUT/$b-$v-$n.cg" >"$OUT/$b-$v-$n-self.txt"
      callgrind_annotate --threshold=100 --inclusive=yes "$OUT/$b-$v-$n.cg" >"$OUT/$b-$v-$n-incl.txt"
      echo "$b $v $n $(grep -m1 -B2 'PROGRAM TOTALS' "$OUT/$b-$v-$n-self.txt" | tr '\n' ' ')"
    done
  done
  [ -n "${CG_EXTRA:-}" ] || python3 "$HERE/cgdiff.py" "$OUT/$b" $CG_VARIANTS | tee "$OUT/$b-per-unit.txt"
done
