#!/usr/bin/env bash
# Runs a command in the v8go dev container. D:\botify is mounted at /src.
# Usage: tools/docker/dev.sh '<command>'
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
HERE_W=$(cd "$HERE" && (pwd -W 2>/dev/null || pwd))
BOTIFY=$(cd "$HERE/../../.." && (pwd -W 2>/dev/null || pwd))
export MSYS_NO_PATHCONV=1

docker build -q -t v8go-dev "$HERE_W" >/dev/null
exec docker run --rm -i \
  -v "$BOTIFY:/src" \
  -v v8go-gomod:/go/pkg/mod \
  -v v8go-gocache:/root/.cache/go-build \
  -e COUNT -e SOAK_ITERATIONS -e GH_TOKEN \
  -w /src/v8go \
  v8go-dev bash -lc "$*"
