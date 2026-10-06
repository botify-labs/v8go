#!/usr/bin/env bash
# Compiles the v8go C++ sources (the *.cc files, tagged v8go_source) for the
# current platform into the prebuilt bridge that consumers link statically:
# deps/<goos>_<goarch>/libv8go.a, or v8go.lib on Windows.
# Needs clang >= 21 as CXX (MSVC-target clang on Windows), like tommie/v8go.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
OUT="deps/${GOOS}_${GOARCH}"
OBJ=$(mktemp -d)
trap 'rm -rf "$OBJ"' EXIT

# _cgo_export.h declares the Go functions the C++ code calls back. cgo gets its
# own directory: it leaves type-probe objects (_cgo_N.o) there.
CGOFILES=$(go list -tags v8go_source -f '{{join .CgoFiles " "}}' .)
go tool cgo -objdir "$OBJ/cgo" -importpath github.com/botify-labs/v8go -- -I. $CGOFILES

CXXFLAGS=$(go list -tags v8go_source -f '{{join .CgoCXXFLAGS " "}}' .)
for src in *.cc; do
  $CXX $CXXFLAGS -nostdinc++ -O2 -g0 -I. -I"$OBJ/cgo" -c "$src" -o "$OBJ/${src%.cc}.o"
done

rm -f "$OUT/libv8go.a" "$OUT/v8go.lib"
if [ "$GOOS" = windows ]; then
  llvm-lib /out:"$OUT/v8go.lib" "$OBJ"/*.o
else
  ar rcs "$OUT/libv8go.a" "$OBJ"/*.o
fi
tools/bridge_hash.sh >"$OUT/bridge.sha256"
echo "built $OUT/$(ls "$OUT" | grep -E '^(libv8go\.a|v8go\.lib)$') ($(cat "$OUT/bridge.sha256"))"
