#!/usr/bin/env bash
# Prints the fingerprint of the sources compiled into the prebuilt bridge:
# C++ sources and headers, Go files with //export (they shape _cgo_export.h),
# the cgo flags, the V8 version, the compiler (the clang version
# .github/actions/setup-clang installs for botify-bridge), and the scripts that
# shape the bridge binary: tools/build_bridge.sh, tools/rename_cxx_runtime.sh
# (the C++ runtime rename applied to the Linux bridges) and its map
# tools/cxx-runtime-rename.map.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
sha() { if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi; }
{ ls *.cc *.h; grep -l '^//export ' *.go; echo cgo.go; echo deps/v8_hash;
  echo .github/actions/setup-clang/action.yml;
  echo tools/build_bridge.sh; echo tools/rename_cxx_runtime.sh; echo tools/cxx-runtime-rename.map; } |
  LC_ALL=C sort -u | xargs cat | sha | cut -d' ' -f1
