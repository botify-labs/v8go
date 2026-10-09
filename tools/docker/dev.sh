#!/usr/bin/env bash
# Runs a command in the v8go dev container. The parent directory of this
# checkout is mounted at /src, and the command runs in this checkout,
# /src/<its directory name> (exported as V8GO_DIR: /src/v8go for a checkout
# named v8go, /src/<name> for a worktree or a second clone). The V8 9.0
# baseline that bench/ compares with is expected in /src/v8go-baseline.
# Usage: tools/docker/dev.sh '<command>'
# Forwarded when set: COUNT and SOAK_ITERATIONS (bench/run.sh).
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
HERE_W=$(cd "$HERE" && (pwd -W 2>/dev/null || pwd))
PARENT=$(cd "$HERE/../../.." && (pwd -W 2>/dev/null || pwd))
NAME=$(basename "$(cd "$HERE/../.." && pwd)")
export MSYS_NO_PATHCONV=1

docker build -q -t v8go-dev "$HERE_W" >/dev/null
exec docker run --rm -i \
  -v "$PARENT:/src" \
  -v v8go-gomod:/go/pkg/mod \
  -v v8go-gocache:/root/.cache/go-build \
  -e COUNT -e SOAK_ITERATIONS \
  -e "V8GO_DIR=/src/$NAME" \
  -w "/src/$NAME" \
  v8go-dev bash -lc "$*"
