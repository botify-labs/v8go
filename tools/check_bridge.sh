#!/usr/bin/env bash
# Fails if a prebuilt bridge doesn't match the current sources.
# Usage: tools/check_bridge.sh [os_arch ...]   (default: all deps/*_*)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
want=$(tools/bridge_hash.sh)
targets=("$@")
[ ${#targets[@]} -gt 0 ] || targets=($(cd deps && ls -d *_*/ | tr -d /))
stale=0
for t in "${targets[@]}"; do
  [ -f "deps/$t/go.mod" ] || continue
  got=$(cat "deps/$t/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" != "$want" ]; then
    echo "stale bridge: deps/$t ($got, want $want)"
    stale=1
  fi
done
exit $stale
