#!/usr/bin/env bash
# Prints the fingerprint of the sources compiled into the prebuilt bridge:
# C++ sources and headers, Go files with //export (they shape _cgo_export.h),
# the cgo flags and the V8 version.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
sha() { if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi; }
{ ls *.cc *.h; grep -l '^//export ' *.go; echo cgo.go; echo deps/v8_hash; } |
  LC_ALL=C sort -u | xargs cat | sha | cut -d' ' -f1
