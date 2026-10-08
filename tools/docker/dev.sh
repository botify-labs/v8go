#!/usr/bin/env bash
# Runs a command in the v8go dev container. The parent directory of this
# checkout is mounted at /src: this checkout is /src/v8go, and the V8 9.0
# baseline that bench/ compares with is expected in /src/v8go-baseline.
# Usage: tools/docker/dev.sh '<command>'
# Forwarded when set: COUNT, SOAK_ITERATIONS, GH_TOKEN, GOPRIVATE, GONOSUMDB and
# the GOJS_* variables of bench/run.sh.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
HERE_W=$(cd "$HERE" && (pwd -W 2>/dev/null || pwd))
PARENT=$(cd "$HERE/../../.." && (pwd -W 2>/dev/null || pwd))
export MSYS_NO_PATHCONV=1

docker build -q -t v8go-dev "$HERE_W" >/dev/null
exec docker run --rm -i \
  -v "$PARENT:/src" \
  -v v8go-gomod:/go/pkg/mod \
  -v v8go-gocache:/root/.cache/go-build \
  -e COUNT -e SOAK_ITERATIONS -e GH_TOKEN -e GOPRIVATE -e GONOSUMDB \
  -e GOJS_BASELINE -e GOJS_UPGRADE -e GOJS_BASELINE_USE -e GOJS_UPGRADE_USE \
  -e GOJS_BASELINE_GITDIR -e GOJS_UPGRADE_GITDIR \
  -w /src/v8go \
  v8go-dev bash -lc "$*"
