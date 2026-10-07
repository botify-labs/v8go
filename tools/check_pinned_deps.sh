#!/usr/bin/env bash
# Fails unless the deps/<os>_<arch> modules that go.mod requires (what every
# consumer resolves: MVS takes them from this module's go.mod) carry a
# bridge.sha256 equal to the current sources' fingerprint (tools/bridge_hash.sh),
# and unless bench/go.mod requires the same versions.
# The in-tree check (tools/check_bridge.sh) only covers deps/*/ in this commit.
#
# After a C++ change this fails until the bridges are rebuilt (botify-bridge)
# and pinned: tools/pin_deps.sh <sha of the bridge commit>, then push.
# Usage: tools/check_pinned_deps.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
export GOWORK=off GOFLAGS=-mod=mod

want=$(tools/bridge_hash.sh)
fail=0
for gomod in deps/*_*/go.mod; do
  mod="github.com/botify-labs/v8go/$(dirname "$gomod")"
  ver=$(go list -m -f '{{.Version}}' "$mod")
  bver=$(go mod edit -json bench/go.mod | tr -d ' \t\n' | grep -oE "\"Path\":\"$mod\",\"Version\":\"[^\"]+\"" |
    sed 's/.*"Version":"//; s/"$//' || true)
  if [ "$bver" != "$ver" ]; then
    echo "bench/go.mod requires $mod ${bver:-<none>}, go.mod $ver: run tools/pin_deps.sh" >&2
    fail=1
  fi
  # Downloading also checks the module against go.sum.
  dir=$(go mod download -json "$mod@$ver" | grep -oE '"Dir": "[^"]+"' | cut -d'"' -f4)
  got=$(cat "$dir/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" = "$want" ]; then
    echo "ok: $mod@$ver"
  else
    echo "stale pin: $mod@$ver carries bridge $got, the sources are $want:" \
      "rebuild the bridges (botify-bridge), then tools/pin_deps.sh <bridge commit sha>" >&2
    fail=1
  fi
done
exit $fail
