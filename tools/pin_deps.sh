#!/usr/bin/env bash
# Pins the deps/<os>_<arch> modules required by go.mod to a commit already
# pushed to github.com/botify-labs/v8go. Usage: tools/pin_deps.sh <commit-sha>
# Consumers get the bridges from these pinned modules, not from deps/ in the
# commit they require: the commit must contain the bridges of the current
# sources (its deps/*/bridge.sha256 equal to tools/bridge_hash.sh), i.e. it is
# the botify-bridge commit or a later one, and its deps/<os>_<arch> trees
# must be HEAD's (tools/check_pinned_deps.sh checks both). Run it after every
# bridge rebuild, before tagging or merging; botify-ci's pinned-deps-fresh job
# (tools/check_pinned_deps.sh) fails until then.
# bench/go.mod gets the same versions: it replaces the deps modules with the
# local directories, but its requirements must still match the root module's.
set -euo pipefail
SHA=${1:?usage: tools/pin_deps.sh <pushed-commit-sha>}
cd "$(git rev-parse --show-toplevel)"
export GOWORK=off GOPROXY=direct GOFLAGS=-mod=mod

want=$(tools/bridge_hash.sh)
for gomod in deps/*_*/go.mod; do
  d=$(dirname "$gomod")
  got=$(git show "$SHA:$d/bridge.sha256" 2>/dev/null || echo "missing (git fetch?)")
  if [ "$got" != "$want" ]; then
    echo "$SHA:$d/bridge.sha256 is $got, the sources are $want: pin the bridge commit" >&2
    exit 1
  fi
  if [ "$(git rev-parse "$SHA:$d")" != "$(git rev-parse "HEAD:$d")" ]; then
    echo "$SHA:$d differs from HEAD's (git diff $SHA HEAD -- $d): pin a commit whose $d is HEAD's" >&2
    exit 1
  fi
done

for gomod in deps/*_*/go.mod; do
  mod="github.com/botify-labs/v8go/$(dirname "$gomod")"
  ver=$(go list -m -f '{{.Version}}' "$mod@$SHA")
  go mod edit -require="$mod@$ver"
  go mod edit -require="$mod@$ver" bench/go.mod
  echo "$mod $ver"
done
go mod tidy
