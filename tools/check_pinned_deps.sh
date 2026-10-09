#!/usr/bin/env bash
# Fails unless the deps/<os>_<arch> modules that go.mod requires (what every
# consumer resolves: MVS takes them from this module's go.mod):
# - carry a bridge.sha256 equal to the current sources' fingerprint
#   (tools/bridge_hash.sh);
# - are, file for file, deps/<os>_<arch> in this commit: the git tree of the
#   directory at the pinned commit equals HEAD's, which also catches changes the
#   fingerprint doesn't cover (cgo.go, the V8 archives, headers...);
# and unless bench/go.mod requires the same versions.
# The in-tree check (tools/check_bridge.sh) only covers deps/*/ in this commit.
# The pinned commits must be in the local clone (CI: fetch-depth 0).
#
# After a C++ change this fails until the bridges are rebuilt (botify-bridge)
# and pinned: tools/pin_deps.sh <sha of the bridge commit>, then push.
# Usage: tools/check_pinned_deps.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
export GOWORK=off GOFLAGS=-mod=mod

# The commit of a module version: a pseudo-version ends with its 12-digit
# abbreviated hash, a release is the tag <module dir>/<version>.
version_commit() {
  local d=$1 ver=$2 rev
  if [[ "$ver" =~ -([0-9a-f]{12})$ ]]; then
    rev=${BASH_REMATCH[1]}
  else
    rev="refs/tags/$d/$ver"
  fi
  git rev-parse -q --verify "$rev^{commit}"
}

want=$(tools/bridge_hash.sh)
fail=0
for gomod in deps/*_*/go.mod; do
  d=$(dirname "$gomod")
  mod="github.com/botify-labs/v8go/$d"
  ver=$(go list -m -f '{{.Version}}' "$mod")
  bver=$(go mod edit -json bench/go.mod | tr -d ' \t\n' | grep -oE "\"Path\":\"$mod\",\"Version\":\"[^\"]+\"" |
    sed 's/.*"Version":"//; s/"$//' || true)
  if [ "$bver" != "$ver" ]; then
    echo "bench/go.mod requires $mod ${bver:-<none>}, go.mod $ver: run tools/pin_deps.sh" >&2
    fail=1
  fi
  # Downloading also checks the module against go.sum.
  if ! json=$(go mod download -json "$mod@$ver"); then
    echo "$mod@$ver: go mod download failed:" >&2
    echo "$json" | grep -E '"Error"' >&2 || echo "$json" >&2
    fail=1
    continue
  fi
  dir=$(echo "$json" | grep -oE '"Dir": "[^"]+"' | cut -d'"' -f4)
  got=$(cat "$dir/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" != "$want" ]; then
    echo "stale pin: $mod@$ver carries bridge $got, the sources are $want:" \
      "rebuild the bridges (botify-bridge), then tools/pin_deps.sh <bridge commit sha>" >&2
    fail=1
    continue
  fi
  if ! commit=$(version_commit "$d" "$ver"); then
    echo "$mod@$ver: its commit isn't in this clone (git fetch, or fetch-depth: 0 in CI)" >&2
    fail=1
    continue
  fi
  pinned=$(git rev-parse "$commit:$d")
  head=$(git rev-parse "HEAD:$d")
  if [ "$pinned" = "$head" ]; then
    echo "ok: $mod@$ver"
  else
    echo "stale pin: $d at $mod@$ver ($commit) differs from HEAD's:" >&2
    git diff --stat "$commit" HEAD -- "$d" >&2
    echo "  pin a commit whose $d is HEAD's: tools/pin_deps.sh <sha>" >&2
    fail=1
  fi
done
exit $fail
