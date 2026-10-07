#!/usr/bin/env bash
# gojs V8 benchmarks: baseline (cdf master + v8go 6f9829d), before (cdf
# upgrade + v8go HEAD, i.e. variant v0) and after (cdf upgrade + this working
# tree), interleaved, COUNT rounds. Then the gojs tests against the working
# tree. Runs inside tools/docker/dev.sh with GH_TOKEN set.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=${OUT:?}
COUNT=${COUNT:-10}
source /etc/profile.d/v8go.sh
mkdir -p "$OUT" /tmp/gb
git config --global url."https://x-access-token:${GH_TOKEN:?}@github.com/".insteadOf "https://github.com/"
export GOPRIVATE='github.com/botify-hq/*'
BASE=/src/cdf-gojs-baseline
UPGRADE=/src/cdf-gojs-upgrade

VARIANTS=v0 bash "$HERE/build_variants.sh" >"$OUT/build.log" 2>&1

use_consumer
gojs_work new
(cd $UPGRADE/gojs && go test -c -o /tmp/gb/after.test .)
sed 's#=> /src/v8go#=> /tmp/x/v8go#' /tmp/gojs.work >/tmp/gojs-before.work
(cd $UPGRADE/gojs && GOWORK=/tmp/gojs-before.work go test -c -o /tmp/gb/before.test .)

# cdf master's gojs imports goja without requiring it (see bench/run.sh): add
# the require for the build only, and restore go.mod/go.sum byte for byte.
SAVED=$(mktemp -d)
cp $BASE/gojs/go.mod $BASE/gojs/go.sum "$SAVED/"
trap 'cp "$SAVED/go.mod" "$SAVED/go.sum" $BASE/gojs/' EXIT
use_baseline
gojs_work baseline
(cd $BASE/gojs && go mod edit \
  -require=github.com/dop251/goja@v0.0.0-20251008123653-cf18d89f3cf6 \
  -require=github.com/dlclark/regexp2@v1.11.4 \
  -require=github.com/go-sourcemap/sourcemap@v2.1.3+incompatible \
  -require=github.com/google/pprof@v0.0.0-20230207041349-798e818bf904 &&
  go test -c -o /tmp/gb/baseline.test .)
cp "$SAVED/go.mod" "$SAVED/go.sum" $BASE/gojs/
md5sum /tmp/gb/*.test

for v in baseline before after; do : >"$OUT/gojs-$v.txt"; done
for i in $(seq "$COUNT"); do
  (cd $BASE/gojs && /tmp/gb/baseline.test -test.run '^$' -test.bench 'BenchmarkV8_' -test.benchmem -test.count 1 >>"$OUT/gojs-baseline.txt")
  (cd $UPGRADE/gojs && /tmp/gb/before.test -test.run '^$' -test.bench 'BenchmarkV8_' -test.benchmem -test.count 1 >>"$OUT/gojs-before.txt")
  (cd $UPGRADE/gojs && /tmp/gb/after.test -test.run '^$' -test.bench 'BenchmarkV8_' -test.benchmem -test.count 1 >>"$OUT/gojs-after.txt")
  echo "gojs round $i done"
done
benchstat baseline="$OUT/gojs-baseline.txt" before="$OUT/gojs-before.txt" after="$OUT/gojs-after.txt" | tee "$OUT/gojs-benchstat.txt"
benchstat before="$OUT/gojs-before.txt" after="$OUT/gojs-after.txt" >"$OUT/gojs-before-after.txt"

echo "== gojs tests (after)"
use_consumer
gojs_work new
(cd $UPGRADE/gojs && go test -count=1 ./... 2>&1) | tee "$OUT/gojs-tests.txt" | grep -E '^(ok|FAIL|---|panic)' || true
