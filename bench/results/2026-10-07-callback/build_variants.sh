#!/usr/bin/env bash
# Builds one consumer-mode bench test binary per optimization variant, plus the
# baseline, into /tmp/bin. Runs inside tools/docker/dev.sh.
#   v0 = tommie files and cleanup.cc at $REV
#   v1 = + 0001-value-tracker (and the working cleanup.cc)
#   v2 = + 0002-context-pointer
#   v3 = + 0003-callback-fast-path (its C++ files must equal the working tree's)
#   v4 = the working tree as is (+ 0004-callback-frame, Go side)
#   v4sp = v4 with the release_if of the first version (stable_partition)
#   nosort = v4 without the sort of released values by handle address
set -euo pipefail
source /etc/profile.d/v8go.sh
REV=${REV:-0d384af}  # the commit before Task 25
VARIANTS=${VARIANTS:-"v0 v1 v2 v3"}
X=/tmp/x
rm -rf "$X" /tmp/bin
mkdir -p "$X" /tmp/bin
ln -s /src/v8go-baseline "$X/v8go-baseline"
rsync -a --exclude .git --exclude .superpowers \
  --exclude 'deps/darwin_*/*.a' --exclude 'deps/linux_arm64/*.a' --exclude 'deps/windows_amd64/*.lib' \
  /src/v8go/ "$X/v8go/"
cd "$X/v8go"
git init -q .
cat >/tmp/x.work <<'EOF'
go 1.27

use (
	/tmp/x/v8go
	/tmp/x/v8go/deps/linux_amd64
)
EOF

TOMMIE="context.cc context.h function_template.cc"
for f in $TOMMIE function_template.go cleanup.cc; do git -C /src/v8go show "$REV:$f" >"$f"; done

build() {
  local name=$1
  use_source
  GOWORK=/tmp/x.work tools/build_bridge.sh
  cp deps/linux_amd64/libv8go.a "/tmp/bin/$name.a"
  (cd bench && use_consumer && GOWORK=off go test -c -o "/tmp/bin/$name.test" .)
  md5sum "/tmp/bin/$name.a" "/tmp/bin/$name.test"
}

for v in $VARIANTS; do
  case $v in
  v0) ;;
  v1) cp /src/v8go/cleanup.cc cleanup.cc; git apply /src/v8go/tools/patches/0001-*.patch ;;
  v2) git apply /src/v8go/tools/patches/0002-*.patch ;;
  v3) git apply /src/v8go/tools/patches/0003-*.patch
      for f in $TOMMIE cleanup.cc; do cmp "$f" "/src/v8go/$f"; done ;;
  v4) rsync -a --exclude .git --exclude .superpowers --exclude bench/results \
        --exclude 'deps/*_*/*.a' --exclude 'deps/*_*/*.lib' /src/v8go/ "$X/v8go/" ;;
  v4sp) cp /src/v8go/bench/results/2026-10-07-callback/botify_values.h.stable_partition botify_values.h ;;
  nosort) cp /src/v8go/botify_values.h botify_values.h; sed -i 's/kSortReleased = 16384/kSortReleased = SIZE_MAX/' botify_values.h
      grep -n 'kSortReleased =' botify_values.h ;;
  esac
  build "$v"
done

cd /src/v8go/bench
use_baseline
GOWORK=off go test -c -tags v8baseline -o /tmp/bin/base.test .
ls -la /tmp/bin
